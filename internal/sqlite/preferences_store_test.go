package sqlite

import (
	"context"
	"reflect"
	"strings"
	"testing"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func TestPreferencesStorePersistsBrowserSwitcherShortcuts(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.Theme = preferencesdomain.ThemeLight
	pref.BrowserSwitcherShortcut = "Ctrl+Alt+B"
	pref.BrowserSwitcherBackwardShortcut = "Ctrl+Alt+Shift+B"

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loaded.BrowserSwitcherShortcut != "Ctrl+Alt+B" {
		t.Fatalf("shortcut = %q, want %q", loaded.BrowserSwitcherShortcut, "Ctrl+Alt+B")
	}
	if loaded.BrowserSwitcherBackwardShortcut != "Ctrl+Alt+Shift+B" {
		t.Fatalf("backward shortcut = %q, want %q", loaded.BrowserSwitcherBackwardShortcut, "Ctrl+Alt+Shift+B")
	}
}

func TestPreferencesStoreDefaultsBrowserSwitcherShortcuts(t *testing.T) {
	store := NewPreferencesStore(openTestDatabase(t))
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loaded.BrowserSwitcherShortcut != "Alt+X" {
		t.Fatalf("default shortcut = %q, want Alt+X", loaded.BrowserSwitcherShortcut)
	}
	if loaded.BrowserSwitcherBackwardShortcut != "Alt+Z" {
		t.Fatalf("default backward shortcut = %q, want Alt+Z", loaded.BrowserSwitcherBackwardShortcut)
	}
	if loaded.BrowserAppearances == nil || len(loaded.BrowserAppearances) != 0 {
		t.Fatalf("default browser appearances = %#v, want non-nil empty map", loaded.BrowserAppearances)
	}
}

func TestPreferencesStorePersistsBrowserAppearances(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.BrowserAppearances = map[string]preferencesdomain.BrowserAppearance{
		"proxy:server-1:google-chrome": {
			Icon:         "X",
			PrimaryColor: "#00A651",
		},
		"regular:google-chrome": {
			Icon: "icon:globe",
		},
	}

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if !reflect.DeepEqual(loaded.BrowserAppearances, pref.BrowserAppearances) {
		t.Fatalf("browser appearances = %#v, want %#v", loaded.BrowserAppearances, pref.BrowserAppearances)
	}
}

func TestPreferencesStorePersistsNilBrowserAppearancesAsEmptyObject(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.BrowserAppearances = nil

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT browser_appearances_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("load stored browser appearances JSON: %v", err)
	}
	if stored != "{}" {
		t.Fatalf("stored browser appearances = %q, want {}", stored)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loaded.BrowserAppearances == nil || len(loaded.BrowserAppearances) != 0 {
		t.Fatalf("loaded browser appearances = %#v, want non-nil empty map", loaded.BrowserAppearances)
	}
}

func TestPreferencesStoreRejectsMalformedBrowserAppearancesJSON(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if _, err := db.Exec(`UPDATE user_preferences SET browser_appearances_json = '{' WHERE id = 1`); err != nil {
		t.Fatalf("corrupt browser appearances JSON: %v", err)
	}

	_, err := store.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "load browser appearances") {
		t.Fatalf("load error = %v, want malformed browser appearances error", err)
	}
}
