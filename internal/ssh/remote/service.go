package remote

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
	sshconnection "ssh-man/internal/ssh/connection"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	ErrNotConnected       = errors.New("remote file explorer is not connected")
	ErrRemoteFileChanged  = errors.New("the remote file changed after it was opened")
	ErrRemoteFileTooLarge = errors.New("remote files larger than 2 MB cannot be edited")
	ErrUnsupportedEdit    = errors.New("this remote file cannot be edited")
	ErrUnsupportedSymlink = errors.New("downloading symbolic links is not supported")
)

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type FileSystem interface {
	Getwd() (string, error)
	ReadDir(string) ([]os.FileInfo, error)
	Lstat(string) (os.FileInfo, error)
	Open(string) (ReadSeekCloser, error)
	OpenFile(string, int, os.FileMode) (io.WriteCloser, error)
	Chmod(string, os.FileMode) error
	PosixRename(string, string) error
	Remove(string) error
	Close() error
}

type Dialer func(context.Context, serverdomain.Server, string) (FileSystem, io.Closer, error)

type Service struct {
	mu            sync.RWMutex
	saveMu        sync.Mutex
	server        serverdomain.Server
	dial          Dialer
	temporaryPath func(string) (string, error)
	fs            FileSystem
	transport     io.Closer
	home          string
}

func NewService(server serverdomain.Server) *Service {
	return NewServiceWithDialer(server, dialSFTP)
}

func NewServiceWithDialer(server serverdomain.Server, dial Dialer) *Service {
	return &Service{server: server, dial: dial, temporaryPath: temporarySavePath}
}

func (s *Service) Server() serverdomain.Server {
	return s.server
}

func (s *Service) Connect(ctx context.Context, passphrase string) (ConnectResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fs != nil {
		return ConnectResult{Connected: true, HomePath: s.home}, nil
	}

	remoteFS, transport, err := s.dial(ctx, s.server, passphrase)
	if err != nil {
		if errors.Is(err, auth.ErrPassphraseRequired) {
			return ConnectResult{NeedsPassphrase: true}, nil
		}
		return ConnectResult{}, err
	}
	home, err := remoteFS.Getwd()
	if err != nil {
		_ = remoteFS.Close()
		if transport != nil {
			_ = transport.Close()
		}
		return ConnectResult{}, fmt.Errorf("find remote home directory: %w", err)
	}
	s.fs = remoteFS
	s.transport = transport
	s.home = cleanRemotePath(home)
	return ConnectResult{Connected: true, HomePath: s.home}, nil
}

func (s *Service) Home() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.home
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	if s.fs != nil {
		errs = append(errs, s.fs.Close())
		s.fs = nil
	}
	if s.transport != nil {
		errs = append(errs, s.transport.Close())
		s.transport = nil
	}
	return errors.Join(errs...)
}

func (s *Service) List(directoryPath string) (Directory, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return Directory{}, err
	}
	directoryPath = resolveRemotePath(home, directoryPath)
	items, err := remoteFS.ReadDir(directoryPath)
	if err != nil {
		return Directory{}, fmt.Errorf("list %q: %w", directoryPath, err)
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		if err := validateRemoteEntryName(item.Name()); err != nil {
			return Directory{}, fmt.Errorf("list %q: %w", directoryPath, err)
		}
		kind := "file"
		if item.IsDir() {
			kind = "directory"
		} else if item.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
		}
		entries = append(entries, Entry{
			Name:       item.Name(),
			Path:       path.Join(directoryPath, item.Name()),
			Kind:       kind,
			Size:       item.Size(),
			ModifiedAt: item.ModTime(),
			Mode:       item.Mode().String(),
			Hidden:     strings.HasPrefix(item.Name(), "."),
		})
	}
	sortEntries(entries)
	return Directory{Path: directoryPath, Entries: entries}, nil
}

