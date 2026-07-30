package sqlite

import (
	"context"
	"encoding/json"
	"reflect"
	"regexp"
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
	pref.DisabledBrowserIDs = []string{"arc", "safari"}
	pref.URLRules = []preferencesdomain.URLRule{
		{ID: "work", MatchMode: preferencesdomain.URLRuleMatchStartsWith, Pattern: "https://github.com/workorg/", Action: preferencesdomain.URLRuleActionBrowser, BrowserID: "firefox", OpenDirect: true},
		{ID: "container", Pattern: `^https://intranet\.example/`, Action: preferencesdomain.URLRuleActionCommand, Command: `open -a "Zen" "<URL>"`},
	}
	pref.CustomBrowsers = []preferencesdomain.CustomBrowser{
		{
			ID:          "custom-kagi",
			DisplayName: "Kagi Browser",
			Command:     `open -a "Kagi Browser" "<URL>"`,
			Icon:        "icon:star",
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
	if !reflect.DeepEqual(loaded.DisabledBrowserIDs, pref.DisabledBrowserIDs) {
		t.Fatalf("disabled browser ids = %#v, want %#v", loaded.DisabledBrowserIDs, pref.DisabledBrowserIDs)
	}
	if !reflect.DeepEqual(loaded.URLRules, pref.URLRules) {
		t.Fatalf("URL rules = %#v, want %#v", loaded.URLRules, pref.URLRules)
	}
	if !reflect.DeepEqual(loaded.CustomBrowsers, pref.CustomBrowsers) {
		t.Fatalf("custom browsers = %#v, want %#v", loaded.CustomBrowsers, pref.CustomBrowsers)
	}
}

func TestPreferencesStoreKeepsNewBrowserRoutingCompatibleWithPreviousReaders(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.CustomBrowsers = []preferencesdomain.CustomBrowser{
		{
			ID:              "legacy-kagi",
			DisplayName:     "Kagi Browser",
			LaunchReference: "/Applications/Kagi Browser.app",
			Engine:          preferencesdomain.BrowserEngineChromium,
		},
		{
			ID:          "command-work",
			DisplayName: "Work Browser",
			Command:     `open -a "Safari" "<URL>"`,
			Icon:        "icon:briefcase",
		},
	}
	pref.URLRules = []preferencesdomain.URLRule{
		{
			ID:         "literal-bracket",
			MatchMode:  preferencesdomain.URLRuleMatchContains,
			Pattern:    "[work]",
			Action:     preferencesdomain.URLRuleActionBrowser,
			BrowserID:  "command-work",
			OpenDirect: true,
		},
		{
			ID:        "regex",
			MatchMode: preferencesdomain.URLRuleMatchRegex,
			Pattern:   `^https://example\.com/`,
			Action:    preferencesdomain.URLRuleActionBrowser,
			BrowserID: "safari",
		},
	}

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}

	var legacyBrowsersJSON, commandBrowsersJSON, legacyRulesJSON, browserRulesJSON string
	if err := db.QueryRow(`
		SELECT custom_browsers_json, command_browsers_json,
		       url_rules_json, browser_routing_rules_json
		FROM user_preferences
		WHERE id = 1
	`).Scan(&legacyBrowsersJSON, &commandBrowsersJSON, &legacyRulesJSON, &browserRulesJSON); err != nil {
		t.Fatalf("load compatibility storage: %v", err)
	}

	type legacyBrowser struct {
		ID              string                          `json:"id"`
		DisplayName     string                          `json:"displayName"`
		LaunchReference string                          `json:"launchReference"`
		Engine          preferencesdomain.BrowserEngine `json:"engine"`
	}
	var legacyBrowsers []legacyBrowser
	if err := json.Unmarshal([]byte(legacyBrowsersJSON), &legacyBrowsers); err != nil {
		t.Fatalf("decode legacy browsers: %v", err)
	}
	if len(legacyBrowsers) != 1 || legacyBrowsers[0].ID != "legacy-kagi" {
		t.Fatalf("legacy browsers = %#v, want only the legacy application browser", legacyBrowsers)
	}

	var commandBrowsers []preferencesdomain.CustomBrowser
	if err := json.Unmarshal([]byte(commandBrowsersJSON), &commandBrowsers); err != nil {
		t.Fatalf("decode command browsers: %v", err)
	}
	if len(commandBrowsers) != 1 || commandBrowsers[0].ID != "command-work" {
		t.Fatalf("command browsers = %#v, want only the command browser", commandBrowsers)
	}

	type legacyRule struct {
		ID        string                          `json:"id"`
		Pattern   string                          `json:"pattern"`
		Action    preferencesdomain.URLRuleAction `json:"action"`
		BrowserID string                          `json:"browserId,omitempty"`
		Command   string                          `json:"command,omitempty"`
	}
	var legacyRules []legacyRule
	if err := json.Unmarshal([]byte(legacyRulesJSON), &legacyRules); err != nil {
		t.Fatalf("decode legacy rules: %v", err)
	}
	if len(legacyRules) != 2 {
		t.Fatalf("legacy rules = %#v, want two projected rules", legacyRules)
	}
	if _, err := regexp.Compile(legacyRules[0].Pattern); err != nil {
		t.Fatalf("literal rule compatibility pattern %q is not valid regex: %v", legacyRules[0].Pattern, err)
	}
	if !regexp.MustCompile(legacyRules[0].Pattern).MatchString("https://example.com/[work]/ticket") {
		t.Fatalf("literal rule compatibility pattern %q does not preserve contains behavior", legacyRules[0].Pattern)
	}

	var browserRules []preferencesdomain.URLRule
	if err := json.Unmarshal([]byte(browserRulesJSON), &browserRules); err != nil {
		t.Fatalf("decode browser routing rules: %v", err)
	}
	if !reflect.DeepEqual(browserRules, pref.URLRules) {
		t.Fatalf("browser routing rules = %#v, want %#v", browserRules, pref.URLRules)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if !reflect.DeepEqual(loaded.CustomBrowsers, pref.CustomBrowsers) {
		t.Fatalf("loaded custom browsers = %#v, want %#v", loaded.CustomBrowsers, pref.CustomBrowsers)
	}
	if !reflect.DeepEqual(loaded.URLRules, pref.URLRules) {
		t.Fatalf("loaded URL rules = %#v, want %#v", loaded.URLRules, pref.URLRules)
	}
}

func TestProjectLegacyURLRulesPreservesMatchBehavior(t *testing.T) {
	tests := []struct {
		name        string
		mode        preferencesdomain.URLRuleMatchMode
		pattern     string
		matching    string
		notMatching string
	}{
		{
			name:        "starts with",
			mode:        preferencesdomain.URLRuleMatchStartsWith,
			pattern:     "https://work.example/[team]",
			matching:    "https://work.example/[team]/ticket",
			notMatching: "prefix-https://work.example/[team]",
		},
		{
			name:        "ends with",
			mode:        preferencesdomain.URLRuleMatchEndsWith,
			pattern:     "/done?status=[ok]",
			matching:    "https://work.example/done?status=[ok]",
			notMatching: "https://work.example/done?status=[ok]&next=1",
		},
		{
			name:        "contains",
			mode:        preferencesdomain.URLRuleMatchContains,
			pattern:     "[work]",
			matching:    "https://example.com/[work]/ticket",
			notMatching: "https://example.com/personal/ticket",
		},
		{
			name:        "regex",
			mode:        preferencesdomain.URLRuleMatchRegex,
			pattern:     `^https://example\.com/[0-9]+$`,
			matching:    "https://example.com/42",
			notMatching: "https://example.com/ticket",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projected := projectLegacyURLRules([]preferencesdomain.URLRule{{
				ID:        "rule",
				MatchMode: test.mode,
				Pattern:   test.pattern,
				Action:    preferencesdomain.URLRuleActionBrowser,
				BrowserID: "safari",
			}})
			if len(projected) != 1 {
				t.Fatalf("projected rules = %#v, want one rule", projected)
			}
			compiled, err := regexp.Compile(projected[0].Pattern)
			if err != nil {
				t.Fatalf("compatibility pattern %q is invalid: %v", projected[0].Pattern, err)
			}
			if !compiled.MatchString(test.matching) {
				t.Fatalf("compatibility pattern %q does not match %q", projected[0].Pattern, test.matching)
			}
			if compiled.MatchString(test.notMatching) {
				t.Fatalf("compatibility pattern %q unexpectedly matches %q", projected[0].Pattern, test.notMatching)
			}
		})
	}
}

func TestPreferencesStorePreservesNewBrowserDataAcrossPreviousVersionSave(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.CustomBrowsers = []preferencesdomain.CustomBrowser{{
		ID:          "command-work",
		DisplayName: "Work Browser",
		Command:     `open -a "Safari" "<URL>"`,
	}}
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:         "literal-work",
		MatchMode:  preferencesdomain.URLRuleMatchContains,
		Pattern:    "[work]",
		Action:     preferencesdomain.URLRuleActionBrowser,
		BrowserID:  "command-work",
		OpenDirect: true,
	}}
	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}

	if _, err := db.Exec(`
		UPDATE user_preferences
		SET custom_browsers_json = ?,
		    theme = 'light',
		    updated_at = '2026-07-29T12:00:00Z'
		WHERE id = 1
	`, `[
		{"id":"legacy-kagi","displayName":"Kagi Browser","launchReference":"/Applications/Kagi Browser.app","engine":"chromium"},
		{"id":"command-work","displayName":"Old Work Browser","launchReference":"/Applications/Safari.app","engine":"regular"}
	]`); err != nil {
		t.Fatalf("simulate previous-version save: %v", err)
	}

	loaded, err := preferencesdomain.NewService(store).Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences after previous-version save: %v", err)
	}
	if loaded.Theme != preferencesdomain.ThemeLight {
		t.Fatalf("theme = %q, want previous-version update to remain visible", loaded.Theme)
	}
	if len(loaded.CustomBrowsers) != 2 ||
		loaded.CustomBrowsers[0].ID != "legacy-kagi" ||
		loaded.CustomBrowsers[1].ID != "command-work" ||
		loaded.CustomBrowsers[1].Command == "" ||
		loaded.CustomBrowsers[1].LaunchReference != "" {
		t.Fatalf("custom browsers = %#v, want distinct previous-version browser plus preserved modern command browser", loaded.CustomBrowsers)
	}
	if !reflect.DeepEqual(loaded.URLRules, pref.URLRules) {
		t.Fatalf("URL rules = %#v, want preserved full routing rules %#v", loaded.URLRules, pref.URLRules)
	}
}

