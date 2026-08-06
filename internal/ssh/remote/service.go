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
)

var (
	ErrNotConnected        = errors.New("remote file explorer is not connected")
	ErrRemoteFileChanged   = errors.New("the remote file changed after it was opened")
	ErrRemoteFileTooLarge  = errors.New("remote files larger than 2 MB cannot be edited")
	ErrUnsupportedEdit     = errors.New("this remote file cannot be edited")
	ErrUnsupportedSymlink  = errors.New("symbolic links are not supported for this operation")
	ErrRemoteFileExists    = errors.New("a remote file with this name already exists")
	ErrUnsupportedUpload   = errors.New("only regular local files can be uploaded")
	ErrUploadDirectory     = errors.New("local directories must be opened before uploading their files")
	ErrLocalFileUnreadable = errors.New("the local file could not be read")
	ErrUploadPermissions   = errors.New("the server did not accept the file permissions")
	ErrIncompleteUpload    = errors.New("the incomplete remote file could not be removed")
	ErrProtectedPath       = errors.New("this remote path cannot be changed")
)

const (
	UploadFailureExists          = "exists"
	UploadFailureDirectory       = "directory"
	UploadFailureUnsupported     = "unsupported"
	UploadFailureLocalPermission = "local-permission"
	UploadFailureMissing         = "missing"
	UploadFailurePermission      = "permission"
	UploadFailurePermissions     = "permissions"
	UploadFailureIncomplete      = "incomplete"
	UploadFailureFailed          = "failed"
)

