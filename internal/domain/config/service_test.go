package config

import (
	"context"
	"strings"
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

type stubStore struct {
	items []ConnectionConfiguration
	saved ConnectionConfiguration
}

func (s *stubStore) ListByServer(context.Context, string) ([]ConnectionConfiguration, error) {
	return s.items, nil
}

func (s *stubStore) ListAll(context.Context) ([]ConnectionConfiguration, error) {
	return s.items, nil
}

func (s *stubStore) Get(context.Context, string) (ConnectionConfiguration, error) {
	return ConnectionConfiguration{}, nil
}

func (s *stubStore) Save(_ context.Context, item ConnectionConfiguration) error {
	s.saved = item
	return nil
}

func (s *stubStore) Delete(context.Context, string) error {
	return nil
}

func TestSaveRejectsConflictingBoundPort(t *testing.T) {
	store := &stubStore{items: []ConnectionConfiguration{{ID: "existing", Label: "Existing SOCKS", ServerID: "server-1", ConnectionType: ConnectionTypeSOCKSProxy, SocksPort: 1080}}}
	service := NewService(store)

	_, err := service.Save(context.Background(), ConnectionConfiguration{ServerID: "server-1", Label: "New SOCKS", ConnectionType: ConnectionTypeSOCKSProxy, SocksPort: 1080})
	if err == nil {
		t.Fatal("expected port conflict error")
	}
	if !strings.Contains(err.Error(), "port 1080") {
		t.Fatalf("expected conflict message to mention bound port, got %q", err)
	}
}

func TestSavePersistsValidConfiguration(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	item, err := service.Save(context.Background(), ConnectionConfiguration{ServerID: "server-1", Label: "Docs tunnel", ConnectionType: ConnectionTypeLocalForward, LocalPort: 9000, RemoteHost: "127.0.0.1", RemotePort: 3000})
	if err != nil {
		t.Fatalf("save configuration: %v", err)
	}
	if item.ID == "" {
		t.Fatal("expected generated id")
	}
	if store.saved.ID == "" {
		t.Fatal("expected configuration to be stored")
	}
}

func TestSaveAllowsAutomaticSOCKSPortAlongsideFixedPorts(t *testing.T) {
	store := &stubStore{items: []ConnectionConfiguration{{ID: "existing", Label: "Existing SOCKS", ServerID: "server-1", ConnectionType: ConnectionTypeSOCKSProxy, SocksPort: 1080}}}
	service := NewService(store)

	item, err := service.Save(context.Background(), ConnectionConfiguration{ServerID: "server-1", Label: "Auto SOCKS", ConnectionType: ConnectionTypeSOCKSProxy, SocksPort: 0})
	if err != nil {
		t.Fatalf("save configuration: %v", err)
	}
	if item.SocksPort != 0 {
		t.Fatalf("expected auto socks port to be preserved, got %d", item.SocksPort)
	}
}

func TestEnsureManagedSOCKSConfigurationCreatesServerBrowserProxy(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)
	server := serverdomain.Server{
		ID:        "server-1",
		Name:      "Production",
		SocksPort: 55123,
	}

	item, err := service.EnsureManagedSOCKSConfiguration(context.Background(), server)
	if err != nil {
		t.Fatalf("ensure managed SOCKS configuration: %v", err)
	}
	if item.ID != ManagedSOCKSConfigurationID(server.ID) {
		t.Fatalf("managed configuration ID = %q", item.ID)
	}
	if item.ServerID != server.ID || item.SocksPort != server.SocksPort {
		t.Fatalf("managed configuration = %#v, want server ID and SOCKS port", item)
	}
	if item.ConnectionType != ConnectionTypeSOCKSProxy || !item.AutoReconnectEnabled {
		t.Fatalf("managed configuration = %#v, want reconnecting SOCKS proxy", item)
	}
	if store.saved.ID != item.ID {
		t.Fatalf("saved configuration = %#v, want managed configuration", store.saved)
	}
}

func TestManagedSOCKSConfigurationUsesStableIdentity(t *testing.T) {
	id := ManagedSOCKSConfigurationID("server-1")
	if id != "server-socks:server-1" {
		t.Fatalf("managed ID = %q, want stable server-derived identity", id)
	}
	if !IsManagedSOCKSConfigurationID(id) {
		t.Fatalf("expected %q to be recognized as managed", id)
	}
	if IsManagedSOCKSConfigurationID("config-1") {
		t.Fatal("ordinary configuration was recognized as managed")
	}
}
