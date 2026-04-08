package config

import (
	"context"
	"strings"
	"testing"
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
