package commandhistory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

type Store interface {
	Add(context.Context, Entry) error
	ListByServer(context.Context, string) ([]Entry, error)
	DeleteByServer(context.Context, string, string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Record(ctx context.Context, input RecordInput) (Entry, error) {
	if strings.TrimSpace(input.ServerID) == "" {
		return Entry{}, fmt.Errorf("server id is required")
	}
	if strings.TrimSpace(input.Command) == "" {
		return Entry{}, fmt.Errorf("command is required")
	}
	if input.EndedAt.Before(input.StartedAt) {
		return Entry{}, fmt.Errorf("command end time cannot be before its start time")
	}
	entry := Entry{
		ID:        newID(),
		ServerID:  input.ServerID,
		Command:   input.Command,
		Output:    input.Output,
		ExitCode:  input.ExitCode,
		StartedAt: input.StartedAt,
		EndedAt:   input.EndedAt,
		Truncated: input.Truncated,
		Error:     input.Error,
	}
	if err := s.store.Add(ctx, entry); err != nil {
		return Entry{}, fmt.Errorf("save command history: %w", err)
	}
	return entry, nil
}

func (s *Service) List(ctx context.Context, serverID string) ([]Entry, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return nil, fmt.Errorf("server id is required")
	}
	entries, err := s.store.ListByServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("load command history: %w", err)
	}
	if entries == nil {
		return []Entry{}, nil
	}
	return entries, nil
}

func (s *Service) Delete(ctx context.Context, serverID, entryID string) error {
	serverID = strings.TrimSpace(serverID)
	entryID = strings.TrimSpace(entryID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	if entryID == "" {
		return fmt.Errorf("history entry id is required")
	}
	if err := s.store.DeleteByServer(ctx, serverID, entryID); err != nil {
		return fmt.Errorf("delete command history: %w", err)
	}
	return nil
}

func newID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
