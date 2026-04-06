package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

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

func (s *Service) Get(ctx context.Context, id string) (ConnectionConfiguration, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Save(ctx context.Context, item ConnectionConfiguration) (ConnectionConfiguration, error) {
	if item.ID == "" {
		item.ID = newID()
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
