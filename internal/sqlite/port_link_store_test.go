package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	portlinkdomain "ssh-man/internal/domain/portlink"
	serverdomain "ssh-man/internal/domain/server"
)

func TestPortLinkStoreCRUDAndServerCascade(t *testing.T) {
	db := openTestDatabase(t)
	serverStore := NewServerStore(db)
	store := NewPortLinkStore(db)
	ctx := context.Background()
	now := time.Now().UTC().Round(0)

	server := serverdomain.Server{
		ID:        "server-1",
		Name:      "Primary",
		Host:      "example.com",
		Port:      22,
		Username:  "eric",
		AuthMode:  serverdomain.AuthModeAgent,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := serverStore.Save(ctx, server); err != nil {
		t.Fatalf("save server: %v", err)
	}

	item := portlinkdomain.Link{
		ID:             "link-1",
		ServerID:       server.ID,
		Port:           3000,
		Name:           "Admin",
		Scheme:         portlinkdomain.SchemeHTTP,
		FaviconDataURL: "data:image/png;base64,aWNvbg==",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := store.Save(ctx, item); err != nil {
		t.Fatalf("save link: %v", err)
	}

	loaded, err := store.GetByServerPort(ctx, server.ID, item.Port)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != item {
		t.Fatalf("loaded link = %#v, want %#v", loaded, item)
	}

	items, err := store.ListByServer(ctx, server.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("links = %#v", items)
	}

	if err := serverStore.Delete(ctx, server.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetByServerPort(ctx, server.ID, item.Port); !errors.Is(err, portlinkdomain.ErrNotFound) && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get cascaded link error = %v", err)
	}
}
