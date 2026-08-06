package portlink

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store interface {
	ListByServer(ctx context.Context, serverID string) ([]Link, error)
	GetByServerPort(ctx context.Context, serverID string, port int) (Link, error)
	Save(ctx context.Context, item Link) error
	Delete(ctx context.Context, id string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) ListByServer(ctx context.Context, serverID string) ([]Link, error) {
	return service.store.ListByServer(ctx, strings.TrimSpace(serverID))
}

func (service *Service) Save(ctx context.Context, link Link) (Link, error) {
	link.ServerID = strings.TrimSpace(link.ServerID)
	link.Name = strings.TrimSpace(link.Name)

	existing, err := service.store.GetByServerPort(ctx, link.ServerID, link.Port)
	switch {
	case err == nil:
		link.ID = existing.ID
		link.CreatedAt = existing.CreatedAt
	case errors.Is(err, ErrNotFound):
		link.ID = newID()
		link.CreatedAt = time.Now().UTC()
	default:
		return Link{}, fmt.Errorf("load saved port link: %w", err)
	}

	link.UpdatedAt = time.Now().UTC()
	if err := link.Validate(); err != nil {
		return Link{}, err
	}
	if err := service.store.Save(ctx, link); err != nil {
		return Link{}, fmt.Errorf("save port link: %w", err)
	}
	canonical, err := service.store.GetByServerPort(ctx, link.ServerID, link.Port)
	if err != nil {
		return Link{}, fmt.Errorf("reload saved port link: %w", err)
	}
	return canonical, nil
}

func (service *Service) Delete(ctx context.Context, id string) error {
	return service.store.Delete(ctx, strings.TrimSpace(id))
}

func newID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
