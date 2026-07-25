package commandrunner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
)

type fakeInfo struct {
	name string
	dir  bool
}

func (f fakeInfo) Name() string { return f.name }
func (f fakeInfo) Size() int64  { return 0 }
func (f fakeInfo) Mode() os.FileMode {
	if f.dir {
		return os.ModeDir | 0o755
	}
	return 0o644
}
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

type fakeSession struct {
	output    string
	err       error
	closed    chan struct{}
	closeOnce bool
}

func (s *fakeSession) Run(_ string, output io.Writer) error {
	_, _ = io.Copy(output, bytes.NewBufferString(s.output))
	if s.closed != nil {
		<-s.closed
	}
	return s.err
}

func (s *fakeSession) Close() error {
	if s.closed != nil && !s.closeOnce {
		close(s.closed)
		s.closeOnce = true
	}
	return nil
}

type fakeConnection struct {
	home          string
	directories   map[string][]os.FileInfo
	session       *fakeSession
	newSessionErr error
	closed        bool
}

func (f *fakeConnection) Getwd() (string, error) { return f.home, nil }
func (f *fakeConnection) ReadDir(remotePath string) ([]os.FileInfo, error) {
	entries, ok := f.directories[remotePath]
	if !ok {
		return nil, os.ErrNotExist
	}
	return entries, nil
}
func (f *fakeConnection) NewSession() (Session, error) {
	if f.newSessionErr != nil {
		return nil, f.newSessionErr
	}
	return f.session, nil
}
func (f *fakeConnection) Close() error { f.closed = true; return nil }

func connectedService(t *testing.T, connection *fakeConnection) *Service {
	t.Helper()
	service := NewServiceWithDialer(serverdomain.Server{ID: "server-1"}, func(context.Context, serverdomain.Server, string) (Connection, error) {
		return connection, nil
	})
	result, err := service.Connect(context.Background(), "")
	if err != nil || !result.Connected {
		t.Fatalf("Connect() = %#v, %v", result, err)
	}
	return service
}

func TestConnectRequestsEncryptedKeyPassphrase(t *testing.T) {
	service := NewServiceWithDialer(serverdomain.Server{}, func(context.Context, serverdomain.Server, string) (Connection, error) {
		return nil, auth.ErrPassphraseRequired
	})

	result, err := service.Connect(context.Background(), "")

	if err != nil || !result.NeedsPassphrase || result.Connected {
		t.Fatalf("Connect() = %#v, %v", result, err)
	}
}

func TestCompletePathListsDirectoriesFirstAndPreservesTypedPrefix(t *testing.T) {
	connection := &fakeConnection{
		home: "/home/deploy",
		directories: map[string][]os.FileInfo{
			"/home/deploy/src": {
				fakeInfo{name: "server.go"},
				fakeInfo{name: "services", dir: true},
				fakeInfo{name: ".secret"},
				fakeInfo{name: "README.md"},
			},
		},
	}
	service := connectedService(t, connection)

	result, err := service.CompletePath("src/se")

	if err != nil {
		t.Fatal(err)
	}
	want := []Completion{
		{Value: "src/services/", Name: "services/", Kind: "directory"},
		{Value: "src/server.go", Name: "server.go", Kind: "file"},
	}
	if !reflect.DeepEqual(result.Items, want) {
		t.Fatalf("completions = %#v, want %#v", result.Items, want)
	}
}

func TestCompletePathExpandsHomeAndOnlyShowsHiddenFilesForDotPrefix(t *testing.T) {
	connection := &fakeConnection{
		home: "/home/deploy",
		directories: map[string][]os.FileInfo{
			"/home/deploy": {
				fakeInfo{name: ".env"},
				fakeInfo{name: "app"},
			},
		},
	}
	service := connectedService(t, connection)

	result, err := service.CompletePath("~/.")

	if err != nil || len(result.Items) != 1 || result.Items[0].Value != "~/.env" {
		t.Fatalf("CompletePath() = %#v, %v", result, err)
	}
}

type fakeExitError struct{ code int }

func (f fakeExitError) Error() string   { return "exit status" }
func (f fakeExitError) ExitStatus() int { return f.code }

func TestRunReturnsCombinedOutputAndExitStatus(t *testing.T) {
	service := connectedService(t, &fakeConnection{
		home:    "/home/deploy",
		session: &fakeSession{output: "failure details\n", err: fakeExitError{code: 7}},
	})

	result, err := service.Run("build")

	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "failure details\n" || result.ExitCode != 7 || result.Error == "" {
		t.Fatalf("Run() = %#v", result)
	}
}

func TestRunCapsSavedOutputWithoutBlockingTheRemoteWriter(t *testing.T) {
	service := connectedService(t, &fakeConnection{
		home:    "/home/deploy",
		session: &fakeSession{output: string(bytes.Repeat([]byte("x"), MaxOutputBytes+512))},
	})

	result, err := service.Run("large-output")

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Output) != MaxOutputBytes || !result.Truncated {
		t.Fatalf("output length = %d, truncated = %v", len(result.Output), result.Truncated)
	}
}

func TestCancelStopsTheRunningCommand(t *testing.T) {
	session := &fakeSession{closed: make(chan struct{})}
	service := connectedService(t, &fakeConnection{home: "/home/deploy", session: session})
	resultChannel := make(chan ExecutionResult, 1)
	go func() {
		result, _ := service.Run("tail -f app.log")
		resultChannel <- result
	}()

	deadline := time.Now().Add(time.Second)
	for {
		service.mu.Lock()
		running := service.running != nil
		service.mu.Unlock()
		if running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("command did not start")
		}
	}
	if err := service.Cancel(); err != nil {
		t.Fatal(err)
	}
	result := <-resultChannel
	if !result.Cancelled || result.ExitCode != -1 {
		t.Fatalf("cancelled result = %#v", result)
	}
}

func TestRunRequiresConnectionAndReportsSessionCreationFailureAsOutputResult(t *testing.T) {
	service := NewService(serverdomain.Server{})
	if _, err := service.Run("pwd"); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Run() error = %v", err)
	}

	service = connectedService(t, &fakeConnection{home: "/home/deploy", newSessionErr: errors.New("transport closed")})
	result, err := service.Run("pwd")
	if err != nil || result.ExitCode != -1 || result.Error == "" {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
}
