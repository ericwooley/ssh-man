package integration

import (
	"context"
	"testing"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/sqlite"
)

func TestPersistedNestedServerAndConfigurationManagement(t *testing.T) {
	db := sqliteTestDB(t)
	serverService := serverdomain.NewService(sqlite.NewServerStore(db))
	configService := configdomain.NewService(sqlite.NewConfigStore(db))
	ctx := context.Background()

	server, err := serverService.Save(ctx, serverdomain.Server{
		Name:         "Primary",
		Host:         "example.com",
		Port:         22,
		Username:     "eric",
		AuthMode:     serverdomain.AuthModePrivateKey,
		KeyReference: "~/.ssh/id_ed25519",
	})
	if err != nil {
		t.Fatalf("save server: %v", err)
	}

	firstConfig, err := configService.Save(ctx, configdomain.ConnectionConfiguration{
		ServerID:             server.ID,
		Label:                "SOCKS",
		ConnectionType:       configdomain.ConnectionTypeSOCKSProxy,
		SocksPort:            1080,
		AutoReconnectEnabled: true,
	})
	if err != nil {
		t.Fatalf("save first configuration: %v", err)
	}
	_, err = configService.Save(ctx, configdomain.ConnectionConfiguration{
		ServerID:             server.ID,
		Label:                "Docs",
		ConnectionType:       configdomain.ConnectionTypeLocalForward,
		LocalPort:            9000,
		RemoteHost:           "127.0.0.1",
		RemotePort:           3000,
		AutoReconnectEnabled: true,
	})
	if err != nil {
		t.Fatalf("save second configuration: %v", err)
	}

	servers, err := serverService.List(ctx)
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected one server, got %d", len(servers))
	}

	configs, err := configService.ListByServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("list configurations: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected two configurations, got %d", len(configs))
	}

	if err := configService.Delete(ctx, firstConfig.ID); err != nil {
		t.Fatalf("delete configuration: %v", err)
	}
	configs, err = configService.ListByServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("list configurations after delete: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected one configuration after delete, got %d", len(configs))
	}
}

func TestServerGetsPersistedAutomaticBrowserProxy(t *testing.T) {
	db := sqliteTestDB(t)
	serverStore := sqlite.NewServerStore(db)
	configStore := sqlite.NewConfigStore(db)
	configService := configdomain.NewService(configStore)
	serverService := serverdomain.NewService(serverStore)
	serverService.SetSOCKSPortAvailabilityCheck(configService.ValidateManagedSOCKSPort)
	ctx := context.Background()

	server, err := serverService.Save(ctx, serverdomain.Server{
		Name:     "Workspace",
		Host:     "workspace.example.com",
		Port:     22,
		Username: "deploy",
		AuthMode: serverdomain.AuthModeAgent,
	})
	if err != nil {
		t.Fatalf("save server: %v", err)
	}
	if server.SocksPort < 49152 || server.SocksPort > 65535 {
		t.Fatalf("automatic SOCKS port = %d, want high port", server.SocksPort)
	}
	if _, err := configService.EnsureManagedSOCKSConfiguration(ctx, server); err != nil {
		t.Fatalf("ensure managed SOCKS configuration: %v", err)
	}

	reloaded, err := serverStore.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("reload server: %v", err)
	}
	if reloaded.SocksPort != server.SocksPort {
		t.Fatalf("reloaded SOCKS port = %d, want %d", reloaded.SocksPort, server.SocksPort)
	}
	managed, err := configStore.Get(ctx, configdomain.ManagedSOCKSConfigurationID(server.ID))
	if err != nil {
		t.Fatalf("load managed SOCKS configuration: %v", err)
	}
	if managed.SocksPort != server.SocksPort || !managed.AutoReconnectEnabled {
		t.Fatalf("managed SOCKS configuration = %#v", managed)
	}
}
