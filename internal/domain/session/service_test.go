package session

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
)

type stubConfigStore struct {
	item  configdomain.ConnectionConfiguration
	items []configdomain.ConnectionConfiguration
}

func (s stubConfigStore) Get(context.Context, string) (configdomain.ConnectionConfiguration, error) {
	return s.item, nil
}

func (s stubConfigStore) ListByServer(context.Context, string) ([]configdomain.ConnectionConfiguration, error) {
	if s.items != nil {
		return s.items, nil
	}
	if s.item.ID == "" {
		return nil, nil
	}
	return []configdomain.ConnectionConfiguration{s.item}, nil
}

type stubServerStore struct {
	item serverdomain.Server
}

func (s stubServerStore) Get(context.Context, string) (serverdomain.Server, error) {
	return s.item, nil
}

type errorServerStore struct {
	err error
}

func (s errorServerStore) Get(context.Context, string) (serverdomain.Server, error) {
	return serverdomain.Server{}, s.err
}

type stubHistoryStore struct {
	entries []SessionHistoryEntry
}

func (s *stubHistoryStore) Add(_ context.Context, entry SessionHistoryEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *stubHistoryStore) ListByConfiguration(_ context.Context, configurationID string) ([]SessionHistoryEntry, error) {
	entries := make([]SessionHistoryEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.ConfigurationID == configurationID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

type stubRunner struct {
	startErr  error
	stopErr   error
	started   bool
	stopped   bool
	stopCalls int
	onDisc    func(error)
	boundPort int
}

func (s *stubRunner) Start() error {
	s.started = true
	return s.startErr
}

func (s *stubRunner) Stop() error {
	s.stopped = true
	s.stopCalls++
	return s.stopErr
}

func (s *stubRunner) BoundPort() int {
	if s.boundPort != 0 {
		return s.boundPort
	}
	return 1080
}

func (s *stubRunner) Disconnect(err error) {
	if s.onDisc != nil {
		s.onDisc(err)
	}
}

type stubFactory struct {
	runners []*stubRunner
}

func (s *stubFactory) New(_ serverdomain.Server, _ configdomain.ConnectionConfiguration, _ string, onDisconnect func(error)) Runner {
	if len(s.runners) == 0 {
		return &stubRunner{}
	}
	runner := s.runners[0]
	s.runners = s.runners[1:]
	runner.onDisc = onDisconnect
	return runner
}

func TestStartRequiresUnlockWhenKeyIsEncrypted(t *testing.T) {
	runtimes := NewRuntimeStore()
	history := &stubHistoryStore{}
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080, AutoReconnectEnabled: true}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		history,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{startErr: auth.ErrPassphraseRequired}}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusNeedsAttention {
		t.Fatalf("expected needs attention state, got %s", state.Status)
	}
	if !state.NeedsUserInput {
		t.Fatal("expected user input to be required")
	}
	if len(history.entries) != 1 || history.entries[0].Outcome != OutcomeFailedAuth {
		t.Fatalf("expected failed auth history entry, got %+v", history.entries)
	}
}

func TestHandleDisconnectMovesToFailedWithoutReconnect(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(nil, nil, nil, runtimes)
	configuration := configdomain.ConnectionConfiguration{ID: "config-1", AutoReconnectEnabled: false}
	runtimes.SetWithToken(RuntimeSession{ConfigurationID: "config-1", Status: StatusConnected}, nil, "", "token-1")

	service.handleDisconnect(configuration, "", "token-1", errors.New("network dropped"))

	state, ok := runtimes.Get("config-1")
	if !ok {
		t.Fatal("expected runtime state to be stored")
	}
	if state.Status != StatusFailed {
		t.Fatalf("expected failed state, got %s", state.Status)
	}
	if state.LastError != "network dropped" {
		t.Fatalf("expected disconnect reason to be stored, got %q", state.LastError)
	}
}

func TestStopReturnsStoppedStateWhenRunnerExists(t *testing.T) {
	runtimes := NewRuntimeStore()
	history := &stubHistoryStore{}
	service := NewService(nil, nil, history, runtimes)
	runner := &stubRunner{}
	startedAt := time.Now().UTC().Add(-time.Minute)
	runtimes.Set(RuntimeSession{ConfigurationID: "config-1", Status: StatusConnected, StartedAt: startedAt, LastStateChangeAt: startedAt}, runner, "")

	state, err := service.Stop(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("stop tunnel: %v", err)
	}
	if !runner.stopped {
		t.Fatal("expected runner stop to be called")
	}
	if state.Status != StatusStopped {
		t.Fatalf("expected stopped state, got %s", state.Status)
	}
	if len(history.entries) != 1 || history.entries[0].Outcome != OutcomeStopped {
		t.Fatalf("expected stopped history entry, got %+v", history.entries)
	}
}