const uploadProgressInterval = 100 * time.Millisecond

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type FileSystem interface {
	Getwd() (string, error)
	ReadDir(string) ([]os.FileInfo, error)
	Lstat(string) (os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
	Open(string) (ReadSeekCloser, error)
	OpenFile(string, int, os.FileMode) (io.WriteCloser, error)
	Chmod(string, os.FileMode) error
	Mkdir(string) error
	PosixRename(string, string) error
	Remove(string) error
	RemoveDirectory(string) error
	Close() error
}

type Dialer func(context.Context, serverdomain.Server, string) (FileSystem, io.Closer, error)

type Service struct {
	mu            sync.RWMutex
	mutationMu    sync.Mutex
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
	if kind == "image" || kind == "video" || (kind == "browser" && !textualBrowserPreview) || kind == "unsupported" {
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

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()

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

func (s *Service) CreateFolder(directoryPath, name string) (string, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return "", err
	}
	if err := validateRemoteEntryName(strings.TrimSpace(name)); err != nil {
		return "", err
	}
	directoryPath = resolveRemotePath(home, directoryPath)

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()

	if err := requireRemoteDirectory(remoteFS, directoryPath); err != nil {
		return "", err
	}
	target := path.Join(directoryPath, strings.TrimSpace(name))
	if _, err := remoteFS.Lstat(target); err == nil {
		return "", fmt.Errorf("%q already exists", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect %q: %w", target, err)
	}
	if err := remoteFS.Mkdir(target); err != nil {
		return "", fmt.Errorf("create folder %q: %w", target, err)
	}
	if err := remoteFS.Chmod(target, 0o755); err != nil {
		_ = remoteFS.RemoveDirectory(target)
		return "", fmt.Errorf("set permissions on folder %q: %w", target, err)
	}
	return target, nil
}

func (s *Service) Rename(remotePath, name string) (string, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if err := validateRemoteEntryName(name); err != nil {
		return "", err
	}
	remotePath = resolveRemotePath(home, remotePath)
	if err := protectRemotePath(home, remotePath); err != nil {
		return "", err
	}
	target := path.Join(path.Dir(remotePath), name)
	if target == remotePath {
		return remotePath, nil
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()

	if _, err := remoteFS.Lstat(remotePath); err != nil {
		return "", fmt.Errorf("inspect %q: %w", remotePath, err)
	}
	if _, err := remoteFS.Lstat(target); err == nil {
		return "", fmt.Errorf("%q already exists", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect %q: %w", target, err)
	}
	if err := remoteFS.PosixRename(remotePath, target); err != nil {
		return "", fmt.Errorf("rename %q: %w", remotePath, err)
	}
	return target, nil
}

func (s *Service) Copy(remotePaths []string, destinationDirectory string) ([]string, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return nil, err
	}
	destinationDirectory = resolveRemotePath(home, destinationDirectory)

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()

	if err := requireRemoteDirectory(remoteFS, destinationDirectory); err != nil {
		return nil, err
	}
	sources := normalizedRemotePaths(home, remotePaths)
	results := make([]string, 0, len(sources))
	for _, source := range sources {
		if err := protectRemotePath(home, source); err != nil {
			return results, err
		}
		info, err := remoteFS.Lstat(source)
		if err != nil {
			return results, fmt.Errorf("inspect %q: %w", source, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return results, fmt.Errorf("%q: %w", source, ErrUnsupportedSymlink)
		}
		if info.IsDir() && remotePathContains(source, destinationDirectory) {
			return results, fmt.Errorf("cannot copy %q into itself", source)
		}
		target, err := availableRemoteTarget(remoteFS, destinationDirectory, path.Base(source))
		if err != nil {
			return results, err
		}
		if err := copyRemoteNode(remoteFS, source, target, info); err != nil {
			return results, err
		}
		results = append(results, target)
	}
	return results, nil
}

func (s *Service) Move(remotePaths []string, destinationDirectory string) ([]string, error) {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return nil, err
	}
	destinationDirectory = resolveRemotePath(home, destinationDirectory)

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()

	if err := requireRemoteDirectory(remoteFS, destinationDirectory); err != nil {
		return nil, err
	}
	sources := normalizedRemotePaths(home, remotePaths)
	results := make([]string, 0, len(sources))
	for _, source := range sources {
		if err := protectRemotePath(home, source); err != nil {
			return results, err
		}
		info, err := remoteFS.Lstat(source)
		if err != nil {
			return results, fmt.Errorf("inspect %q: %w", source, err)
		}
		if info.IsDir() && remotePathContains(source, destinationDirectory) {
			return results, fmt.Errorf("cannot move %q into itself", source)
		}
		if path.Dir(source) == destinationDirectory {
			results = append(results, source)
			continue
		}
		target, err := availableRemoteTarget(remoteFS, destinationDirectory, path.Base(source))
		if err != nil {
			return results, err
		}
		if err := remoteFS.PosixRename(source, target); err != nil {
			return results, fmt.Errorf("move %q: %w", source, err)
		}
		results = append(results, target)
	}
	return results, nil
}

func (s *Service) Delete(remotePaths []string) error {
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return err
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()

	for _, remotePath := range normalizedRemotePaths(home, remotePaths) {
		if err := protectRemotePath(home, remotePath); err != nil {
			return err
		}
		if err := removeRemoteNode(remoteFS, remotePath); err != nil {
			return fmt.Errorf("delete %q: %w", remotePath, err)
		}
	}
	return nil
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

func (s *Service) Upload(ctx context.Context, localPaths []string, remoteDirectory string) (UploadResult, error) {
	return s.UploadWithProgress(ctx, localPaths, remoteDirectory, nil)
}

func (s *Service) UploadWithProgress(ctx context.Context, localPaths []string, remoteDirectory string, report UploadProgressReporter) (UploadResult, error) {
	result := UploadResult{
		Uploaded: []string{},
		Failures: []UploadFailure{},
	}
	remoteFS, home, err := s.connectedFS()
	if err != nil {
		return result, err
	}
	if len(localPaths) == 0 {
		return result, nil
	}
	remoteDirectory = resolveRemotePath(home, remoteDirectory)
	directoryInfo, err := remoteFS.Stat(remoteDirectory)
	if err != nil {
		return result, fmt.Errorf("inspect upload destination %q: %w", remoteDirectory, err)
	}
	if !directoryInfo.IsDir() {
		return result, fmt.Errorf("upload destination %q is not a directory", remoteDirectory)
	}

	fileSizes, overallBytesTotal := plannedUploadSizes(localPaths)
	overallBytesProcessed := int64(0)
	filesProcessed := 0
	for fileIndex, localPath := range localPaths {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		name := uploadFailureName(localPath)
		fileSize := fileSizes[fileIndex]
		emitUploadProgress(report, UploadProgress{
			FileIndex:             fileIndex,
			Name:                  name,
			Status:                UploadStatusTransferring,
			TotalBytes:            fileSize,
			OverallBytesProcessed: overallBytesProcessed,
			OverallBytesTotal:     overallBytesTotal,
			FilesProcessed:        filesProcessed,
			FilesTotal:            len(localPaths),
		})
		bytesTransferred := int64(0)
		remotePath, uploadErr := uploadFile(ctx, remoteFS, localPath, remoteDirectory, func(transferred int64) {
			bytesTransferred = transferred
			emitUploadProgress(report, UploadProgress{
				FileIndex:             fileIndex,
				Name:                  name,
				Status:                UploadStatusTransferring,
				BytesTransferred:      transferred,
				TotalBytes:            fileSize,
				OverallBytesProcessed: overallBytesProcessed + minInt64(transferred, fileSize),
				OverallBytesTotal:     overallBytesTotal,
				FilesProcessed:        filesProcessed,
				FilesTotal:            len(localPaths),
			})
		})
		filesProcessed++
		overallBytesProcessed += fileSize
		if uploadErr != nil {
			if errors.Is(uploadErr, context.Canceled) || errors.Is(uploadErr, context.DeadlineExceeded) {
				return result, uploadErr
			}
			failureCode := uploadFailureCode(uploadErr)
			result.Failures = append(result.Failures, UploadFailure{
				Name: name,
				Code: failureCode,
			})
			emitUploadProgress(report, UploadProgress{
				FileIndex:             fileIndex,
				Name:                  name,
				Status:                UploadStatusFailed,
				BytesTransferred:      bytesTransferred,
				TotalBytes:            fileSize,
				OverallBytesProcessed: overallBytesProcessed,
				OverallBytesTotal:     overallBytesTotal,
				FilesProcessed:        filesProcessed,
				FilesTotal:            len(localPaths),
				FailureCode:           failureCode,
			})
			continue
		}
		result.Uploaded = append(result.Uploaded, remotePath)
		emitUploadProgress(report, UploadProgress{
			FileIndex:             fileIndex,
			Name:                  name,
			Status:                UploadStatusCompleted,
			BytesTransferred:      fileSize,
			TotalBytes:            fileSize,
			OverallBytesProcessed: overallBytesProcessed,
			OverallBytesTotal:     overallBytesTotal,
			FilesProcessed:        filesProcessed,
			FilesTotal:            len(localPaths),
		})
	}
	return result, nil
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

func uploadFile(ctx context.Context, remoteFS FileSystem, localPath, remoteDirectory string, report func(int64)) (string, error) {
	if strings.TrimSpace(localPath) == "" {
		return "", ErrUnsupportedUpload
	}
	localInfo, err := os.Stat(localPath)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return "", fmt.Errorf("%w: inspect local file %q: %v", ErrLocalFileUnreadable, localPath, err)
		}
		return "", fmt.Errorf("inspect local file %q: %w", localPath, err)
	}
	if localInfo.IsDir() {
		return "", fmt.Errorf("%q: %w", localPath, ErrUploadDirectory)
	}
	if !localInfo.Mode().IsRegular() {
		return "", fmt.Errorf("%q: %w", localPath, ErrUnsupportedUpload)
	}
	name := filepath.Base(localPath)
	if err := validateRemoteEntryName(name); err != nil {
		return "", fmt.Errorf("upload %q: %w", localPath, err)
	}
	remotePath := path.Join(remoteDirectory, name)
	if _, err := remoteFS.Lstat(remotePath); err == nil {
		return "", fmt.Errorf("%q: %w", remotePath, ErrRemoteFileExists)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect upload target %q: %w", remotePath, err)
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return "", fmt.Errorf("%w: open local file %q: %v", ErrLocalFileUnreadable, localPath, err)
		}
		return "", fmt.Errorf("open local file %q: %w", localPath, err)
	}
	defer localFile.Close()

	remoteFile, err := remoteFS.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("%q: %w", remotePath, ErrRemoteFileExists)
		}
		if _, targetErr := remoteFS.Lstat(remotePath); targetErr == nil {
			return "", fmt.Errorf("%q: %w", remotePath, ErrRemoteFileExists)
		}
		return "", fmt.Errorf("create remote file %q: %w", remotePath, err)
	}
	mode := safeUploadMode(localInfo.Mode())
	if err := remoteFS.Chmod(remotePath, mode); err != nil {
		uploadErr := fmt.Errorf("%w for %q: %v", ErrUploadPermissions, remotePath, err)
		return "", discardIncompleteUpload(remoteFS, remotePath, remoteFile, uploadErr)
	}
	progressWriter := &uploadProgressWriter{
		writer:     remoteFile,
		total:      localInfo.Size(),
		report:     report,
		lastReport: time.Now(),
	}
	if _, err := io.Copy(progressWriter, &contextReader{ctx: ctx, reader: localFile}); err != nil {
		uploadErr := fmt.Errorf("upload %q: %w", localPath, err)
		return "", discardIncompleteUpload(remoteFS, remotePath, remoteFile, uploadErr)
	}
	if err := remoteFile.Close(); err != nil {
		uploadErr := fmt.Errorf("close remote file %q: %w", remotePath, err)
		return "", discardIncompleteUpload(remoteFS, remotePath, nil, uploadErr)
	}
	return remotePath, nil
}

type uploadProgressWriter struct {
	writer      io.Writer
	total       int64
	transferred int64
	report      func(int64)
	lastReport  time.Time
}

func (w *uploadProgressWriter) Write(content []byte) (int, error) {
	written, err := w.writer.Write(content)
	w.transferred += int64(written)
	now := time.Now()
	if w.report != nil && (err != nil || w.transferred >= w.total || now.Sub(w.lastReport) >= uploadProgressInterval) {
		w.report(w.transferred)
		w.lastReport = now
	}
	return written, err
}

func plannedUploadSizes(localPaths []string) ([]int64, int64) {
	sizes := make([]int64, len(localPaths))
	total := int64(0)
	for index, localPath := range localPaths {
		info, err := os.Stat(localPath)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		sizes[index] = info.Size()
		total += info.Size()
	}
	return sizes, total
}

func emitUploadProgress(report UploadProgressReporter, progress UploadProgress) {
	if report != nil {
		report(progress)
	}
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func discardIncompleteUpload(remoteFS FileSystem, remotePath string, remoteFile io.Closer, uploadErr error) error {
	if remoteFile != nil {
		_ = remoteFile.Close()
	}
	if err := remoteFS.Remove(remotePath); err != nil {
		return fmt.Errorf("%w at %q: %v", ErrIncompleteUpload, remotePath, err)
	}
	return uploadErr
}

func safeUploadMode(localMode os.FileMode) os.FileMode {
	mode := localMode.Perm() & 0o755
	if mode == 0 {
		return 0o644
	}
	return mode
}

func uploadFailureName(localPath string) string {
	if strings.TrimSpace(localPath) == "" {
		return "Dropped item"
	}
	name := filepath.Base(localPath)
	if name == "." || name == string(filepath.Separator) {
		return "Dropped item"
	}
	return name
}

func uploadFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrRemoteFileExists):
		return UploadFailureExists
	case errors.Is(err, ErrUploadDirectory):
		return UploadFailureDirectory
	case errors.Is(err, ErrUnsupportedUpload):
		return UploadFailureUnsupported
	case errors.Is(err, ErrLocalFileUnreadable):
		return UploadFailureLocalPermission
	case errors.Is(err, os.ErrNotExist):
		return UploadFailureMissing
	case errors.Is(err, ErrUploadPermissions):
		return UploadFailurePermissions
	case errors.Is(err, ErrIncompleteUpload):
		return UploadFailureIncomplete
	case errors.Is(err, os.ErrPermission):
		return UploadFailurePermission
	default:
		return UploadFailureFailed
	}
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
	sshClient, err := sshconnection.DialSSH(ctx, server, authMethod)
	if err != nil {
		return nil, nil, err
	}
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

