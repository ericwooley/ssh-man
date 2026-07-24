package session

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
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

func (s stubConfigStore) Get(_ context.Context, id string) (configdomain.ConnectionConfiguration, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
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

func (s stubConfigStore) ListAll(context.Context) ([]configdomain.ConnectionConfiguration, error) {
	if s.items != nil {
		return s.items, nil
	}
	if s.item.ID == "" {
		return nil, nil
	}
	return []configdomain.ConnectionConfiguration{s.item}, nil
}

type selectiveErrorConfigStore struct {
	stubConfigStore
	errorID string
}

func (s selectiveErrorConfigStore) Get(ctx context.Context, id string) (configdomain.ConnectionConfiguration, error) {
	if id == s.errorID {
		return configdomain.ConnectionConfiguration{}, errors.New("configuration read failed")
	}
	return s.stubConfigStore.Get(ctx, id)
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
	startErr     error
	stopErr      error
	started      bool
	stopped      bool
	stopCalls    int
	onDisc       func(error)
	boundPort    int
	startEntered chan struct{}
	startRelease <-chan struct{}
}

func (s *stubRunner) Start() error {
	if s.startEntered != nil {
		close(s.startEntered)
	}
	if s.startRelease != nil {
		<-s.startRelease
	}
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
	mu      sync.Mutex
	runners []*stubRunner
}

func (s *stubFactory) New(_ serverdomain.Server, _ configdomain.ConnectionConfiguration, _ string, onDisconnect func(error)) Runner {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func TestStartRecordsRejectedSSHAuthenticationAsFailedAuth(t *testing.T) {
	runtimes := NewRuntimeStore()
	history := &stubHistoryStore{}
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "MacBook", Host: "mac.example.test", Port: 22, Username: "tester", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "/Users/tester/.ssh/id_ed25519"}},
		history,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{startErr: errors.New("connect to ssh server: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain")}}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusFailed {
		t.Fatalf("expected failed state, got %s", state.Status)
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

func TestHandleDisconnectKeepsRetryingUntilRecovery(t *testing.T) {
	runtimes := NewRuntimeStore()
	history := &stubHistoryStore{}
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080, AutoReconnectEnabled: true}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		history,
		runtimes,
	)
	service.reconnectDelay = func(int) time.Duration { return 10 * time.Millisecond }
	service.reconnectTimeout = 100 * time.Millisecond

	initialRunner := &stubRunner{}
	failingReconnectOne := &stubRunner{startErr: errors.New("connect to ssh server: dial tcp example.com:22: network is unreachable")}
	failingReconnectTwo := &stubRunner{startErr: errors.New("connect to ssh server: dial tcp example.com:22: network is unreachable")}
	recoveredRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{initialRunner, failingReconnectOne, failingReconnectTwo, recoveredRunner}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}

	initialRunner.Disconnect(errors.New("ssh keepalive timed out after 5s: context deadline exceeded"))

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		state, ok := runtimes.Get("config-1")
		if ok && state.Status == StatusConnected {
			if !recoveredRunner.started {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if state.ReconnectAttemptCount != 0 {
				t.Fatalf("expected connected state to clear reconnect attempts, got %+v", state)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	state, _ = runtimes.Get("config-1")
	t.Fatalf("expected reconnect loop to recover eventually, got %+v", state)
}

func TestStartLeavesAnActiveTunnelRunning(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	existingRunner := &stubRunner{}
	runtimes.Set(RuntimeSession{ConfigurationID: "config-1", Status: StatusConnected}, existingRunner, "")
	replacementRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{replacementRunner}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start active tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}
	if existingRunner.stopCalls != 0 {
		t.Fatalf("expected existing runner to stay connected, got %d stop calls", existingRunner.stopCalls)
	}
	if replacementRunner.started {
		t.Fatal("expected start to be idempotent while the tunnel is active")
	}
}

func TestConcurrentStartsWaitForTheSameTunnelOperation(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	entered := make(chan struct{})
	release := make(chan struct{})
	firstRunner := &stubRunner{startEntered: entered, startRelease: release}
	replacementRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{firstRunner, replacementRunner}}

	firstDone := make(chan RuntimeSession, 1)
	secondDone := make(chan RuntimeSession, 1)
	go func() {
		state, _ := service.Start(context.Background(), "config-1")
		firstDone <- state
	}()
	<-entered
	go func() {
		state, _ := service.Start(context.Background(), "config-1")
		secondDone <- state
	}()

	select {
	case state := <-secondDone:
		t.Fatalf("second start returned before the first completed: %+v", state)
	case <-time.After(25 * time.Millisecond):
	}

	close(release)
	firstState := <-firstDone
	secondState := <-secondDone
	if firstState.Status != StatusConnected || secondState.Status != StatusConnected {
		t.Fatalf("expected both callers to observe the connected state, got %+v and %+v", firstState, secondState)
	}
	if replacementRunner.started || firstRunner.stopCalls != 0 {
		t.Fatal("expected concurrent start to reuse the completed tunnel operation")
	}
}