func (s *Service) Preview(remotePath string) (Preview, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return Preview{}, err
	}
	remotePath = resolveRemotePath(home, remotePath)
	info, err := remoteFS.Lstat(remotePath)
	if err != nil {
		return Preview{}, fmt.Errorf("inspect %q: %w", remotePath, err)
	}
	if info.IsDir() {
		return Preview{}, fmt.Errorf("%q is a directory", remotePath)
	}

	kind, mimeType := classifyPreview(remotePath)
	preview := Preview{Path: remotePath, Name: path.Base(remotePath), Kind: kind, MimeType: mimeType, Size: info.Size()}
	textualBrowserPreview := kind == "browser" && (strings.HasPrefix(mimeType, "text/") || strings.Contains(mimeType, "svg"))
	if kind == "image" || (kind == "browser" && !textualBrowserPreview) || kind == "unsupported" {
		return preview, nil
	}

	file, err := remoteFS.Open(remotePath)
	if err != nil {
		return Preview{}, fmt.Errorf("open %q: %w", remotePath, err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, MaxPreviewBytes+1))
	if err != nil {
		return Preview{}, fmt.Errorf("read preview for %q: %w", remotePath, err)
	}
	if len(content) > int(MaxPreviewBytes) {
		content = content[:MaxPreviewBytes]
		preview.Truncated = true
	}
	if kind == "text" && looksBinary(content) {
		preview.Kind = "unsupported"
		return preview, nil
	}
	preview.Content = string(content)
	if !preview.Truncated {
		preview.Revision = contentRevision(content)
	}
	return preview, nil
}

func (s *Service) Save(remotePath, content, expectedRevision string) (Preview, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return Preview{}, err
	}
	remotePath = resolveRemotePath(home, remotePath)
	contentBytes := []byte(content)
	if len(contentBytes) > int(MaxPreviewBytes) {
		return Preview{}, ErrRemoteFileTooLarge
	}
	if looksBinary(contentBytes) || strings.TrimSpace(expectedRevision) == "" {
		return Preview{}, ErrUnsupportedEdit
	}

	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	info, currentContent, err := editableRemoteFile(remoteFS, remotePath)
	if err != nil {
		return Preview{}, err
	}
	if contentRevision(currentContent) != expectedRevision {
		return Preview{}, ErrRemoteFileChanged
	}
	temporaryPath, err := s.temporaryPath(remotePath)
	if err != nil {
		return Preview{}, fmt.Errorf("prepare temporary remote file: %w", err)
	}
	writer, err := remoteFS.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return Preview{}, fmt.Errorf("create temporary remote file: %w", err)
	}
	keepTemporary := true
	defer func() {
		if keepTemporary {
			_ = remoteFS.Remove(temporaryPath)
		}
	}()
	if _, err := io.Copy(writer, bytes.NewReader(contentBytes)); err != nil {
		_ = writer.Close()
		return Preview{}, fmt.Errorf("write temporary remote file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return Preview{}, fmt.Errorf("close temporary remote file: %w", err)
	}
	if err := remoteFS.Chmod(temporaryPath, info.Mode().Perm()); err != nil {
		return Preview{}, fmt.Errorf("preserve remote file permissions: %w", err)
	}
	_, latestContent, err := editableRemoteFile(remoteFS, remotePath)
	if err != nil || contentRevision(latestContent) != expectedRevision {
		return Preview{}, ErrRemoteFileChanged
	}
	if err := remoteFS.PosixRename(temporaryPath, remotePath); err != nil {
		return Preview{}, fmt.Errorf("replace remote file atomically: %w", err)
	}
	keepTemporary = false
	return s.Preview(remotePath)
}

func (s *Service) Open(remotePath string) (ReadSeekCloser, os.FileInfo, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return nil, nil, err
	}
	remotePath = resolveRemotePath(home, remotePath)
	info, err := remoteFS.Lstat(remotePath)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("%q is a directory", remotePath)
	}
	file, err := remoteFS.Open(remotePath)
	if err != nil {
		return nil, nil, err
	}
	return file, info, nil
}

