package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxRequestBytes = 1 << 20

type Server struct {
	socketPath string
	backend    Backend

	mu         sync.Mutex
	listener   net.Listener
	httpServer *http.Server
	socketInfo os.FileInfo
}

func NewServer(socketPath string, backend Backend) *Server {
	return &Server{socketPath: socketPath, backend: backend}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o700); err != nil {
		return fmt.Errorf("create control socket directory: %w", err)
	}
	if connection, err := net.DialTimeout("unix", s.socketPath, 100*time.Millisecond); err == nil {
		_ = connection.Close()
		return fmt.Errorf("SSH Man control socket is already active")
	}
	if info, err := os.Lstat(s.socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("refusing to replace non-socket control path %q", s.socketPath)
		}
		if err := os.Remove(s.socketPath); err != nil {
			return fmt.Errorf("remove stale control socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect control socket: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on control socket: %w", err)
	}
	if unixListener, ok := listener.(*net.UnixListener); ok {
		// Remove the socket ourselves after verifying its inode. The default
		// close behavior unlinks by path and could delete a replacement socket.
		unixListener.SetUnlinkOnClose(false)
	}
	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(s.socketPath)
		return fmt.Errorf("secure control socket: %w", err)
	}
	socketInfo, err := os.Lstat(s.socketPath)
	if err != nil {
		_ = listener.Close()
		_ = os.Remove(s.socketPath)
		return fmt.Errorf("inspect control socket after listen: %w", err)
	}

	httpServer := &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.listener = listener
	s.httpServer = httpServer
	s.socketInfo = socketInfo
	go func() {
		_ = httpServer.Serve(listener)
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	httpServer := s.httpServer
	listener := s.listener
	ownedSocketInfo := s.socketInfo
	s.httpServer = nil
	s.listener = nil
	s.socketInfo = nil
	s.mu.Unlock()

	var stopErr error
	if httpServer != nil {
		stopErr = httpServer.Shutdown(ctx)
	}
	if listener != nil {
		_ = listener.Close()
	}
	if ownedSocketInfo != nil {
		if info, err := os.Lstat(s.socketPath); err == nil {
			if !os.SameFile(info, ownedSocketInfo) {
				if stopErr == nil {
					stopErr = fmt.Errorf("control socket was replaced before shutdown; refusing to remove %q", s.socketPath)
				}
			} else if info.Mode()&os.ModeSocket == 0 {
				if stopErr == nil {
					stopErr = fmt.Errorf("owned control path is no longer a socket: %q", s.socketPath)
				}
			} else if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) && stopErr == nil {
				stopErr = err
			}
		} else if !errors.Is(err, os.ErrNotExist) && stopErr == nil {
			stopErr = err
		}
	}
	return stopErr
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	if request.Method != http.MethodPost || request.URL.Path != "/v1/command" {
		s.writeError(writer, http.StatusNotFound, "not_found", "control endpoint not found")
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var command Request
	if err := decoder.Decode(&command); err != nil {
		s.writeError(writer, http.StatusBadRequest, "invalid_request", "invalid control request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		s.writeError(writer, http.StatusBadRequest, "invalid_request", "control request must contain one JSON object")
		return
	}
	if command.ProtocolVersion != ProtocolVersion {
		s.writeError(writer, http.StatusConflict, "protocol_mismatch", fmt.Sprintf("app uses protocol %d, CLI requested %d", ProtocolVersion, command.ProtocolVersion))
		return
	}

	result, err := s.dispatch(request.Context(), command)
	if err != nil {
		s.writeError(writer, http.StatusBadRequest, "operation_failed", err.Error())
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		s.writeError(writer, http.StatusInternalServerError, "encode_failed", "could not encode control response")
		return
	}
	writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(writer).Encode(Response{ProtocolVersion: ProtocolVersion, Data: encoded})
}

func (s *Server) dispatch(ctx context.Context, request Request) (any, error) {
	switch request.Command {
	case "ping":
		return map[string]any{"ready": true}, nil
	case "state":
		return call0(ctx, s.backend.State)
	case "server.save":
		if request.Server == nil {
			return nil, fmt.Errorf("server payload is required")
		}
		return call1(ctx, s.backend.SaveServer, *request.Server)
	case "server.delete":
		return nil, callVoid1(ctx, s.backend.DeleteServer, request.ServerID)
	case "configuration.save":
		if request.Configuration == nil {
			return nil, fmt.Errorf("configuration payload is required")
		}
		return call1(ctx, s.backend.SaveConfiguration, *request.Configuration)
	case "configuration.delete":
		return nil, callVoid1(ctx, s.backend.DeleteConfiguration, request.ConfigurationID)
	case "session.start":
		return call1(ctx, s.backend.Start, request.ConfigurationID)
	case "session.start_server":
		return callBulk(ctx, s.backend.StartServer, request.ServerID)
	case "session.stop":
		return call1(ctx, s.backend.Stop, request.ConfigurationID)
	case "session.stop_server":
		return callBulk(ctx, s.backend.StopServer, request.ServerID)
	case "session.retry":
		return call1(ctx, s.backend.Retry, request.ConfigurationID)
	case "session.unlock":
		if s.backend.Unlock == nil {
			return nil, fmt.Errorf("command is unavailable")
		}
		return s.backend.Unlock(ctx, request.ConfigurationID, request.Secret)
	case "session.history":
		return call1(ctx, s.backend.History, request.ConfigurationID)
	case "browser.list":
		return call0(ctx, s.backend.DiscoverBrowsers)
	case "browser.preview":
		if s.backend.PreviewBrowser == nil {
			return nil, fmt.Errorf("command is unavailable")
		}
		return s.backend.PreviewBrowser(ctx, request.ConfigurationID, request.BrowserID)
	case "browser.launch":
		if s.backend.LaunchBrowser == nil {
			return nil, fmt.Errorf("command is unavailable")
		}
		return nil, s.backend.LaunchBrowser(ctx, request.ConfigurationID, request.BrowserID)
	case "preferences.save":
		if request.Preferences == nil {
			return nil, fmt.Errorf("preferences payload is required")
		}
		return call1(ctx, s.backend.SavePreferences, *request.Preferences)
	case "app.show":
		return nil, callVoid0(s.backend.Show)
	case "app.hide":
		return nil, callVoid0(s.backend.Hide)
	case "app.quit":
		if s.backend.Quit == nil {
			return nil, fmt.Errorf("command is unavailable")
		}
		go func() {
			time.Sleep(25 * time.Millisecond)
			_ = s.backend.Quit()
		}()
		return map[string]bool{"quitting": true}, nil
	default:
		return nil, fmt.Errorf("unknown control command %q", request.Command)
	}
}

func (s *Server) writeError(writer http.ResponseWriter, status int, code string, message string) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(Response{
		ProtocolVersion: ProtocolVersion,
		Error:           &Error{Code: code, Message: message},
	})
}

func call0[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if fn == nil {
		return zero, fmt.Errorf("command is unavailable")
	}
	return fn(ctx)
}

func call1[I, O any](ctx context.Context, fn func(context.Context, I) (O, error), input I) (O, error) {
	var zero O
	if fn == nil {
		return zero, fmt.Errorf("command is unavailable")
	}
	return fn(ctx, input)
}

func callVoid0(fn func() error) error {
	if fn == nil {
		return fmt.Errorf("command is unavailable")
	}
	return fn()
}

func callVoid1[I any](ctx context.Context, fn func(context.Context, I) error, input I) error {
	if fn == nil {
		return fmt.Errorf("command is unavailable")
	}
	return fn(ctx, input)
}

func callBulk(ctx context.Context, fn func(context.Context, string) BulkResult, id string) (BulkResult, error) {
	if fn == nil {
		return BulkResult{}, fmt.Errorf("command is unavailable")
	}
	return fn(ctx, id), nil
}