func TestRetryStopsExistingRunnerBeforeRestart(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	existingRunner := &stubRunner{}
	runtimes.Set(RuntimeSession{ConfigurationID: "config-1", Status: StatusConnected}, existingRunner, "passphrase")
	replacementRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{replacementRunner}}

	state, err := service.Retry(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("retry tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}
	if existingRunner.stopCalls != 1 {
		t.Fatalf("expected existing runner to be stopped once, got %d", existingRunner.stopCalls)
	}
	if !replacementRunner.started {
		t.Fatal("expected retry to start a replacement runner")
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

func TestStartAllLeavesManagedBrowserProxyForTheWorkspaceAction(t *testing.T) {
	managedID := configdomain.ManagedSOCKSConfigurationID("server-1")
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{items: []configdomain.ConnectionConfiguration{
			{ID: managedID, ServerID: "server-1", Label: "Browser proxy", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 55123},
			{ID: "config-1", ServerID: "server-1", Label: "Docs", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9000, RemoteHost: "127.0.0.1", RemotePort: 3000},
		}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, SocksPort: 55123, Username: "eric", AuthMode: serverdomain.AuthModeAgent}},
		nil,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{}}}

	states, err := service.StartAll(context.Background(), "server-1")
	if err != nil {
		t.Fatalf("start all tunnels: %v", err)
	}
	if len(states) != 1 || states[0].ConfigurationID != "config-1" {
		t.Fatalf("started states = %#v, want only user-created tunnel", states)
	}
	if _, ok := runtimes.Get(managedID); ok {
		t.Fatal("managed browser proxy started during user tunnel bulk start")
	}
}

func TestConfigurationsStartingOnLaunchSelectsOnlyEnabledConfigurations(t *testing.T) {
	configurations := []configdomain.ConnectionConfiguration{
		{ID: "config-1", StartOnLaunch: true},
		{ID: "config-2", StartOnLaunch: false},
		{ID: "config-3", StartOnLaunch: true},
	}

	selected := configurationsStartingOnLaunch(configurations)
	if len(selected) != 2 || selected[0].ID != "config-1" || selected[1].ID != "config-3" {
		t.Fatalf("configurationsStartingOnLaunch() = %+v, want config-1 and config-3", selected)
	}
}

func TestStartOnLaunchStartsOnlyEnabledConfigurations(t *testing.T) {
	configurations := []configdomain.ConnectionConfiguration{
		{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080, StartOnLaunch: true},
		{ID: "config-2", ServerID: "server-1", Label: "Manual", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9000, RemoteHost: "127.0.0.1", RemotePort: 3000},
		{ID: "config-3", ServerID: "server-1", Label: "Docs", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9001, RemoteHost: "127.0.0.1", RemotePort: 3001, StartOnLaunch: true},
	}
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{items: configurations},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModeAgent}},
		nil,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{}, {}}}

	if err := service.StartOnLaunch(context.Background()); err != nil {
		t.Fatalf("start on launch: %v", err)
	}
	if state, ok := runtimes.Get("config-1"); !ok || state.Status != StatusConnected {
		t.Fatalf("expected config-1 to connect, got %+v, exists %v", state, ok)
	}
	if _, ok := runtimes.Get("config-2"); ok {
		t.Fatal("expected config-2 to remain inactive")
	}
	if state, ok := runtimes.Get("config-3"); !ok || state.Status != StatusConnected {
		t.Fatalf("expected config-3 to connect, got %+v, exists %v", state, ok)
	}
}

func TestStartManagedSOCKSProxiesStartsOnlyAutomaticBrowserProxies(t *testing.T) {
	managedOne := configdomain.ManagedSOCKSConfigurationID("server-1")
	managedTwo := configdomain.ManagedSOCKSConfigurationID("server-2")
	configurations := []configdomain.ConnectionConfiguration{
		{ID: managedOne, ServerID: "server-1", Label: "Browser proxy", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 41001},
		{ID: "user-socks", ServerID: "server-1", Label: "Manual SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 42001},
		{ID: managedTwo, ServerID: "server-2", Label: "Browser proxy", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 41002},
	}
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{items: configurations},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModeAgent}},
		nil,
		runtimes,
	)
	service.factory = &stubFactory{runners: []*stubRunner{{}, {}}}

	states, err := service.StartManagedSOCKSProxies(context.Background())
	if err != nil {
		t.Fatalf("start managed SOCKS proxies: %v", err)
	}
	if len(states) != 2 || states[0].ConfigurationID != managedOne || states[1].ConfigurationID != managedTwo {
		t.Fatalf("started states = %#v", states)
	}
	if _, ok := runtimes.Get("user-socks"); ok {
		t.Fatal("user-created SOCKS tunnel was started")
	}
}

