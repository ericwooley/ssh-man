package commandhistory

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	entries []Entry
	err     error
}

func (s *memoryStore) Add(_ context.Context, entry Entry) error {
	if s.err != nil {
		return s.err
	}
	s.entries = append(s.entries, entry)
	return nil
}

func (s *memoryStore) ListByServer(_ context.Context, serverID string) ([]Entry, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := []Entry{}
	for _, entry := range s.entries {
		if entry.ServerID == serverID {
			result = append(result, entry)
		}
	}
	return result, nil
}

func (s *memoryStore) DeleteByServer(_ context.Context, serverID, entryID string) error {
	if s.err != nil {
		return s.err
	}
	next := s.entries[:0]
	for _, entry := range s.entries {
		if entry.ServerID != serverID || entry.ID != entryID {
			next = append(next, entry)
		}
	}
	s.entries = next
	return nil
}

func TestRecordPreservesCommandAndOutput(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	startedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)

	entry, err := service.Record(context.Background(), RecordInput{
		ServerID:  "server-1",
		Command:   "printf 'hello\\n'",
		Output:    "hello\n",
		ExitCode:  0,
		StartedAt: startedAt,
		EndedAt:   startedAt.Add(time.Second),
	})

	if err != nil {
		t.Fatal(err)
	}
	if entry.ID == "" || entry.Command != "printf 'hello\\n'" || entry.Output != "hello\n" {
		t.Fatalf("recorded entry = %#v", entry)
	}
	if len(store.entries) != 1 || store.entries[0] != entry {
		t.Fatalf("stored entries = %#v", store.entries)
	}
}

func TestDeleteIsScopedToServer(t *testing.T) {
	store := &memoryStore{entries: []Entry{
		{ID: "shared-id", ServerID: "server-1"},
		{ID: "shared-id", ServerID: "server-2"},
	}}
	service := NewService(store)

	if err := service.Delete(context.Background(), "server-1", "shared-id"); err != nil {
		t.Fatal(err)
	}

	if len(store.entries) != 1 || store.entries[0].ServerID != "server-2" {
		t.Fatalf("remaining entries = %#v", store.entries)
	}
}

func TestRecordRejectsInvalidInputAndWrapsStorageErrors(t *testing.T) {
	service := NewService(&memoryStore{})
	if _, err := service.Record(context.Background(), RecordInput{}); err == nil {
		t.Fatal("expected invalid record error")
	}

	storeErr := errors.New("database unavailable")
	service = NewService(&memoryStore{err: storeErr})
	now := time.Now()
	_, err := service.Record(context.Background(), RecordInput{
		ServerID: "server-1", Command: "pwd", StartedAt: now, EndedAt: now,
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("Record() error = %v, want wrapped storage error", err)
	}
}
