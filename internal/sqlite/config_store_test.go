package sqlite

import (
	"context"
	"testing"
	"time"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
)

func TestConfigStoreCRUD(t *testing.T) {
	db := openTestDatabase(t)
	serverStore := NewServerStore(db)
	store := NewConfigStore(db)
	ctx := context.Background()
	now := time.Now().UTC().Round(0)

	server := serverdomain.Server{
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
	if err := serverStore.Save(ctx, server); err != nil {
		t.Fatalf("save server: %v", err)
	}

	item := configdomain.ConnectionConfiguration{
		ID:                   "config-1",
		ServerID:             server.ID,
		Label:                "SOCKS",
		ConnectionType:       configdomain.ConnectionTypeSOCKSProxy,
		SocksPort:            1080,
		AutoReconnectEnabled: true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := store.Save(ctx, item); err != nil {
		t.Fatalf("save configuration: %v", err)
	}

	loaded, err := store.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get configuration: %v", err)
	}
	if loaded.Label != item.Label || loaded.SocksPort != item.SocksPort {
		t.Fatalf("unexpected loaded configuration: %+v", loaded)
	}

	item.Label = "SOCKS Admin"
	item.UpdatedAt = now.Add(time.Minute)
	if err := store.Save(ctx, item); err != nil {
		t.Fatalf("update configuration: %v", err)
	}

	byServer, err := store.ListByServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("list by server: %v", err)
	}
	if len(byServer) != 1 || byServer[0].Label != "SOCKS Admin" {
		t.Fatalf("unexpected configurations by server: %+v", byServer)
	}

	all, err := store.ListAll(ctx)
	if err != nil {
		t.Fatalf("list all configurations: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one configuration, got %+v", all)
	}

	if err := store.Delete(ctx, item.ID); err != nil {
		t.Fatalf("delete configuration: %v", err)
	}
	all, err = store.ListAll(ctx)
	if err != nil {
		t.Fatalf("list all after delete: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected no configurations after delete, got %+v", all)
	}
}
