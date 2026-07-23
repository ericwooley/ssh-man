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

func TestPreferencesStorePersistsURLRoutingSettings(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.ProxyBrowserID = "google-chrome"
	pref.URLRules = []preferencesdomain.URLRule{
		{ID: "work", Pattern: `^https://github\.com/workorg/`, Action: preferencesdomain.URLRuleActionBrowser, BrowserID: "firefox"},
		{ID: "container", Pattern: `^https://intranet\.example/`, Action: preferencesdomain.URLRuleActionCommand, Command: `open -a "Zen" "<URL>"`},
	}
	pref.CustomBrowsers = []preferencesdomain.CustomBrowser{
		{
			ID:              "custom-kagi",
			DisplayName:     "Kagi Browser",
			LaunchReference: "/Applications/Kagi Browser.app",
			Engine:          preferencesdomain.BrowserEngineChromium,
		},
	}

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loaded.DefaultBrowserID != pref.DefaultBrowserID || loaded.ProxyBrowserID != pref.ProxyBrowserID {
		t.Fatalf("browser routing preferences = %#v", loaded)
	}
	if !reflect.DeepEqual(loaded.URLRules, pref.URLRules) {
		t.Fatalf("URL rules = %#v, want %#v", loaded.URLRules, pref.URLRules)
	}
	if !reflect.DeepEqual(loaded.CustomBrowsers, pref.CustomBrowsers) {
		t.Fatalf("custom browsers = %#v, want %#v", loaded.CustomBrowsers, pref.CustomBrowsers)
	}
}

func TestPreferencesStorePersistsNilURLRulesAsEmptyArray(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.URLRules = nil

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT url_rules_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("load stored URL rules JSON: %v", err)
	}
	if stored != "[]" {
		t.Fatalf("stored URL rules = %q, want []", stored)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loaded.URLRules == nil || len(loaded.URLRules) != 0 {
		t.Fatalf("loaded URL rules = %#v, want non-nil empty slice", loaded.URLRules)
	}
}

func TestPreferencesStoreRejectsMalformedURLRulesJSON(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	if err := store.Save(context.Background(), preferencesdomain.Default()); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if _, err := db.Exec(`UPDATE user_preferences SET url_rules_json = '{' WHERE id = 1`); err != nil {
		t.Fatalf("corrupt URL rules JSON: %v", err)
	}

	_, err := store.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "load URL rules") {
		t.Fatalf("load error = %v, want malformed URL rules error", err)
	}
}

func TestPreferencesStorePersistsNilCustomBrowsersAsEmptyArray(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.CustomBrowsers = nil

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT custom_browsers_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("load stored custom browsers JSON: %v", err)
	}
	if stored != "[]" {
		t.Fatalf("stored custom browsers = %q, want []", stored)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loaded.CustomBrowsers == nil || len(loaded.CustomBrowsers) != 0 {
		t.Fatalf("loaded custom browsers = %#v, want non-nil empty slice", loaded.CustomBrowsers)
	}
}

func TestPreferencesStoreRejectsMalformedCustomBrowsersJSON(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	if err := store.Save(context.Background(), preferencesdomain.Default()); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if _, err := db.Exec(`UPDATE user_preferences SET custom_browsers_json = '{' WHERE id = 1`); err != nil {
		t.Fatalf("corrupt custom browsers JSON: %v", err)
	}

	_, err := store.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "load custom browsers") {
		t.Fatalf("load error = %v, want malformed custom browsers error", err)
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
