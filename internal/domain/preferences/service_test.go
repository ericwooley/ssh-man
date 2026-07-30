package preferences

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
	pref      UserPreference
	saveCalls int
}

func (s *memoryStore) Load(context.Context) (UserPreference, error) {
	return s.pref, nil
}

func (s *memoryStore) Save(_ context.Context, pref UserPreference) error {
	s.pref = pref
	s.saveCalls++
	return nil
}

func TestServiceSaveDefaultsBrowserSwitcherShortcuts(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	saved, err := service.Save(context.Background(), UserPreference{Theme: ThemeDark})
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if saved.BrowserSwitcherShortcut != "Alt+X" {
		t.Fatalf("forward shortcut = %q, want Alt+X", saved.BrowserSwitcherShortcut)
	}
	if saved.BrowserSwitcherBackwardShortcut != "Alt+Z" {
		t.Fatalf("backward shortcut = %q, want Alt+Z", saved.BrowserSwitcherBackwardShortcut)
	}
}

func TestServiceSaveAdvancesRevisionPastAcceptedFutureValue(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	input := Default()
	input.UpdatedAt = time.Now().UTC().Add(time.Hour)
	acceptedRevision := input.UpdatedAt

	saved, err := service.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if !saved.UpdatedAt.After(acceptedRevision) {
		t.Fatalf("saved revision = %s, want after accepted revision %s", saved.UpdatedAt, acceptedRevision)
	}
}

func TestServiceSaveRejectsExhaustedMaximumRevision(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	input := Default()
	input.UpdatedAt = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)

	if _, err := service.Save(context.Background(), input); err == nil {
		t.Fatal("expected maximum preference revision to be rejected")
	}
	if store.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0", store.saveCalls)
	}
}

func TestServiceSaveCanonicalizesBrowserSwitcherShortcuts(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	saved, err := service.Save(context.Background(), UserPreference{
		Theme:                           ThemeLight,
		BrowserSwitcherShortcut:         "option+x",
		BrowserSwitcherBackwardShortcut: "option+shift+z",
	})
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if saved.BrowserSwitcherShortcut != "Alt+X" {
		t.Fatalf("forward shortcut = %q, want Alt+X", saved.BrowserSwitcherShortcut)
	}
	if saved.BrowserSwitcherBackwardShortcut != "Alt+Shift+Z" {
		t.Fatalf("backward shortcut = %q, want Alt+Shift+Z", saved.BrowserSwitcherBackwardShortcut)
	}
}

func TestServiceSaveRejectsIdenticalBrowserSwitcherShortcuts(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	_, err := service.Save(context.Background(), UserPreference{
		Theme:                           ThemeDark,
		BrowserSwitcherShortcut:         "option+x",
		BrowserSwitcherBackwardShortcut: "Alt+X",
	})
	if err == nil || !strings.Contains(err.Error(), "must be different") {
		t.Fatalf("save error = %v, want distinct-shortcuts error", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0", store.saveCalls)
	}
}

func TestServiceSaveRejectsInvalidBackwardShortcut(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	_, err := service.Save(context.Background(), UserPreference{
		Theme:                           ThemeDark,
		BrowserSwitcherShortcut:         "Alt+X",
		BrowserSwitcherBackwardShortcut: "Z",
	})
	if err == nil {
		t.Fatal("expected invalid backward shortcut error")
	}
	if store.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0", store.saveCalls)
	}
}