func TestPreferencesStoreLoadsRollbackURLRuleEditsWhenLegacyProjectionDiverges(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:         "modern-rule",
		MatchMode:  preferencesdomain.URLRuleMatchContains,
		Pattern:    "[work]",
		Action:     preferencesdomain.URLRuleActionBrowser,
		BrowserID:  "safari",
		OpenDirect: true,
	}}
	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save modern preferences: %v", err)
	}

	if _, err := db.Exec(`
		UPDATE user_preferences
		SET url_rules_json = ?,
		    updated_at = '2026-07-29T12:00:00Z'
		WHERE id = 1
	`, `[
		{"id":"rollback-rule","pattern":"^https://legacy\\.example/","action":"browser","browserId":"firefox"}
	]`); err != nil {
		t.Fatalf("simulate rollback URL rule edit: %v", err)
	}

	loaded, err := preferencesdomain.NewService(store).Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences after rollback edit: %v", err)
	}
	want := []preferencesdomain.URLRule{{
		ID:        "rollback-rule",
		MatchMode: preferencesdomain.URLRuleMatchRegex,
		Pattern:   `^https://legacy\.example/`,
		Action:    preferencesdomain.URLRuleActionBrowser,
		BrowserID: "firefox",
	}}
	if !reflect.DeepEqual(loaded.URLRules, want) {
		t.Fatalf("URL rules = %#v, want rollback edits %#v", loaded.URLRules, want)
	}
}

