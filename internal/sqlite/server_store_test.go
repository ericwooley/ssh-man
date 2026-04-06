package sqlite

import (
	"context"
	"testing"
	"time"

	serverdomain "ssh-man/internal/domain/server"
)

func TestServerStoreCRUD(t *testing.T) {
	db := openTestDatabase(t)
	store := NewServerStore(db)
	ctx := context.Background()
	now := time.Now().UTC().Round(0)

	item := serverdomain.Server{
		ID:           "server-1",
		Name:         "Primary",
		Host:         "example.com",
		Port:         22,
		Username:     "eric",
		AuthMode:     serverdomain.AuthModePrivateKey,
		KeyReference: "~/.ssh/id_ed25519",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := store.Save(ctx, item); err != nil {
		t.Fatalf("save server: %v", err)
	}

	loaded, err := store.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if loaded.Name != item.Name || loaded.Host != item.Host {
		t.Fatalf("unexpected loaded server: %+v", loaded)
	}

	item.Name = "Renamed"
	item.UpdatedAt = now.Add(time.Minute)
	if err := store.Save(ctx, item); err != nil {
		t.Fatalf("update server: %v", err)
	}

	items, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Renamed" {
		t.Fatalf("unexpected listed servers: %+v", items)
	}

	if err := store.Delete(ctx, item.ID); err != nil {
		t.Fatalf("delete server: %v", err)
	}
	items, err = store.List(ctx)
	if err != nil {
		t.Fatalf("list servers after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no servers after delete, got %+v", items)
	}
}