func (s *sftpFileSystem) RemoveDirectory(remotePath string) error {
	return s.Client.RemoveDirectory(remotePath)
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
	if videoType := videoMIMEType(extension); videoType != "" {
		return "video", videoType
	}
	switch extension {
	case ".md", ".markdown", ".mdown", ".mkd":
		return "markdown", "text/markdown; charset=utf-8"
	case ".html", ".htm", ".xhtml", ".svg":
		return "browser", mimeType
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico", ".avif":
		return "image", mimeType
	case ".pdf", ".mp3", ".wav", ".ogg":
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

func requireRemoteDirectory(remoteFS FileSystem, remotePath string) error {
	info, err := remoteFS.Lstat(remotePath)
	if err != nil {
		return fmt.Errorf("inspect destination %q: %w", remotePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a folder", remotePath)
	}
	return nil
}

func protectRemotePath(home, remotePath string) error {
	if remotePath == "/" || remotePath == cleanRemotePath(home) {
		return fmt.Errorf("%q: %w", remotePath, ErrProtectedPath)
	}
	return nil
}

func normalizedRemotePaths(home string, values []string) []string {
	paths := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		remotePath := resolveRemotePath(home, value)
		if _, exists := seen[remotePath]; exists {
			continue
		}
		seen[remotePath] = struct{}{}
		paths = append(paths, remotePath)
	}
	sort.SliceStable(paths, func(left, right int) bool {
		if len(paths[left]) == len(paths[right]) {
			return paths[left] < paths[right]
		}
		return len(paths[left]) < len(paths[right])
	})
	filtered := make([]string, 0, len(paths))
	for _, candidate := range paths {
		nested := false
		for _, parent := range filtered {
			if remotePathContains(parent, candidate) {
				nested = true
				break
			}
		}
		if !nested {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func remotePathContains(parent, candidate string) bool {
	parent = cleanRemotePath(parent)
	candidate = cleanRemotePath(candidate)
	if parent == "/" {
		return true
	}
	return candidate == parent || strings.HasPrefix(candidate, parent+"/")
}

func availableRemoteTarget(remoteFS FileSystem, directory, name string) (string, error) {
	target := path.Join(directory, name)
	if _, err := remoteFS.Lstat(target); os.IsNotExist(err) {
		return target, nil
	} else if err != nil {
		return "", fmt.Errorf("inspect %q: %w", target, err)
	}

	extension := path.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	if stem == "" {
		stem = name
		extension = ""
	}
	for suffix := 1; ; suffix++ {
		copySuffix := " copy"
		if suffix > 1 {
			copySuffix = fmt.Sprintf(" copy %d", suffix)
		}
		candidate := path.Join(directory, stem+copySuffix+extension)
		if _, err := remoteFS.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect %q: %w", candidate, err)
		}
	}
}

func copyRemoteNode(remoteFS FileSystem, source, target string, info os.FileInfo) (returnErr error) {
	if info.IsDir() {
		if err := remoteFS.Mkdir(target); err != nil {
			return fmt.Errorf("create folder %q: %w", target, err)
		}
		defer func() {
			if returnErr != nil {
				_ = removeRemoteNode(remoteFS, target)
			}
		}()
		if err := remoteFS.Chmod(target, info.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve permissions on %q: %w", target, err)
		}
		items, err := remoteFS.ReadDir(source)
		if err != nil {
			return fmt.Errorf("list %q: %w", source, err)
		}
		for _, item := range items {
			if err := validateRemoteEntryName(item.Name()); err != nil {
				return err
			}
			if item.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%q: %w", path.Join(source, item.Name()), ErrUnsupportedSymlink)
			}
			if err := copyRemoteNode(remoteFS, path.Join(source, item.Name()), path.Join(target, item.Name()), item); err != nil {
				return err
			}
		}
		return nil
	}

	sourceFile, err := remoteFS.Open(source)
	if err != nil {
		return fmt.Errorf("open %q: %w", source, err)
	}
	defer sourceFile.Close()
	targetFile, err := remoteFS.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %q: %w", target, err)
	}
	keepTarget := false
	defer func() {
		if !keepTarget {
			_ = remoteFS.Remove(target)
		}
	}()
	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		_ = targetFile.Close()
		return fmt.Errorf("copy %q: %w", source, err)
	}
	if err := targetFile.Close(); err != nil {
		return fmt.Errorf("close %q: %w", target, err)
	}
	if err := remoteFS.Chmod(target, info.Mode().Perm()); err != nil {
		return fmt.Errorf("preserve permissions on %q: %w", target, err)
	}
	keepTarget = true
	return nil
}

func removeRemoteNode(remoteFS FileSystem, remotePath string) error {
	info, err := remoteFS.Lstat(remotePath)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return remoteFS.Remove(remotePath)
	}
	items, err := remoteFS.ReadDir(remotePath)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := validateRemoteEntryName(item.Name()); err != nil {
			return err
		}
		if err := removeRemoteNode(remoteFS, path.Join(remotePath, item.Name())); err != nil {
			return err
		}
	}
	return remoteFS.RemoveDirectory(remotePath)
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
	extension := strings.ToLower(path.Ext(remotePath))
	if videoType := videoMIMEType(extension); videoType != "" {
		return videoType
	}
	if value := mime.TypeByExtension(extension); value != "" {
		return value
	}
	return http.DetectContentType(sample)
}

func videoMIMEType(extension string) string {
	switch extension {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return ""
	}
}
