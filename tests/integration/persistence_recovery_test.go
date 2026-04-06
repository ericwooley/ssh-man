package integration

import (
	"context"
	"testing"

	preferencesdomain "ssh-man/internal/domain/preferences"
	"ssh-man/internal/sqlite"
)

func TestPersistenceRecoveryLoadsSavedPreferencesAndData(t *testing.T) {
	db := sqliteTestDB(t)
	ctx := context.Background()
	serverStore := sqlite.NewServerStore(db)
	configStore := sqlite.NewConfigStore(db)
	prefStore := sqlite.NewPreferencesStore(db)

	server := mustSaveServer(t, ctx, serverStore)
	_ = mustSaveSOCKSConfig(t, ctx, configStore, server.ID)

	prefService := preferencesdomain.NewService(prefStore)
	pref, err := prefService.Save(ctx, preferencesdomain.UserPreference{Theme: preferencesdomain.ThemeLight, LastSelectedServerID: server.ID})
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}

	loadedPref, err := prefService.Load(ctx)
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loadedPref.Theme != preferencesdomain.ThemeLight || loadedPref.LastSelectedServerID != server.ID {
		t.Fatalf("unexpected loaded preferences: %+v", loadedPref)
	}

	if pref.Theme != loadedPref.Theme {
		t.Fatalf("expected persisted preference theme %q, got %q", pref.Theme, loadedPref.Theme)
	}

	configs, err := configStore.ListByServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("list configurations: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected persisted configuration, got %d", len(configs))
	}
}
