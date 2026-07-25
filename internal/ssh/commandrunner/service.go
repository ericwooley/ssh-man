package commandrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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
	ErrNotConnected          = errors.New("the quick command window is not connected")
	ErrCommandAlreadyRunning = errors.New("another command is already running")
)

const maxCompletionItems = 64

type Session interface {
	Run(string, io.Writer) error
	Close() error
}

type Connection interface {
	Getwd() (string, error)
	ReadDir(string) ([]os.FileInfo, error)
	NewSession() (Session, error)
	Close() error
}

type Dialer func(context.Context, serverdomain.Server, string) (Connection, error)

type runningCommand struct {
	session   Session
	cancelled bool
}

type Service struct {
	mu      sync.Mutex
	server  serverdomain.Server
	dial    Dialer
	conn    Connection
	home    string
	running *runningCommand
}

func NewService(server serverdomain.Server) *Service {
	return NewServiceWithDialer(server, dialConnection)
}

func NewServiceWithDialer(server serverdomain.Server, dial Dialer) *Service {
	return &Service{server: server, dial: dial}
}

func (s *Service) Connect(ctx context.Context, passphrase string) (ConnectResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		return ConnectResult{Connected: true, HomePath: s.home}, nil
	}
	conn, err := s.dial(ctx, s.server, passphrase)
	if err != nil {
		if errors.Is(err, auth.ErrPassphraseRequired) {
			return ConnectResult{NeedsPassphrase: true}, nil
		}
		return ConnectResult{}, err
	}
	home, err := conn.Getwd()
	if err != nil {
		_ = conn.Close()
		return ConnectResult{}, fmt.Errorf("find remote home directory: %w", err)
	}
	s.conn = conn
	s.home = path.Clean(home)
	return ConnectResult{Connected: true, HomePath: s.home}, nil
}

func (s *Service) Run(command string) (ExecutionResult, error) {
	if strings.TrimSpace(command) == "" {
		return ExecutionResult{}, fmt.Errorf("command is required")
	}
	s.mu.Lock()
	if s.conn == nil {
		s.mu.Unlock()
		return ExecutionResult{}, ErrNotConnected
	}
	if s.running != nil {
		s.mu.Unlock()
		return ExecutionResult{}, ErrCommandAlreadyRunning
	}
	startedAt := time.Now().UTC()
	session, err := s.conn.NewSession()
	if err != nil {
		s.mu.Unlock()
		endedAt := time.Now().UTC()
		return ExecutionResult{
			ExitCode:  -1,
			StartedAt: startedAt,
			EndedAt:   endedAt,
			Error:     fmt.Sprintf("start remote command: %v", err),
		}, nil
	}
	running := &runningCommand{session: session}
	s.running = running
	s.mu.Unlock()

	output := &limitedBuffer{limit: MaxOutputBytes}
	runErr := session.Run(command, output)
	endedAt := time.Now().UTC()

	s.mu.Lock()
	cancelled := running.cancelled
	if s.running == running {
		s.running = nil
	}
	s.mu.Unlock()

	result := ExecutionResult{
		Output:    output.String(),
		ExitCode:  0,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Truncated: output.Truncated(),
		Cancelled: cancelled,
	}
	if cancelled {
		result.ExitCode = -1
		result.Error = "Command stopped."
		return result, nil
	}
	if runErr == nil {
		return result, nil
	}
	result.Error = runErr.Error()
	result.ExitCode = -1
	var exitError interface{ ExitStatus() int }
	if errors.As(runErr, &exitError) {
		result.ExitCode = exitError.ExitStatus()
	}
	return result, nil
}

func (s *Service) Cancel() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		return nil
	}
	s.running.cancelled = true
	if err := s.running.session.Close(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("stop remote command: %w", err)
	}
	return nil
}