func TestHandleDisconnectReconnectsWhenEnabled(t *testing.T) {
	runtimes := NewRuntimeStore()
	history := &stubHistoryStore{}
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080, AutoReconnectEnabled: true}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		history,
		runtimes,
	)
	initialRunner := &stubRunner{}
	reconnectRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{initialRunner, reconnectRunner}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}

	initialRunner.Disconnect(errors.New("ssh keepalive timed out after 5s: context deadline exceeded"))
	time.Sleep(1200 * time.Millisecond)

	state, ok := runtimes.Get("config-1")
	if !ok {
		t.Fatal("expected runtime state to exist")
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected reconnect to recover connection, got %+v", state)
	}
	if !initialRunner.stopped {
		t.Fatal("expected disconnected runner to be stopped before reconnect")
	}
}

func TestStartStopsExistingRunnerBeforeRestart(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	existingRunner := &stubRunner{}
	runtimes.Set(RuntimeSession{ConfigurationID: "config-1", Status: StatusConnected}, existingRunner, "")
	service.factory = &stubFactory{runners: []*stubRunner{{}}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("restart tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}
	if existingRunner.stopCalls != 1 {
		t.Fatalf("expected existing runner to be stopped once, got %d", existingRunner.stopCalls)
	}
}

func TestStartReturnsMissingServerMessage(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		errorServerStore{err: sql.ErrNoRows},
		nil,
		runtimes,
	)

	_, err := service.Start(context.Background(), "config-1")
	if err == nil {
		t.Fatal("expected missing server error")
	}
	if !strings.Contains(err.Error(), "server no longer exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartUsesActionableRuntimeErrorMessage(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{startErr: errors.New("bind local port 127.0.0.1:1080: address already in use")}}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusFailed {
		t.Fatalf("expected failed state, got %s", state.Status)
	}
	if !strings.Contains(state.StatusDetail, "Another app may already be using that port") {
		t.Fatalf("unexpected status detail: %q", state.StatusDetail)
	}
}

func TestStartAllStartsEachConfigurationForServer(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{items: []configdomain.ConnectionConfiguration{
			{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080},
			{ID: "config-2", ServerID: "server-1", Label: "Docs", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9000, RemoteHost: "127.0.0.1", RemotePort: 3000},
		}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{}, {}}}

	states, err := service.StartAll(context.Background(), "server-1")
	if err != nil {
		t.Fatalf("start all tunnels: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}
	for _, state := range states {
		if state.Status != StatusConnected {
			t.Fatalf("expected connected state, got %+v", state)
		}
	}
}

func TestStartUsesAssignedSOCKSPortWhenAutoIsConfigured(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 0}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{boundPort: 43123}}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}
	if state.BoundPort != 43123 {
		t.Fatalf("expected assigned bound port, got %d", state.BoundPort)
	}
	if state.StatusDetail != "Listening on localhost:43123" {
		t.Fatalf("unexpected status detail: %q", state.StatusDetail)
	}
}

func TestHandleDisconnectDoesNotOverwriteManualStop(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080, AutoReconnectEnabled: true}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	initialRunner := &stubRunner{}
	reconnectRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{initialRunner, reconnectRunner}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}

	initialRunner.Disconnect(errors.New("ssh keepalive timed out after 5s: context deadline exceeded"))
	time.Sleep(100 * time.Millisecond)

	state, err = service.Stop(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("stop tunnel: %v", err)
	}
	if state.Status != StatusStopped {
		t.Fatalf("expected stopped state, got %s", state.Status)
	}

	time.Sleep(1200 * time.Millisecond)

	state, ok := runtimes.Get("config-1")
	if !ok {
		t.Fatal("expected runtime state to exist")
	}
	if state.Status != StatusStopped {
		t.Fatalf("expected manual stop to win, got %+v", state)
	}
	if reconnectRunner.started {
		t.Fatal("expected reconnect runner not to start after manual stop")
	}
}

func TestListHistoryReturnsEntriesForConfiguration(t *testing.T) {
	history := &stubHistoryStore{entries: []SessionHistoryEntry{
		{ID: "entry-1", ConfigurationID: "config-1", Outcome: OutcomeConnected, Message: "Listening on localhost:1080"},
		{ID: "entry-2", ConfigurationID: "config-2", Outcome: OutcomeStopped, Message: "Tunnel stopped"},
	}}
	service := NewService(nil, nil, history, NewRuntimeStore())

	entries, err := service.ListHistory(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}
	if entries[0].ID != "entry-1" {
		t.Fatalf("unexpected history entry: %+v", entries[0])
	}
}