func TestServiceSaveRejectsDifferentHeldModifiers(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	_, err := service.Save(context.Background(), UserPreference{
		Theme:                           ThemeDark,
		BrowserSwitcherShortcut:         "Ctrl+X",
		BrowserSwitcherBackwardShortcut: "Alt+Z",
	})
	if err == nil || !strings.Contains(err.Error(), "same Control, Alt, and Command modifiers") {
		t.Fatalf("save error = %v, want shared-held-modifiers error", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0", store.saveCalls)
	}
}

func TestServiceSaveNormalizesBrowserAppearances(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)

	saved, err := service.Save(context.Background(), UserPreference{
		Theme: ThemeDark,
		BrowserAppearances: map[string]BrowserAppearance{
			" proxy:server-1:google-chrome ": {Icon: " X ", PrimaryColor: " #00a651 "},
			"regular:google-chrome":          {Icon: " icon:shield "},
			"regular:firefox":                {},
		},
	})
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if len(saved.BrowserAppearances) != 2 {
		t.Fatalf("browser appearances = %#v, want two non-empty entries", saved.BrowserAppearances)
	}
	proxy := saved.BrowserAppearances["proxy:server-1:google-chrome"]
	if proxy.Icon != "X" || proxy.PrimaryColor != "#00A651" {
		t.Fatalf("normalized proxy appearance = %#v", proxy)
	}
	regular := saved.BrowserAppearances["regular:google-chrome"]
	if regular.Icon != "icon:shield" || regular.PrimaryColor != "" {
		t.Fatalf("normalized regular appearance = %#v", regular)
	}
	if _, exists := saved.BrowserAppearances["regular:firefox"]; exists {
		t.Fatal("empty browser appearance should be removed")
	}
	if store.pref.BrowserAppearances["proxy:server-1:google-chrome"] != proxy {
		t.Fatalf("stored browser appearances were not normalized: %#v", store.pref.BrowserAppearances)
	}
}

func TestUserPreferenceValidateAcceptsLowercaseBrowserColorBeforeNormalization(t *testing.T) {
	pref := Default()
	pref.BrowserAppearances = map[string]BrowserAppearance{
		"regular:google-chrome": {PrimaryColor: "#00a651"},
	}
	if err := pref.Validate(); err != nil {
		t.Fatalf("validate lowercase browser color: %v", err)
	}
}

func TestServiceLoadNormalizesBrowserAppearancesAndEmptyMap(t *testing.T) {
	pref := Default()
	pref.BrowserAppearances = map[string]BrowserAppearance{
		" regular:google-chrome ": {Icon: " ⭐ ", PrimaryColor: " #abcdef "},
	}
	service := NewService(&memoryStore{pref: pref})

	loaded, err := service.Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	appearance := loaded.BrowserAppearances["regular:google-chrome"]
	if appearance.Icon != "⭐" || appearance.PrimaryColor != "#ABCDEF" {
		t.Fatalf("loaded appearance = %#v", appearance)
	}

	pref.BrowserAppearances = nil
	loaded, err = NewService(&memoryStore{pref: pref}).Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences with nil appearances: %v", err)
	}
	if loaded.BrowserAppearances == nil || len(loaded.BrowserAppearances) != 0 {
		t.Fatalf("empty browser appearances = %#v, want non-nil empty map", loaded.BrowserAppearances)
	}
}

func TestServiceSaveAcceptsSupportedBrowserAppearanceIcons(t *testing.T) {
	icons := []string{
		"icon:x",
		"icon:shield",
		"icon:terminal",
		"icon:globe",
		"icon:network",
		"icon:star",
		"icon:briefcase",
		"icon:code",
		"🟢",
		"👩‍💻",
		"X",
	}
	for _, icon := range icons {
		t.Run(icon, func(t *testing.T) {
			store := &memoryStore{}
			service := NewService(store)
			_, err := service.Save(context.Background(), UserPreference{
				Theme: ThemeDark,
				BrowserAppearances: map[string]BrowserAppearance{
					"regular:google-chrome": {Icon: icon},
				},
			})
			if err != nil {
				t.Fatalf("save supported icon %q: %v", icon, err)
			}
		})
	}
}