func (s *Service) CompletePath(pathPrefix string) (CompletionResult, error) {
	if len(pathPrefix) > 4096 {
		return CompletionResult{}, fmt.Errorf("completion path is too long")
	}
	s.mu.Lock()
	conn := s.conn
	home := s.home
	s.mu.Unlock()
	if conn == nil {
		return CompletionResult{}, ErrNotConnected
	}

	request, ok := completionRequestFor(pathPrefix, home)
	if !ok {
		return CompletionResult{Items: []Completion{}}, nil
	}
	entries, err := conn.ReadDir(request.directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CompletionResult{Items: []Completion{}}, nil
		}
		return CompletionResult{}, fmt.Errorf("list remote path completions: %w", err)
	}
	items := make([]Completion, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, request.namePrefix) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(request.namePrefix, ".") {
			continue
		}
		kind := "file"
		suffix := ""
		if entry.IsDir() {
			kind = "directory"
			suffix = "/"
		}
		items = append(items, Completion{
			Value: request.typedDirectory + name + suffix,
			Name:  name + suffix,
			Kind:  kind,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind == "directory"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	if len(items) > maxCompletionItems {
		items = items[:maxCompletionItems]
	}
	return CompletionResult{Items: items}, nil
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running != nil {
		s.running.cancelled = true
		_ = s.running.session.Close()
	}
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	s.home = ""
	return err
}

type completionRequest struct {
	directory      string
	typedDirectory string
	namePrefix     string
}

func completionRequestFor(pathPrefix, home string) (completionRequest, bool) {
	if strings.ContainsRune(pathPrefix, '\x00') {
		return completionRequest{}, false
	}
	if pathPrefix == "~" {
		pathPrefix = "~/"
	}
	if strings.HasPrefix(pathPrefix, "~") && !strings.HasPrefix(pathPrefix, "~/") {
		return completionRequest{}, false
	}

	slash := strings.LastIndex(pathPrefix, "/")
	typedDirectory := ""
	namePrefix := pathPrefix
	if slash >= 0 {
		typedDirectory = pathPrefix[:slash+1]
		namePrefix = pathPrefix[slash+1:]
	}

	directory := home
	switch {
	case strings.HasPrefix(typedDirectory, "~/"):
		directory = path.Join(home, strings.TrimPrefix(typedDirectory, "~/"))
	case strings.HasPrefix(typedDirectory, "/"):
		directory = path.Clean(typedDirectory)
	case typedDirectory != "":
		directory = path.Join(home, typedDirectory)
	}
	return completionRequest{
		directory:      directory,
		typedDirectory: typedDirectory,
		namePrefix:     namePrefix,
	}, true
}

type limitedBuffer struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		writeLength := len(data)
		if writeLength > remaining {
			writeLength = remaining
		}
		b.data = append(b.data, data[:writeLength]...)
	}
	if len(data) > remaining {
		b.truncated = true
	}
	return len(data), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func (b *limitedBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}

type sshConnection struct {
	ssh  *ssh.Client
	sftp *sftp.Client
}

func dialConnection(ctx context.Context, server serverdomain.Server, passphrase string) (Connection, error) {
	authMethod, err := sshconnection.AuthMethod(server, passphrase)
	if err != nil {
		return nil, err
	}
	sshClient, err := sshconnection.DialSSH(ctx, server, authMethod)
	if err != nil {
		return nil, err
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("start SFTP for path completion: %w", err)
	}
	return &sshConnection{ssh: sshClient, sftp: sftpClient}, nil
}

func (c *sshConnection) Getwd() (string, error) {
	return c.sftp.Getwd()
}

func (c *sshConnection) ReadDir(remotePath string) ([]os.FileInfo, error) {
	return c.sftp.ReadDir(remotePath)
}

func (c *sshConnection) NewSession() (Session, error) {
	session, err := c.ssh.NewSession()
	if err != nil {
		return nil, err
	}
	return &sshSession{Session: session}, nil
}

func (c *sshConnection) Close() error {
	return errors.Join(c.sftp.Close(), c.ssh.Close())
}

type sshSession struct {
	*ssh.Session
}

func (s *sshSession) Run(command string, output io.Writer) error {
	s.Stdout = output
	s.Stderr = output
	return s.Session.Run(command)
}
