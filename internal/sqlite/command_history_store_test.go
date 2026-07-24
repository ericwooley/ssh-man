package sqlite

import (
	"context"
	"testing"
	"time"

	"ssh-man/internal/domain/commandhistory"
	serverdomain "ssh-man/internal/domain/server"
)

func TestCommandHistoryStorePersistsListsAndDeletesByServer(t *testing.T) {
	db := openTestDatabase(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	serverStore := NewServerStore(db)
	for _, serverID := range []string{"server-1", "server-2"} {
		if err := serverStore.Save(ctx, serverdomain.Server{
			ID: serverID, Name: serverID, Host: "example.com", Port: 22,
			Username: "deploy", AuthMode: serverdomain.AuthModeAgent,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("save %s: %v", serverID, err)
		}
	}
	store := NewCommandHistoryStore(db)
	entries := []commandhistory.Entry{
		{ID: "older", ServerID: "server-1", Command: "pwd", Output: "/srv\n", ExitCode: 0, StartedAt: now, EndedAt: now.Add(time.Second)},
		{ID: "newer", ServerID: "server-1", Command: "false", ExitCode: 1, StartedAt: now.Add(time.Minute), EndedAt: now.Add(time.Minute + time.Second), Error: "exit status 1"},
		{ID: "other", ServerID: "server-2", Command: "whoami", Output: "deploy\n", ExitCode: 0, StartedAt: now, EndedAt: now, Truncated: true},
	}
	for _, entry := range entries {
		if err := store.Add(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}

	got, err := store.ListByServer(ctx, "server-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "newer" || got[1].ID != "older" || got[0].Error != "exit status 1" {
		t.Fatalf("listed entries = %#v", got)
	}

	if err := store.DeleteByServer(ctx, "server-2", "newer"); err == nil {
		t.Fatal("expected cross-server deletion to fail")
	}
	if err := store.DeleteByServer(ctx, "server-1", "newer"); err != nil {
		t.Fatal(err)
	}
	got, err = store.ListByServer(ctx, "server-1")
	if err != nil || len(got) != 1 || got[0].ID != "older" {
		t.Fatalf("entries after delete = %#v, %v", got, err)
	}
}