func TestServiceSaveRejectsInvalidBrowserAppearances(t *testing.T) {
	tests := []struct {
		name        string
		appearances map[string]BrowserAppearance
		wantError   string
	}{
		{
			name:        "empty key",
			appearances: map[string]BrowserAppearance{" ": {Icon: "X"}},
			wantError:   "key is required",
		},
		{
			name:        "unsafe key",
			appearances: map[string]BrowserAppearance{"regular/google-chrome": {Icon: "X"}},
			wantError:   "unsupported characters",
		},
		{
			name:        "long key",
			appearances: map[string]BrowserAppearance{strings.Repeat("a", maxBrowserAppearanceKeyBytes+1): {Icon: "X"}},
			wantError:   "at most 256",
		},
		{
			name:        "invalid color",
			appearances: map[string]BrowserAppearance{"regular:google-chrome": {PrimaryColor: "green"}},
			wantError:   "#RRGGBB",
		},
		{
			name:        "unknown icon token",
			appearances: map[string]BrowserAppearance{"regular:google-chrome": {Icon: "icon:unknown"}},
			wantError:   "not supported",
		},
		{
			name:        "control character",
			appearances: map[string]BrowserAppearance{"regular:google-chrome": {Icon: "X\nY"}},
			wantError:   "control characters",
		},
		{
			name:        "long custom mark",
			appearances: map[string]BrowserAppearance{"regular:google-chrome": {Icon: strings.Repeat("X", maxBrowserIconGraphemes+1)}},
			wantError:   "at most 2",
		},
		{
			name:        "markup custom mark",
			appearances: map[string]BrowserAppearance{"regular:google-chrome": {Icon: "<"}},
			wantError:   "markup characters",
		},
		{
			name:        "invalid utf8 custom mark",
			appearances: map[string]BrowserAppearance{"regular:google-chrome": {Icon: string([]byte{0xff})}},
			wantError:   "valid UTF-8",
		},
		{
			name: "duplicate normalized key",
			appearances: map[string]BrowserAppearance{
				"regular:google-chrome":  {Icon: "X"},
				" regular:google-chrome": {Icon: "🟢"},
			},
			wantError: "duplicates after trimming",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &memoryStore{}
			service := NewService(store)
			_, err := service.Save(context.Background(), UserPreference{
				Theme:              ThemeDark,
				BrowserAppearances: test.appearances,
			})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("save error = %v, want error containing %q", err, test.wantError)
			}
			if store.saveCalls != 0 {
				t.Fatalf("save calls = %d, want 0", store.saveCalls)
			}
		})
	}
}

func TestServiceSaveNormalizesAndValidatesURLRoutingRules(t *testing.T) {
	store := &memoryStore{pref: Default()}
	service := NewService(store)
	input := Default()
	input.DefaultBrowserID = "  safari "
	input.ProxyBrowserID = " google-chrome "
	input.URLRules = []URLRule{
		{
			ID:        " work ",
			Pattern:   ` https:\/\/github\.com\/workorg\/.* `,
			Action:    URLRuleActionBrowser,
			BrowserID: " brave-browser ",
		},
		{
			ID:      "container",
			Pattern: `^https://intranet\.example/`,
			Action:  URLRuleActionCommand,
			Command: ` open -a "Zen" "ext+container:name=Work&url=<URL>" `,
		},
	}

	saved, err := service.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if saved.DefaultBrowserID != "safari" || saved.ProxyBrowserID != "google-chrome" {
		t.Fatalf("browser ids were not normalized: %#v", saved)
	}
	if got := saved.URLRules[0]; got.ID != "work" || got.Pattern != `https:\/\/github\.com\/workorg\/.*` || got.BrowserID != "brave-browser" {
		t.Fatalf("browser rule was not normalized: %#v", got)
	}
	if got := saved.URLRules[1].Command; got != `open -a "Zen" "ext+container:name=Work&url=<URL>"` {
		t.Fatalf("command rule was not normalized: %q", got)
	}
}

func TestServiceSaveNormalizesBrowserVisibilityAndLiteralRuleModes(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	input := Default()
	input.DisabledBrowserIDs = []string{" firefox ", "safari", "firefox"}
	input.URLRules = []URLRule{
		{
			ID:         "work",
			MatchMode:  URLRuleMatchStartsWith,
			Pattern:    " https://github.com/workorg/ ",
			Action:     URLRuleActionBrowser,
			BrowserID:  " brave-browser ",
			OpenDirect: true,
		},
		{
			ID:        "issues",
			MatchMode: URLRuleMatchContains,
			Pattern:   " ticket= ",
			Action:    URLRuleActionBrowser,
			BrowserID: "firefox",
		},
	}

	saved, err := service.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if got, want := saved.DisabledBrowserIDs, []string{"firefox", "safari"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("disabled browser ids = %#v, want %#v", got, want)
	}
	if got := saved.URLRules[0]; got.MatchMode != URLRuleMatchStartsWith || got.Pattern != "https://github.com/workorg/" || !got.OpenDirect {
		t.Fatalf("normalized starts-with rule = %#v", got)
	}
	if got := saved.URLRules[1]; got.MatchMode != URLRuleMatchContains || got.Pattern != "ticket=" {
		t.Fatalf("normalized contains rule = %#v", got)
	}
}

