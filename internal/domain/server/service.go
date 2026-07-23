package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"time"
)

const (
	automaticSOCKSPortMinimum = 49152
	automaticSOCKSPortMaximum = 65535
)

type Store interface {
	List(ctx context.Context) ([]Server, error)
	Get(ctx context.Context, id string) (Server, error)
	Save(ctx context.Context, item Server) error
	Delete(ctx context.Context, id string) error
}

type Service struct {
	store                 Store
	portAllocator         func(map[int]struct{}) (int, error)
	socksPortAvailability func(context.Context, string, int) error
}

func NewService(store Store) *Service {
	return &Service{
		store:         store,
		portAllocator: findAvailableHighPort,
	}
}

func (s *Service) SetSOCKSPortAvailabilityCheck(check func(context.Context, string, int) error) {
	s.socksPortAvailability = check
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
	if item.SocksPort == 0 {
		port, err := s.assignSOCKSPort(ctx, item.ID)
		if err != nil {
			return Server{}, err
		}
		item.SocksPort = port
	} else if err := s.validateSOCKSPortAvailable(ctx, item.ID, item.SocksPort); err != nil {
		return Server{}, err
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

func (s *Service) EnsureSOCKSPorts(ctx context.Context) error {
	items, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list servers for SOCKS port assignment: %w", err)
	}
	for _, item := range items {
		if item.SocksPort != 0 {
			continue
		}
		if _, err := s.Save(ctx, item); err != nil {
			return fmt.Errorf("assign SOCKS port for %q: %w", item.Name, err)
		}
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

func (s *Service) assignSOCKSPort(ctx context.Context, serverID string) (int, error) {
	items, err := s.store.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list reserved server SOCKS ports: %w", err)
	}
	reserved := make(map[int]struct{}, len(items))
	for _, item := range items {
		if item.ID != serverID && item.SocksPort > 0 {
			reserved[item.SocksPort] = struct{}{}
		}
	}

	for attempts := 0; attempts < 64; attempts++ {
		port, err := s.portAllocator(reserved)
		if err != nil {
			return 0, fmt.Errorf("find an available high SOCKS port: %w", err)
		}
		if err := s.validateAdditionalSOCKSPortAvailability(ctx, serverID, port); err == nil {
			return port, nil
		}
		reserved[port] = struct{}{}
	}
	return 0, fmt.Errorf("find an available high SOCKS port: no unreserved port was accepted")
}

func (s *Service) validateSOCKSPortAvailable(ctx context.Context, serverID string, port int) error {
	items, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list reserved server SOCKS ports: %w", err)
	}
	for _, item := range items {
		if item.ID != serverID && item.SocksPort == port {
			return fmt.Errorf("server SOCKS port %d is already assigned to %q", port, item.Name)
		}
	}
	return s.validateAdditionalSOCKSPortAvailability(ctx, serverID, port)
}

func (s *Service) validateAdditionalSOCKSPortAvailability(ctx context.Context, serverID string, port int) error {
	if s.socksPortAvailability == nil {
		return nil
	}
	return s.socksPortAvailability(ctx, serverID, port)
}

func findAvailableHighPort(reserved map[int]struct{}) (int, error) {
	portCount := automaticSOCKSPortMaximum - automaticSOCKSPortMinimum + 1
	offset, err := rand.Int(rand.Reader, big.NewInt(int64(portCount)))
	if err != nil {
		return 0, fmt.Errorf("choose port search start: %w", err)
	}
	for checked := 0; checked < portCount; checked++ {
		port := automaticSOCKSPortMinimum + (int(offset.Int64())+checked)%portCount
		if _, exists := reserved[port]; exists {
			continue
		}
		listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		if err := listener.Close(); err != nil {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("no open port between %d and %d", automaticSOCKSPortMinimum, automaticSOCKSPortMaximum)
}

func newID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
