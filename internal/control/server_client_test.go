package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestServerClientRoundTrip(t *testing.T) {
	t.Parallel()

	socketPath := testSocketPath(t)
	server := NewServer(socketPath, Backend{
		State: func(context.Context) (State, error) {
			return State{
				Servers: []ServerRecord{{
					Server:         serverdomain.Server{ID: "server-1", Name: "Production"},
					Configurations: nil,
				}},
				Sessions: []sessiondomain.RuntimeSession{},
			}, nil
		},
	})
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	client := NewClient(socketPath, time.Second)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	var state State
	if err := client.Call(context.Background(), Request{Command: "state"}, &state); err != nil {
		t.Fatalf("Call(state) error = %v", err)
	}
	if len(state.Servers) != 1 || state.Servers[0].Server.ID != "server-1" {
		t.Fatalf("state.Servers = %#v, want server-1", state.Servers)
	}
	if state.Sessions == nil {
		t.Fatal("state.Sessions is nil, want an empty JSON array")
	}

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Stat(socket) error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("socket permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestServerPassesRequestContextToBackendOperations(t *testing.T) {
	type contextKey struct{}
	marker := &struct{}{}
	var received context.Context
	record := func(ctx context.Context) {
		received = ctx
	}

	backend := Backend{
		State: func(ctx context.Context) (State, error) {
			record(ctx)
			return State{}, nil
		},
		SaveServer: func(ctx context.Context, value serverdomain.Server) (serverdomain.Server, error) {
			record(ctx)
			return value, nil
		},
		DeleteServer: func(ctx context.Context, _ string) error {
			record(ctx)
			return nil
		},
		SaveConfiguration: func(ctx context.Context, value configdomain.ConnectionConfiguration) (configdomain.ConnectionConfiguration, error) {
			record(ctx)
			return value, nil
		},
		DeleteConfiguration: func(ctx context.Context, _ string) error {
			record(ctx)
			return nil
		},
		Start: func(ctx context.Context, _ string) (sessiondomain.RuntimeSession, error) {
			record(ctx)
			return sessiondomain.RuntimeSession{}, nil
		},
		StartServer: func(ctx context.Context, _ string) BulkResult {
			record(ctx)
			return BulkResult{}
		},
		Stop: func(ctx context.Context, _ string) (sessiondomain.RuntimeSession, error) {
			record(ctx)
			return sessiondomain.RuntimeSession{}, nil
		},
		StopServer: func(ctx context.Context, _ string) BulkResult {
			record(ctx)
			return BulkResult{}
		},
		Retry: func(ctx context.Context, _ string) (sessiondomain.RuntimeSession, error) {
			record(ctx)
			return sessiondomain.RuntimeSession{}, nil
		},
		Unlock: func(ctx context.Context, _, _ string) (sessiondomain.RuntimeSession, error) {
			record(ctx)
			return sessiondomain.RuntimeSession{}, nil
		},
		History: func(ctx context.Context, _ string) ([]sessiondomain.SessionHistoryEntry, error) {
			record(ctx)
			return []sessiondomain.SessionHistoryEntry{}, nil
		},
		DiscoverBrowsers: func(ctx context.Context) ([]browser.BrowserOption, error) {
			record(ctx)
			return []browser.BrowserOption{}, nil
		},
		PreviewBrowser: func(ctx context.Context, _, _ string) (browser.LaunchPreview, error) {
			record(ctx)
			return browser.LaunchPreview{}, nil
		},
		LaunchBrowser: func(ctx context.Context, _, _ string) error {
			record(ctx)
			return nil
		},
		SavePreferences: func(ctx context.Context, value preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error) {
			record(ctx)
			return value, nil
		},
	}

	tests := []struct {
		name    string
		request Request
	}{
		{name: "state", request: Request{Command: "state"}},
		{name: "save server", request: Request{Command: "server.save", Server: &serverdomain.Server{}}},
		{name: "delete server", request: Request{Command: "server.delete", ServerID: "server-1"}},
		{name: "save configuration", request: Request{Command: "configuration.save", Configuration: &configdomain.ConnectionConfiguration{}}},
		{name: "delete configuration", request: Request{Command: "configuration.delete", ConfigurationID: "tunnel-1"}},
		{name: "start session", request: Request{Command: "session.start", ConfigurationID: "tunnel-1"}},
		{name: "start server", request: Request{Command: "session.start_server", ServerID: "server-1"}},
		{name: "stop session", request: Request{Command: "session.stop", ConfigurationID: "tunnel-1"}},
		{name: "stop server", request: Request{Command: "session.stop_server", ServerID: "server-1"}},
		{name: "retry session", request: Request{Command: "session.retry", ConfigurationID: "tunnel-1"}},
		{name: "unlock session", request: Request{Command: "session.unlock", ConfigurationID: "tunnel-1", Secret: "secret"}},
		{name: "session history", request: Request{Command: "session.history", ConfigurationID: "tunnel-1"}},
		{name: "list browsers", request: Request{Command: "browser.list"}},
		{name: "preview browser", request: Request{Command: "browser.preview", ConfigurationID: "tunnel-1", BrowserID: "browser-1"}},
		{name: "launch browser", request: Request{Command: "browser.launch", ConfigurationID: "tunnel-1", BrowserID: "browser-1"}},
		{name: "save preferences", request: Request{Command: "preferences.save", Preferences: &preferencesdomain.UserPreference{}}},
	}

	server := NewServer("", backend)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			received = nil
			test.request.ProtocolVersion = ProtocolVersion
			body, err := json.Marshal(test.request)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, marker))
			cancel()
			httpRequest := httptest.NewRequest(http.MethodPost, "/v1/command", bytes.NewReader(body)).WithContext(ctx)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httpRequest)

			if response.Code != http.StatusOK {
				t.Fatalf("response status = %d, body = %s", response.Code, response.Body.String())
			}
			if received == nil {
				t.Fatal("backend did not receive a context")
			}
			if received.Value(contextKey{}) != marker {
				t.Fatal("backend context did not preserve the request value")
			}
			if !errors.Is(received.Err(), context.Canceled) {
				t.Fatalf("backend context error = %v, want context canceled", received.Err())
			}
		})
	}
}

