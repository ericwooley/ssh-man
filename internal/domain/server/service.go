package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type Store interface {
	List(ctx context.Context) ([]Server, error)
	Get(ctx context.Context, id string) (Server, error)
	Save(ctx context.Context, item Server) error
	Delete(ctx context.Context, id string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context) ([]Server, error) {
	return s.store.List(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (Server, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Save(ctx context.Context, item Server) (Server, error) {
	if item.ID == "" {
		item.ID = newID()
		item.CreatedAt = time.Now().UTC()
	}
	item.UpdatedAt = time.Now().UTC()
	if err := item.Validate(); err != nil {
		return Server{}, err
	}
	if err := s.store.Save(ctx, item); err != nil {
		return Server{}, fmt.Errorf("save server: %w", err)
	}
	return item, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

func newID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