func TestServiceLoadDefaultsLegacyURLRulesToRegex(t *testing.T) {
	pref := Default()
	pref.URLRules = []URLRule{{
		ID:        "legacy",
		Pattern:   `^https://example\.com/`,
		Action:    URLRuleActionBrowser,
		BrowserID: "safari",
	}}

	loaded, err := NewService(&memoryStore{pref: pref}).Load(context.Background())
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if got := loaded.URLRules[0].MatchMode; got != URLRuleMatchRegex {
		t.Fatalf("legacy match mode = %q, want %q", got, URLRuleMatchRegex)
	}
}

func TestServiceSaveNormalizesCustomBrowsers(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	input := Default()
	applicationPath := filepath.Join(t.TempDir(), "Applications", "Kagi Browser.app")
	input.CustomBrowsers = []CustomBrowser{
		{
			ID:              " custom-kagi ",
			DisplayName:     " Kagi Browser ",
			LaunchReference: applicationPath + string(filepath.Separator),
			Engine:          BrowserEngineChromium,
		},
	}

	saved, err := service.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	wantPath := applicationPath
	if len(saved.CustomBrowsers) != 1 {
		t.Fatalf("custom browsers = %#v, want one entry", saved.CustomBrowsers)
	}
	got := saved.CustomBrowsers[0]
	if got.ID != "custom-kagi" || got.DisplayName != "Kagi Browser" || got.LaunchReference != wantPath || got.Engine != BrowserEngineChromium {
		t.Fatalf("normalized custom browser = %#v", got)
	}
}

func TestServiceSaveNormalizesCommandCustomBrowserWithIcon(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	input := Default()
	input.CustomBrowsers = []CustomBrowser{{
		ID:          " custom-work ",
		DisplayName: " Work profile ",
		Command:     ` open -a "Zen" "ext+container:name=Work&url=<URL>" `,
		Icon:        " icon:briefcase ",
	}}

	saved, err := service.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	got := saved.CustomBrowsers[0]
	if got.ID != "custom-work" || got.DisplayName != "Work profile" ||
		got.Command != `open -a "Zen" "ext+container:name=Work&url=<URL>"` ||
		got.Icon != "icon:briefcase" {
		t.Fatalf("normalized custom browser = %#v", got)
	}
}

