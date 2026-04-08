package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/sqlite"

	"ssh-man/internal/ssh/auth"
)

type integrationRunner struct {
	startErr  error
	stopErr   error
	stopped   bool
	boundPort int
}

func (r *integrationRunner) Start() error { return r.startErr }
func (r *integrationRunner) Stop() error {
	r.stopped = true
	return r.stopErr
}

func (r *integrationRunner) BoundPort() int {
	if r.boundPort != 0 {
		return r.boundPort
	}
	return 1080
}

type integrationFactory struct {
	runner *integrationRunner
}

func (f integrationFactory) New(serverdomain.Server, configdomain.ConnectionConfiguration, string, func(error)) sessiondomain.Runner {
	return f.runner
}

func TestSessionLifecycleStartUnlockRetryAndStop(t *testing.T) {
	db := sqliteTestDB(t)
	ctx := context.Background()
	serverStore := sqlite.NewServerStore(db)
	configStore := sqlite.NewConfigStore(db)
	historyStore := sqlite.NewSessionHistoryStore(db)
	runtimeStore := sessiondomain.NewRuntimeStore()
	service := sessiondomain.NewService(configStore, serverStore, historyStore, runtimeStore)

	server := mustSaveServer(t, ctx, serverStore)
	configuration := mustSaveSOCKSConfig(t, ctx, configStore, server.ID)

	service.SetFactory(integrationFactory{runner: &integrationRunner{startErr: auth.ErrPassphraseRequired}})
	state, err := service.Start(ctx, configuration.ID)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if state.Status != sessiondomain.StatusNeedsAttention {
		t.Fatalf("expected needs attention, got %s", state.Status)
	}

	service.SetFactory(integrationFactory{runner: &integrationRunner{}})
	state, err = service.SubmitKeyUnlock(ctx, configuration.ID, "hunter2")
	if err != nil {
		t.Fatalf("unlock session: %v", err)
	}
	if state.Status != sessiondomain.StatusConnected {
		t.Fatalf("expected connected, got %s", state.Status)
	}

	stopRunner := &integrationRunner{}
	service.SetFactory(integrationFactory{runner: stopRunner})
	runtimeStore.Set(state, stopRunner, "hunter2")
	state, err = service.Stop(ctx, configuration.ID)
	if err != nil {
		t.Fatalf("stop session: %v", err)
	}
	if state.Status != sessiondomain.StatusStopped {
		t.Fatalf("expected stopped, got %s", state.Status)
	}
	if !stopRunner.stopped {
		t.Fatal("expected runner stop to be called")
	}
}

func TestSessionLifecycleReconnectFailureBecomesFailed(t *testing.T) {
	db := sqliteTestDB(t)
	ctx := context.Background()
	serverStore := sqlite.NewServerStore(db)
	configStore := sqlite.NewConfigStore(db)
	historyStore := sqlite.NewSessionHistoryStore(db)
	runtimeStore := sessiondomain.NewRuntimeStore()
	service := sessiondomain.NewService(configStore, serverStore, historyStore, runtimeStore)

	server := mustSaveServer(t, ctx, serverStore)
	configuration := mustSaveLocalForwardConfig(t, ctx, configStore, server.ID)
	service.SetFactory(integrationFactory{runner: &integrationRunner{startErr: errors.New("bind local port 127.0.0.1:9000: address already in use")}})

	state, err := service.Start(ctx, configuration.ID)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if state.Status != sessiondomain.StatusFailed {
		t.Fatalf("expected failed, got %s", state.Status)
	}
	if state.StatusDetail == "" {
		t.Fatal("expected actionable failure detail")
	}

	time.Sleep(50 * time.Millisecond)
	runtimeState, ok := runtimeStore.Get(configuration.ID)
	if !ok || runtimeState.Status != sessiondomain.StatusFailed {
		t.Fatalf("expected failed runtime state, got %+v", runtimeState)
	}
}

func mustSaveServer(t *testing.T, ctx context.Context, store *sqlite.ServerStore) serverdomain.Server {
	t.Helper()
	service := serverdomain.NewService(store)
	server, err := service.Save(ctx, serverdomain.Server{
		Name:         "Primary",
		Host:         "example.com",
		Port:         22,
		Username:     "eric",
		AuthMode:     serverdomain.AuthModePrivateKey,
		KeyReference: "~/.ssh/id_ed25519",
	})
	if err != nil {
		t.Fatalf("save server: %v", err)
	}
	return server
}

func mustSaveSOCKSConfig(t *testing.T, ctx context.Context, store *sqlite.ConfigStore, serverID string) configdomain.ConnectionConfiguration {
	t.Helper()
	service := configdomain.NewService(store)
	configuration, err := service.Save(ctx, configdomain.ConnectionConfiguration{
		ServerID:             serverID,
		Label:                "SOCKS",
		ConnectionType:       configdomain.ConnectionTypeSOCKSProxy,
		SocksPort:            1080,
		AutoReconnectEnabled: true,
	})
	if err != nil {
		t.Fatalf("save socks config: %v", err)
	}
	return configuration
}

func mustSaveLocalForwardConfig(t *testing.T, ctx context.Context, store *sqlite.ConfigStore, serverID string) configdomain.ConnectionConfiguration {
	t.Helper()
	service := configdomain.NewService(store)
	configuration, err := service.Save(ctx, configdomain.ConnectionConfiguration{
		ServerID:             serverID,
		Label:                "Docs",
		ConnectionType:       configdomain.ConnectionTypeLocalForward,
		LocalPort:            9000,
		RemoteHost:           "127.0.0.1",
		RemotePort:           3000,
		AutoReconnectEnabled: false,
	})
	if err != nil {
		t.Fatalf("save local forward config: %v", err)
	}
	return configuration
}
