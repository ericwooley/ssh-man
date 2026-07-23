package server

import (
	"context"
	"testing"
)

type serviceTestStore struct {
	items []Server
	saved []Server
}

func (s *serviceTestStore) List(context.Context) ([]Server, error) {
	return append([]Server(nil), s.items...), nil
}

func (s *serviceTestStore) Get(_ context.Context, id string) (Server, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
	return Server{}, nil
}

func (s *serviceTestStore) Save(_ context.Context, item Server) error {
	s.saved = append(s.saved, item)
	for index := range s.items {
		if s.items[index].ID == item.ID {
			s.items[index] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}

func (s *serviceTestStore) Delete(context.Context, string) error {
	return nil
}

func validServer() Server {
	return Server{
		Name:      "Production",
		Host:      "example.com",
		Port:      22,
		Username:  "deploy",
		AuthMode:  AuthModeAgent,
		SocksPort: 0,
	}
}

func TestSaveAssignsAndPersistsAnAvailableHighSOCKSPort(t *testing.T) {
	store := &serviceTestStore{
		items: []Server{{ID: "existing", SocksPort: 55000}},
	}
	service := NewService(store)
	var reserved map[int]struct{}
	service.portAllocator = func(ports map[int]struct{}) (int, error) {
		reserved = ports
		return 55123, nil
	}

	saved, err := service.Save(context.Background(), validServer())
	if err != nil {
		t.Fatalf("save server: %v", err)
	}
	if saved.SocksPort != 55123 {
		t.Fatalf("SocksPort = %d, want 55123", saved.SocksPort)
	}
	if _, ok := reserved[55000]; !ok {
		t.Fatalf("allocator reservations = %#v, want existing server port", reserved)
	}
	if len(store.saved) != 1 || store.saved[0].SocksPort != 55123 {
		t.Fatalf("stored servers = %#v, want assigned port persisted", store.saved)
	}
}

func TestSavePreservesAnEditableSOCKSPort(t *testing.T) {
	store := &serviceTestStore{}
	service := NewService(store)
	item := validServer()
	item.SocksPort = 61234

	saved, err := service.Save(context.Background(), item)
	if err != nil {
		t.Fatalf("save server: %v", err)
	}
	if saved.SocksPort != 61234 {
		t.Fatalf("SocksPort = %d, want 61234", saved.SocksPort)
	}
}

func TestEnsureSOCKSPortsBackfillsExistingServers(t *testing.T) {
	item := validServer()
	item.ID = "legacy-server"
	store := &serviceTestStore{items: []Server{item}}
	service := NewService(store)
	service.portAllocator = func(map[int]struct{}) (int, error) {
		return 55234, nil
	}

	if err := service.EnsureSOCKSPorts(context.Background()); err != nil {
		t.Fatalf("ensure SOCKS ports: %v", err)
	}
	if len(store.saved) != 1 || store.saved[0].SocksPort != 55234 {
		t.Fatalf("stored servers = %#v, want legacy server backfilled", store.saved)
	}
}