func TestCanBulkStartOnlyAllowsInactiveTunnels(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		exists bool
		want   bool
	}{
		{name: "missing runtime", exists: false, want: true},
		{name: "stopped", status: StatusStopped, exists: true, want: true},
		{name: "failed", status: StatusFailed, exists: true, want: true},
		{name: "starting", status: StatusStarting, exists: true, want: false},
		{name: "connected", status: StatusConnected, exists: true, want: false},
		{name: "reconnecting", status: StatusReconnecting, exists: true, want: false},
		{name: "needs attention", status: StatusNeedsAttention, exists: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canBulkStart(RuntimeSession{Status: tt.status}, tt.exists); got != tt.want {
				t.Fatalf("canBulkStart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStartAllLeavesActiveTunnelsRunning(t *testing.T) {
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
	existingRunner := &stubRunner{}
	runtimes.Set(RuntimeSession{ConfigurationID: "config-1", Status: StatusConnected}, existingRunner, "")
	newRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{newRunner}}

	states, err := service.StartAll(context.Background(), "server-1")
	if err != nil {
		t.Fatalf("start inactive tunnels: %v", err)
	}
	if len(states) != 1 || states[0].ConfigurationID != "config-2" {
		t.Fatalf("expected only config-2 to start, got %+v", states)
	}
	if existingRunner.stopCalls != 0 {
		t.Fatalf("expected connected tunnel to keep running, got %d stop calls", existingRunner.stopCalls)
	}
	if !newRunner.started {
		t.Fatal("expected inactive tunnel to start")
	}
}

func TestStartAllContinuesAfterOneConfigurationFails(t *testing.T) {
	configurations := []configdomain.ConnectionConfiguration{
		{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080},
		{ID: "config-2", ServerID: "server-1", Label: "Broken", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9000, RemoteHost: "127.0.0.1", RemotePort: 3000},
		{ID: "config-3", ServerID: "server-1", Label: "Docs", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9001, RemoteHost: "127.0.0.1", RemotePort: 3001},
	}
	runtimes := NewRuntimeStore()
	service := NewService(
		selectiveErrorConfigStore{
			stubConfigStore: stubConfigStore{items: configurations},
			errorID:         "config-2",
		},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	firstRunner := &stubRunner{}
	thirdRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{firstRunner, thirdRunner}}

	states, err := service.StartAll(context.Background(), "server-1")
	if err == nil || !strings.Contains(err.Error(), "Broken: load configuration") {
		t.Fatalf("expected labeled partial failure, got %v", err)
	}
	if len(states) != 2 || states[0].ConfigurationID != "config-1" || states[1].ConfigurationID != "config-3" {
		t.Fatalf("expected the other tunnels to start, got %+v", states)
	}
	if !firstRunner.started || !thirdRunner.started {
		t.Fatal("expected start-all to continue after the failed configuration")
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
	service.reconnectDelay = func(int) time.Duration { return 200 * time.Millisecond }
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

func TestManualStopWinsWhileReconnectStartIsInFlight(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080, AutoReconnectEnabled: true}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	service.reconnectDelay = func(int) time.Duration { return 0 }
	service.reconnectTimeout = time.Second

	reconnectEntered := make(chan struct{})
	reconnectRelease := make(chan struct{})
	var releaseOnce sync.Once
	releaseReconnect := func() {
		releaseOnce.Do(func() { close(reconnectRelease) })
	}
	defer releaseReconnect()

	initialRunner := &stubRunner{}
	reconnectRunner := &stubRunner{startEntered: reconnectEntered, startRelease: reconnectRelease}
	service.factory = &stubFactory{runners: []*stubRunner{initialRunner, reconnectRunner}}

	state, err := service.Start(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("start tunnel: %v", err)
	}
	if state.Status != StatusConnected {
		t.Fatalf("expected connected state, got %s", state.Status)
	}

	initialRunner.Disconnect(errors.New("network dropped"))
	select {
	case <-reconnectEntered:
	case <-time.After(time.Second):
		t.Fatal("reconnect did not enter runner start")
	}

	operationLock := service.operationLock("config-1")
	if operationLock.TryLock() {
		operationLock.Unlock()
		t.Fatal("reconnect start did not hold the per-configuration operation lock")
	}

	type stopResult struct {
		state RuntimeSession
		err   error
	}
	stopInvoked := make(chan struct{})
	stopDone := make(chan stopResult, 1)
	go func() {
		close(stopInvoked)
		stopped, stopErr := service.Stop(context.Background(), "config-1")
		stopDone <- stopResult{state: stopped, err: stopErr}
	}()
	<-stopInvoked
	releaseReconnect()

	var result stopResult
	select {
	case result = <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("manual stop did not complete after reconnect start was released")
	}
	if result.err != nil {
		t.Fatalf("stop tunnel: %v", result.err)
	}
	if result.state.Status != StatusStopped {
		t.Fatalf("expected stop to return stopped state, got %+v", result.state)
	}

	state, ok := runtimes.Get("config-1")
	if !ok || state.Status != StatusStopped {
		t.Fatalf("expected manual stop to remain authoritative, got %+v", state)
	}
	storedRunner, _, _ := runtimes.Runner("config-1")
	if storedRunner != nil {
		t.Fatal("expected stopped runtime not to retain the reconnect runner")
	}
	if !reconnectRunner.stopped || reconnectRunner.stopCalls != 1 {
		t.Fatalf("expected manual stop to stop the reconnect runner once, stopped=%v calls=%d", reconnectRunner.stopped, reconnectRunner.stopCalls)
	}
}

func TestReconnectRevalidatesTokenAfterManualStop(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080, AutoReconnectEnabled: true}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/id_ed25519"}},
		nil,
		runtimes,
	)
	reconnectRunner := &stubRunner{}
	service.factory = &stubFactory{runners: []*stubRunner{reconnectRunner}}
	runtimes.SetWithToken(RuntimeSession{ConfigurationID: "config-1", Status: StatusReconnecting}, nil, "", "reconnect-token")

	stopped, err := service.Stop(context.Background(), "config-1")
	if err != nil {
		t.Fatalf("stop tunnel: %v", err)
	}
	if stopped.Status != StatusStopped {
		t.Fatalf("expected stopped state, got %+v", stopped)
	}

	_, restarted, err := service.restartWithToken(context.Background(), "config-1", "", "reconnect-token")
	if err != nil {
		t.Fatalf("restart stale reconnect generation: %v", err)
	}
	if restarted {
		t.Fatal("expected stale reconnect generation to be rejected")
	}
	if reconnectRunner.started {
		t.Fatal("expected stale reconnect generation not to create a runner")
	}

	state, ok := runtimes.Get("config-1")
	if !ok || state.Status != StatusStopped {
		t.Fatalf("expected manual stop to remain authoritative, got %+v", state)
	}
}

func TestCanceledStartStopsRunnerAndDoesNotPublishConnected(t *testing.T) {
	runtimes := NewRuntimeStore()
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModeAgent}},
		nil,
		runtimes,
	)
	entered := make(chan struct{})
	release := make(chan struct{})
	runner := &stubRunner{startEntered: entered, startRelease: release}
	service.factory = &stubFactory{runners: []*stubRunner{runner}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.Start(ctx, "config-1")
		done <- err
	}()
	<-entered
	cancel()
	close(release)

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() error = %v, want context canceled", err)
	}
	if !runner.stopped {
		t.Fatal("runner was not stopped after request cancellation")
	}
	state, ok := runtimes.Get("config-1")
	if !ok || state.Status != StatusStopped {
		t.Fatalf("runtime state = %+v, %v; want stopped", state, ok)
	}
}

func TestCanceledExclusiveMutationNeverRunsAfterWaitingForStart(t *testing.T) {
	service := NewService(
		stubConfigStore{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", Label: "SOCKS", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubServerStore{item: serverdomain.Server{ID: "server-1", Name: "Host", Host: "example.com", Port: 22, Username: "eric", AuthMode: serverdomain.AuthModeAgent}},
		nil,
		NewRuntimeStore(),
	)
	entered := make(chan struct{})
	release := make(chan struct{})
	service.factory = &stubFactory{runners: []*stubRunner{{startEntered: entered, startRelease: release}}}

	startDone := make(chan error, 1)
	go func() {
		_, err := service.Start(context.Background(), "config-1")
		startDone <- err
	}()
	<-entered

	ctx, cancel := context.WithCancel(context.Background())
	mutationCalled := false
	mutationDone := make(chan error, 1)
	go func() {
		mutationDone <- service.WithExclusiveMutation(ctx, func(context.Context) error {
			mutationCalled = true
			return nil
		})
	}()
	cancel()
	close(release)

	if err := <-startDone; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := <-mutationDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("WithExclusiveMutation() error = %v, want context canceled", err)
	}
	if mutationCalled {
		t.Fatal("mutation callback ran after its request was canceled")
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