func TestServiceSaveRejectsInvalidCustomBrowsers(t *testing.T) {
	applicationDirectory := filepath.Join(t.TempDir(), "Applications")
	valid := CustomBrowser{
		ID:              "custom-kagi",
		DisplayName:     "Kagi Browser",
		LaunchReference: filepath.Join(applicationDirectory, "Kagi Browser.app"),
		Engine:          BrowserEngineChromium,
	}
	tests := []struct {
		name      string
		browsers  []CustomBrowser
		wantError string
	}{
		{
			name:      "missing name",
			browsers:  []CustomBrowser{{ID: "custom-kagi", LaunchReference: valid.LaunchReference, Engine: BrowserEngineChromium}},
			wantError: "display name is required",
		},
		{
			name:      "relative path",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", LaunchReference: "Kagi.app", Engine: BrowserEngineChromium}},
			wantError: "absolute path",
		},
		{
			name:      "unknown engine",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", LaunchReference: valid.LaunchReference, Engine: BrowserEngine("webkit-maybe")}},
			wantError: "engine must be",
		},
		{
			name:      "duplicate ids",
			browsers:  []CustomBrowser{valid, {ID: " custom-kagi ", DisplayName: "Other", LaunchReference: filepath.Join(applicationDirectory, "Other.app"), Engine: BrowserEngineRegular}},
			wantError: "duplicate id",
		},
		{
			name:      "duplicate paths",
			browsers:  []CustomBrowser{valid, {ID: "custom-other", DisplayName: "Other", LaunchReference: valid.LaunchReference + string(filepath.Separator), Engine: BrowserEngineRegular}},
			wantError: "duplicate application path",
		},
		{
			name:      "shell pipeline command",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `printf '%s' <URL> | open`}},
			wantError: "shell operators",
		},
		{
			name:      "nested shell command",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `/usr/bin/nice /bin/sh -c "open <URL>"`}},
			wantError: "executable must be open",
		},
		{
			name:      "combined interpreter flag",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `python3 "-cprint('<URL>')"`}},
			wantError: "executable must be open",
		},
		{
			name:      "versioned interpreter flag",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `perl5.34 "-eprint('<URL>')"`}},
			wantError: "executable must be open",
		},
		{
			name:      "nested interpreter behind delegator",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `/usr/bin/caffeinate /bin/sh -c "open <URL>"`}},
			wantError: "executable must be open",
		},
		{
			name:      "attached cmd command",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `cmd.exe "/cecho <URL>"`}},
			wantError: "executable must be open",
		},
		{
			name:      "positional awk program",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `awk "BEGIN { system(\"printf %s <URL>\") }"`}},
			wantError: "executable must be open",
		},
		{
			name:      "nested osascript source",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `/usr/bin/caffeinate /usr/bin/osascript -e "do shell script \"echo <URL>\""`}},
			wantError: "executable must be open",
		},
		{
			name:      "nested environment splitter",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `/usr/bin/caffeinate /usr/bin/env -S "printf %s <URL>"`}},
			wantError: "executable must be open",
		},
		{
			name:      "nested parallel command",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `/usr/bin/caffeinate parallel <URL> ::: 1`}},
			wantError: "executable must be open",
		},
		{
			name:      "open child arguments",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `open -a Terminal --args -c <URL>`}},
			wantError: "must not forward",
		},
		{
			name:      "non-system open executable",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `/tmp/open <URL>`}},
			wantError: "open or /usr/bin/open",
		},
		{
			name:      "URL-derived open executable",
			browsers:  []CustomBrowser{{ID: "custom-kagi", DisplayName: "Kagi", Command: `<URL>/open`}},
			wantError: "open or /usr/bin/open",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &memoryStore{}
			input := Default()
			input.CustomBrowsers = test.browsers
			_, err := NewService(store).Save(context.Background(), input)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("save error = %v, want error containing %q", err, test.wantError)
			}
			if store.saveCalls != 0 {
				t.Fatalf("save calls = %d, want 0", store.saveCalls)
			}
		})
	}
}

func TestServiceSavePreservesUnchangedLegacyCommandTemplatesDuringUnrelatedEdits(t *testing.T) {
	pref := Default()
	pref.Theme = ThemeLight
	pref.CustomBrowsers = []CustomBrowser{{
		ID:          "legacy-browser",
		DisplayName: "Legacy browser",
		Command:     `printf '%s' <URL> | open`,
	}}
	pref.URLRules = []URLRule{{
		ID:        "legacy-rule",
		MatchMode: URLRuleMatchContains,
		Pattern:   "example.com",
		Action:    URLRuleActionCommand,
		Command:   `/bin/zsh -lc "open <URL>"`,
	}}
	store := &memoryStore{pref: pref}

	loaded, err := NewService(store).Load(context.Background())
	if err != nil {
		t.Fatalf("load legacy preferences: %v", err)
	}
	if loaded.Theme != ThemeLight ||
		loaded.CustomBrowsers[0].Command != `printf '%s' <URL> | open` ||
		loaded.URLRules[0].Command != `/bin/zsh -lc "open <URL>"` {
		t.Fatalf("loaded preferences = %#v, want legacy commands preserved", loaded)
	}
	loaded.Theme = ThemeDark
	saved, err := NewService(store).Save(context.Background(), loaded)
	if err != nil {
		t.Fatalf("save unrelated theme change: %v", err)
	}
	if saved.Theme != ThemeDark ||
		saved.CustomBrowsers[0].Command != `printf '%s' <URL> | open` ||
		saved.URLRules[0].Command != `/bin/zsh -lc "open <URL>"` {
		t.Fatalf("saved preferences = %#v, want unchanged legacy commands preserved", saved)
	}
	if store.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", store.saveCalls)
	}

	changed := saved
	changed.URLRules = append([]URLRule(nil), saved.URLRules...)
	changed.URLRules[0].Command = `/bin/sh -c "open <URL>"`
	if _, err := NewService(store).Save(context.Background(), changed); err == nil {
		t.Fatal("expected changed legacy command syntax to require repair")
	}
	if store.saveCalls != 1 {
		t.Fatalf("save calls = %d, want changed legacy command rejected", store.saveCalls)
	}
}

func TestURLRoutingRuleValidationRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name string
		rule URLRule
	}{
		{
			name: "invalid regex",
			rule: URLRule{ID: "bad-regex", Pattern: "[", Action: URLRuleActionBrowser, BrowserID: "safari"},
		},
		{
			name: "browser without browser id",
			rule: URLRule{ID: "missing-browser", Pattern: ".*", Action: URLRuleActionBrowser},
		},
		{
			name: "command without placeholder",
			rule: URLRule{ID: "missing-url", Pattern: ".*", Action: URLRuleActionCommand, Command: "open -a Safari"},
		},
		{
			name: "command with nested interpreter",
			rule: URLRule{ID: "nested-shell", Pattern: ".*", Action: URLRuleActionCommand, Command: `/usr/bin/nohup /bin/zsh -lc "open <URL>"`},
		},
		{
			name: "unknown action",
			rule: URLRule{ID: "unknown", Pattern: ".*", Action: URLRuleAction("other")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := Default()
			input.URLRules = []URLRule{tt.rule}
			if err := input.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestURLRoutingRuleValidationRejectsDuplicateIDs(t *testing.T) {
	input := Default()
	input.URLRules = []URLRule{
		{ID: "work", Pattern: "github", Action: URLRuleActionBrowser, BrowserID: "safari"},
		{ID: "work", Pattern: "linear", Action: URLRuleActionBrowser, BrowserID: "safari"},
	}

	if err := input.Validate(); err == nil {
		t.Fatal("expected duplicate rule id validation error")
	}
}

func TestServiceNormalizesURLPortAssignments(t *testing.T) {
	store := &memoryStore{}
	input := Default()
	input.URLPortAssignments = []URLPortAssignment{{
		ID:        " docs ",
		Port:      3000,
		ServerID:  " staging ",
		BrowserID: " firefox ",
	}}

	saved, err := NewService(store).Save(context.Background(), input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	want := URLPortAssignment{ID: "docs", Port: 3000, ServerID: "staging", BrowserID: "firefox"}
	if len(saved.URLPortAssignments) != 1 || saved.URLPortAssignments[0] != want {
		t.Fatalf("assignments = %#v, want %#v", saved.URLPortAssignments, want)
	}
}

func TestURLPortAssignmentValidationRejectsInvalidAndDuplicatePorts(t *testing.T) {
	tests := []struct {
		name        string
		assignments []URLPortAssignment
	}{
		{name: "missing id", assignments: []URLPortAssignment{{Port: 3000, ServerID: "host", BrowserID: "firefox"}}},
		{name: "invalid port", assignments: []URLPortAssignment{{ID: "docs", Port: 70000, ServerID: "host", BrowserID: "firefox"}}},
		{name: "missing host", assignments: []URLPortAssignment{{ID: "docs", Port: 3000, BrowserID: "firefox"}}},
		{name: "missing browser", assignments: []URLPortAssignment{{ID: "docs", Port: 3000, ServerID: "host"}}},
		{name: "duplicate port", assignments: []URLPortAssignment{
			{ID: "docs", Port: 3000, ServerID: "host", BrowserID: "firefox"},
			{ID: "api", Port: 3000, ServerID: "other", BrowserID: "google-chrome"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := Default()
			input.URLPortAssignments = tt.assignments
			if err := input.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