func TestClientReturnsStructuredRemoteError(t *testing.T) {
	t.Parallel()

	socketPath := testSocketPath(t)
	server := NewServer(socketPath, Backend{})
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Stop(ctx)
	})

	err := NewClient(socketPath, time.Second).Call(
		context.Background(),
		Request{Command: "not-a-command", Secret: "do-not-echo"},
		nil,
	)
	var remoteError *RemoteError
	if !errors.As(err, &remoteError) {
		t.Fatalf("Call() error = %T %v, want *RemoteError", err, err)
	}
	if remoteError.Code != "operation_failed" {
		t.Fatalf("RemoteError.Code = %q, want operation_failed", remoteError.Code)
	}
	if remoteError.Message == "" {
		t.Fatal("RemoteError.Message is empty")
	}
	if containsSecret(remoteError.Message, "do-not-echo") {
		t.Fatalf("remote error leaked request secret: %q", remoteError.Message)
	}
}

func TestClientReturnsTypedProtocolMismatch(t *testing.T) {
	t.Parallel()

	client := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body, err := json.Marshal(Response{ProtocolVersion: ProtocolVersion + 1})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusConflict,
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
		}, nil
	})}}

	err := client.Call(context.Background(), Request{Command: "ping"}, nil)
	var mismatch *ProtocolMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("Call() error = %T %v, want *ProtocolMismatchError", err, err)
	}
	if mismatch.AppVersion != ProtocolVersion+1 || mismatch.CLIVersion != ProtocolVersion {
		t.Fatalf("protocol mismatch = %+v", mismatch)
	}
}

func TestClientWaitHonorsContext(t *testing.T) {
	t.Parallel()

	client := NewClient(filepath.Join(t.TempDir(), "missing.sock"), 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	err := client.Wait(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() error = %v, want deadline exceeded", err)
	}
}

func TestServerRefusesToReplaceNonSocketPath(t *testing.T) {
	t.Parallel()

	socketPath := testSocketPath(t)
	if err := os.WriteFile(socketPath, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	server := NewServer(socketPath, Backend{})
	err := server.Start()
	if err == nil {
		t.Fatal("Start() error = nil, want refusal to replace a regular file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() after failed start error = %v", err)
	}
	contents, readErr := os.ReadFile(socketPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(contents) != "keep me" {
		t.Fatalf("regular file contents = %q, want preserved", contents)
	}
}

func TestServerStopPreservesReplacementSocket(t *testing.T) {
	t.Parallel()

	socketPath := testSocketPath(t)
	server := NewServer(socketPath, Backend{})
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := os.Remove(socketPath); err != nil {
		t.Fatalf("remove owned socket for replacement test: %v", err)
	}
	replacement, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on replacement socket: %v", err)
	}
	if unixListener, ok := replacement.(*net.UnixListener); ok {
		unixListener.SetUnlinkOnClose(false)
	}
	t.Cleanup(func() {
		_ = replacement.Close()
		_ = os.Remove(socketPath)
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = server.Stop(ctx)
	if err == nil || !strings.Contains(err.Error(), "was replaced") {
		t.Fatalf("Stop() error = %v, want replacement refusal", err)
	}
	if _, err := os.Lstat(socketPath); err != nil {
		t.Fatalf("replacement socket was removed: %v", err)
	}
}

func containsSecret(value string, secret string) bool {
	for i := 0; i+len(secret) <= len(value); i++ {
		if value[i:i+len(secret)] == secret {
			return true
		}
	}
	return false
}

func testSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ssh-man-control-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "control.sock")
}
