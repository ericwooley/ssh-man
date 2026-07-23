package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	serverdomain "ssh-man/internal/domain/server"
)

const managedSOCKSConfigurationPrefix = "server-socks:"

type Store interface {
	ListByServer(ctx context.Context, serverID string) ([]ConnectionConfiguration, error)
	ListAll(ctx context.Context) ([]ConnectionConfiguration, error)
	Get(ctx context.Context, id string) (ConnectionConfiguration, error)
	Save(ctx context.Context, item ConnectionConfiguration) error
	Delete(ctx context.Context, id string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListByServer(ctx context.Context, serverID string) ([]ConnectionConfiguration, error) {
	return s.store.ListByServer(ctx, serverID)
}

func (s *Service) ListAll(ctx context.Context) ([]ConnectionConfiguration, error) {
	return s.store.ListAll(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (ConnectionConfiguration, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Save(ctx context.Context, item ConnectionConfiguration) (ConnectionConfiguration, error) {
	if item.ID == "" {
		item.ID = newID()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	item.UpdatedAt = time.Now().UTC()
	if err := item.Validate(); err != nil {
		return ConnectionConfiguration{}, err
	}
	if err := s.validateConflict(ctx, item); err != nil {
		return ConnectionConfiguration{}, err
	}
	if err := s.store.Save(ctx, item); err != nil {
		return ConnectionConfiguration{}, fmt.Errorf("save configuration: %w", err)
	}
	return item, nil
}

func ManagedSOCKSConfigurationID(serverID string) string {
	return managedSOCKSConfigurationPrefix + strings.TrimSpace(serverID)
}

func IsManagedSOCKSConfigurationID(configurationID string) bool {
	return strings.HasPrefix(configurationID, managedSOCKSConfigurationPrefix)
}

func (s *Service) EnsureManagedSOCKSConfiguration(ctx context.Context, server serverdomain.Server) (ConnectionConfiguration, error) {
	items, err := s.store.ListByServer(ctx, server.ID)
	if err != nil {
		return ConnectionConfiguration{}, fmt.Errorf("list server configurations: %w", err)
	}
	managedID := ManagedSOCKSConfigurationID(server.ID)
	item := ConnectionConfiguration{
		ID:                   managedID,
		ServerID:             server.ID,
		Label:                availableManagedSOCKSLabel(items),
		ConnectionType:       ConnectionTypeSOCKSProxy,
		SocksPort:            server.SocksPort,
		AutoReconnectEnabled: true,
	}
	for _, candidate := range items {
		if candidate.ID != managedID {
			continue
		}
		item.Label = candidate.Label
		item.CreatedAt = candidate.CreatedAt
		break
	}
	saved, err := s.Save(ctx, item)
	if err != nil {
		return ConnectionConfiguration{}, fmt.Errorf("save automatic browser proxy: %w", err)
	}
	return saved, nil
}

func (s *Service) ValidateManagedSOCKSPort(ctx context.Context, serverID string, port int) error {
	return s.validateConflict(ctx, ConnectionConfiguration{
		ID:             ManagedSOCKSConfigurationID(serverID),
		ServerID:       serverID,
		Label:          "Browser proxy",
		ConnectionType: ConnectionTypeSOCKSProxy,
		SocksPort:      port,
	})
}

func availableManagedSOCKSLabel(items []ConnectionConfiguration) string {
	const base = "Browser proxy"
	used := make(map[string]struct{}, len(items))
	for _, item := range items {
		used[strings.ToLower(strings.TrimSpace(item.Label))] = struct{}{}
	}
	if _, exists := used[strings.ToLower(base)]; !exists {
		return base
	}
	for suffix := 2; ; suffix++ {
		label := fmt.Sprintf("%s (%d)", base, suffix)
		if _, exists := used[strings.ToLower(label)]; !exists {
			return label
		}
	}
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

func (s *Service) validateConflict(ctx context.Context, item ConnectionConfiguration) error {
	items, err := s.store.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("check port conflicts: %w", err)
	}
	for _, candidate := range items {
		if candidate.ID == item.ID {
			continue
		}
		if candidate.UsesAutomaticSOCKSPort() || item.UsesAutomaticSOCKSPort() {
			continue
		}
		if candidate.BoundPort() == item.BoundPort() {
			return fmt.Errorf("port %d already belongs to configuration %q", item.BoundPort(), candidate.Label)
		}
	}
	return nil
}

func newID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