func (s *Service) Download(ctx context.Context, remotePaths []string, destinationDirectory string) ([]string, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return nil, err
	}
	if len(remotePaths) == 0 {
		return []string{}, nil
	}
	if info, err := os.Stat(destinationDirectory); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("download destination must be an existing directory")
	}

	results := make([]string, 0, len(remotePaths))
	for _, remotePath := range remotePaths {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		remotePath = resolveRemotePath(home, remotePath)
		info, err := remoteFS.Lstat(remotePath)
		if err != nil {
			return results, fmt.Errorf("inspect %q: %w", remotePath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return results, fmt.Errorf("%q: %w", remotePath, ErrUnsupportedSymlink)
		}
		target := availableTarget(destinationDirectory, path.Base(remotePath))
		if info.IsDir() {
			err = downloadDirectory(ctx, remoteFS, remotePath, target)
		} else {
			err = downloadFile(ctx, remoteFS, remotePath, target, info)
		}
		if err != nil {
			return results, err
		}
		results = append(results, target)
	}
	return results, nil
}

func (s *Service) connectedFS() (FileSystem, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.fs == nil {
		return nil, "", ErrNotConnected
	}
	return s.fs, s.home, nil
}

func dialSFTP(ctx context.Context, server serverdomain.Server, passphrase string) (FileSystem, io.Closer, error) {
	authMethod, err := sshconnection.AuthMethod(server, passphrase)
	if err != nil {
		return nil, nil, err
	}
	hostKeyCallback, err := sshconnection.KnownHostsCallback()
	if err != nil {
		return nil, nil, fmt.Errorf("configure SSH host key verification: %w", err)
	}
	address := fmt.Sprintf("%s:%d", server.Host, server.Port)
	netConn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to ssh server: %w", err)
	}
	handshakeDeadline := time.Now().Add(10 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(handshakeDeadline) {
		handshakeDeadline = contextDeadline
	}
	_ = netConn.SetDeadline(handshakeDeadline)
	config := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}
	sshConn, channels, requests, err := ssh.NewClientConn(netConn, address, config)
	if err != nil {
		_ = netConn.Close()
		return nil, nil, fmt.Errorf("connect to ssh server: %w", err)
	}
	_ = netConn.SetDeadline(time.Time{})
	sshClient := ssh.NewClient(sshConn, channels, requests)
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, nil, fmt.Errorf("start sftp session: %w", err)
	}
	return &sftpFileSystem{Client: sftpClient}, sshClient, nil
}

type sftpFileSystem struct {
	*sftp.Client
}

func (s *sftpFileSystem) Open(remotePath string) (ReadSeekCloser, error) {
	return s.Client.Open(remotePath)
}

func (s *sftpFileSystem) OpenFile(remotePath string, flags int, _ os.FileMode) (io.WriteCloser, error) {
	return s.Client.OpenFile(remotePath, flags)
}

func resolveRemotePath(home, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "~" {
		return cleanRemotePath(home)
	}
	if strings.HasPrefix(value, "~/") {
		return cleanRemotePath(path.Join(home, strings.TrimPrefix(value, "~/")))
	}
	if !strings.HasPrefix(value, "/") {
		return cleanRemotePath(path.Join(home, value))
	}
	return cleanRemotePath(value)
}

func cleanRemotePath(value string) string {
	return path.Clean("/" + strings.TrimPrefix(value, "/"))
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(left, right int) bool {
		if entries[left].Kind == "directory" && entries[right].Kind != "directory" {
			return true
		}
		if entries[left].Kind != "directory" && entries[right].Kind == "directory" {
			return false
		}
		return strings.ToLower(entries[left].Name) < strings.ToLower(entries[right].Name)
	})
}

func classifyPreview(remotePath string) (string, string) {
	extension := strings.ToLower(path.Ext(remotePath))
	mimeType := mime.TypeByExtension(extension)
	if mimeType == "" {
		mimeType = "text/plain; charset=utf-8"
	}
	switch extension {
	case ".md", ".markdown", ".mdown", ".mkd":
		return "markdown", "text/markdown; charset=utf-8"
	case ".html", ".htm", ".xhtml", ".svg":
		return "browser", mimeType
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico", ".avif":
		return "image", mimeType
	case ".pdf", ".mp3", ".wav", ".ogg", ".mp4", ".webm":
		return "browser", mimeType
	case ".7z", ".bz2", ".dmg", ".gz", ".iso", ".rar", ".tar", ".tgz", ".xz", ".zip":
		return "unsupported", mimeType
	}
	return "text", mimeType
}

func looksBinary(content []byte) bool {
	return bytes.IndexByte(content, 0) >= 0
}

