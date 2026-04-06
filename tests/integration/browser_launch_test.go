package integration

import (
	"context"
	"testing"

	configdomain "ssh-man/internal/domain/config"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
	"ssh-man/internal/sqlite"
)

func TestBrowserLaunchValidationRequiresRunningSOCKSSession(t *testing.T) {
	db := sqliteTestDB(t)
	ctx := context.Background()
	serverStore := sqlite.NewServerStore(db)
	configStore := sqlite.NewConfigStore(db)
	runtimeStore := sessiondomain.NewRuntimeStore()

	server := mustSaveServer(t, ctx, serverStore)
	configuration := mustSaveSOCKSConfig(t, ctx, configStore, server.ID)
	service := browser.NewService(configStore, runtimeStore)

	err := service.LaunchThroughSOCKS(ctx, configuration.ID, "google-chrome")
	if err == nil {
		t.Fatal("expected launch validation error")
	}
}

func TestBrowserLaunchValidationRejectsLocalForwardConfiguration(t *testing.T) {
	db := sqliteTestDB(t)
	ctx := context.Background()
	serverStore := sqlite.NewServerStore(db)
	configStore := sqlite.NewConfigStore(db)
	runtimeStore := sessiondomain.NewRuntimeStore()

	server := mustSaveServer(t, ctx, serverStore)
	configuration := mustSaveLocalForwardConfig(t, ctx, configStore, server.ID)
	runtimeStore.Set(sessiondomain.RuntimeSession{ConfigurationID: configuration.ID, Status: sessiondomain.StatusConnected}, nil, "")

	service := browser.NewService(configStore, runtimeStore)
	err := service.LaunchThroughSOCKS(ctx, configuration.ID, "google-chrome")
	if err == nil {
		t.Fatal("expected local forward validation error")
	}
	if configuration.ConnectionType != configdomain.ConnectionTypeLocalForward {
		t.Fatalf("expected local forward configuration, got %s", configuration.ConnectionType)
	}
}