func TestPreferencesStoreRejectsDuplicateLegacyBrowserIDsBeforeModernPrecedence(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.CustomBrowsers = []preferencesdomain.CustomBrowser{{
		ID:          "command-work",
		DisplayName: "Work Browser",
		Command:     `open -a "Safari" "<URL>"`,
	}}
	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE user_preferences
		SET custom_browsers_json = ?
		WHERE id = 1
	`, `[
		{"id":"command-work","displayName":"Old Work Browser","launchReference":"/Applications/Safari.app","engine":"regular"},
		{"id":" command-work ","displayName":"Duplicate Work Browser","launchReference":"/Applications/Firefox.app","engine":"regular"}
	]`); err != nil {
		t.Fatalf("seed duplicate legacy browsers: %v", err)
	}

	_, err := store.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "duplicate legacy custom browser id") {
		t.Fatalf("load error = %v, want duplicate legacy custom browser id", err)
	}
}

func TestPreferencesStoreRejectsLegacyBrowserShapeInCommandStorage(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	if err := store.Save(context.Background(), preferencesdomain.Default()); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE user_preferences
		SET command_browsers_json = ?
		WHERE id = 1
	`, `[
		{"id":"legacy-kagi","displayName":"Kagi Browser","launchReference":"/Applications/Kagi Browser.app","engine":"chromium"}
	]`); err != nil {
		t.Fatalf("seed malformed command browser storage: %v", err)
	}

	_, err := store.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "command browser storage contains a browser without a command") {
		t.Fatalf("load error = %v, want command browser partition error", err)
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
	if _, err := db.Exec(`UPDATE user_preferences SET browser_routing_rules_json = '{' WHERE id = 1`); err != nil {
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

func TestPreferencesStorePersistsURLPortAssignments(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.URLPortAssignments = []preferencesdomain.URLPortAssignment{{
		ID:        "docs",
		Port:      3000,
		ServerID:  "staging",
		BrowserID: "firefox",
	}}

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if !reflect.DeepEqual(loaded.URLPortAssignments, pref.URLPortAssignments) {
		t.Fatalf("assignments = %#v, want %#v", loaded.URLPortAssignments, pref.URLPortAssignments)
	}
}

func TestPreferencesStorePersistsNilURLPortAssignmentsAsEmptyArray(t *testing.T) {
	db := openTestDatabase(t)
	store := NewPreferencesStore(db)
	pref := preferencesdomain.Default()
	pref.URLPortAssignments = nil

	if err := store.Save(context.Background(), pref); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT url_port_assignments_json FROM user_preferences WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("load stored URL port assignments JSON: %v", err)
	}
	if stored != "[]" {
		t.Fatalf("stored URL port assignments = %q, want []", stored)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if loaded.URLPortAssignments == nil || len(loaded.URLPortAssignments) != 0 {
		t.Fatalf("loaded assignments = %#v, want non-nil empty slice", loaded.URLPortAssignments)
	}
}