func editableRemoteFile(remoteFS FileSystem, remotePath string) (os.FileInfo, []byte, error) {
	info, err := remoteFS.Lstat(remotePath)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %q: %w", remotePath, err)
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, ErrUnsupportedEdit
	}
	kind, mimeType := classifyPreview(remotePath)
	textualBrowserFile := kind == "browser" && (strings.HasPrefix(mimeType, "text/") || strings.Contains(mimeType, "svg"))
	if kind != "text" && kind != "markdown" && !textualBrowserFile {
		return nil, nil, ErrUnsupportedEdit
	}
	file, err := remoteFS.Open(remotePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open %q: %w", remotePath, err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, MaxPreviewBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read %q: %w", remotePath, err)
	}
	if len(content) > int(MaxPreviewBytes) {
		return nil, nil, ErrRemoteFileTooLarge
	}
	if looksBinary(content) {
		return nil, nil, ErrUnsupportedEdit
	}
	return info, content, nil
}

func contentRevision(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func temporarySavePath(remotePath string) (string, error) {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return path.Join(path.Dir(remotePath), fmt.Sprintf(".%s.ssh-man-save-%s", path.Base(remotePath), hex.EncodeToString(randomBytes))), nil
}

func availableTarget(directory, name string) string {
	target := filepath.Join(directory, name)
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return target
	}
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for suffix := 1; ; suffix++ {
		candidate := filepath.Join(directory, fmt.Sprintf("%s (%d)%s", stem, suffix, extension))
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

func downloadDirectory(ctx context.Context, remoteFS FileSystem, remotePath, target string) (returnErr error) {
	parent := filepath.Dir(target)
	temp, err := os.MkdirTemp(parent, ".ssh-man-download-")
	if err != nil {
		return err
	}
	if err := os.Chmod(temp, 0o755); err != nil {
		_ = os.RemoveAll(temp)
		return err
	}
	defer func() {
		if returnErr != nil {
			_ = os.RemoveAll(temp)
		}
	}()
	if err := downloadDirectoryContents(ctx, remoteFS, remotePath, temp); err != nil {
		return err
	}
	return os.Rename(temp, target)
}

func downloadDirectoryContents(ctx context.Context, remoteFS FileSystem, remotePath, localPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	items, err := remoteFS.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("list remote folder %q: %w", remotePath, err)
	}
	for _, item := range items {
		if err := validateRemoteEntryName(item.Name()); err != nil {
			return err
		}
		if item.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q: %w", path.Join(remotePath, item.Name()), ErrUnsupportedSymlink)
		}
		remoteChild := path.Join(remotePath, item.Name())
		localChild := filepath.Join(localPath, item.Name())
		if item.IsDir() {
			if err := os.Mkdir(localChild, 0o755); err != nil {
				return err
			}
			if err := downloadDirectoryContents(ctx, remoteFS, remoteChild, localChild); err != nil {
				return err
			}
			continue
		}
		if err := downloadFile(ctx, remoteFS, remoteChild, localChild, item); err != nil {
			return err
		}
	}
	return nil
}

func downloadFile(ctx context.Context, remoteFS FileSystem, remotePath, target string, info os.FileInfo) (returnErr error) {
	remoteFile, err := remoteFS.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file %q: %w", remotePath, err)
	}
	defer remoteFile.Close()

	temp, err := os.CreateTemp(filepath.Dir(target), ".ssh-man-download-")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		if returnErr != nil {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := io.Copy(temp, &contextReader{ctx: ctx, reader: remoteFile}); err != nil {
		_ = temp.Close()
		return fmt.Errorf("download %q: %w", remotePath, err)
	}
	if err := temp.Close(); err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	if err := os.Chmod(tempName, mode); err != nil {
		return err
	}
	if err := os.Chtimes(tempName, time.Now(), info.ModTime()); err != nil {
		return err
	}
	return os.Rename(tempName, target)
}

func validateRemoteEntryName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return fmt.Errorf("remote entry has an unsafe name %q", name)
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func contentTypeFor(remotePath string, sample []byte) string {
	if value := mime.TypeByExtension(strings.ToLower(path.Ext(remotePath))); value != "" {
		return value
	}
	return http.DetectContentType(sample)
}
