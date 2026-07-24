package preferences

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
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

func TestServiceSaveNormalizesCustomBrowsers(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	input := Default()
	input.CustomBrowsers = []CustomBrowser{
		{
			ID:              " custom-kagi ",
			DisplayName:     " Kagi Browser ",
			LaunchReference: filepath.Join(string(filepath.Separator), "Applications", "Kagi Browser.app") + string(filepath.Separator),
			Engine:          BrowserEngineChromium,
		},
	}

	saved, err := service.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	wantPath := filepath.Join(string(filepath.Separator), "Applications", "Kagi Browser.app")
	if len(saved.CustomBrowsers) != 1 {
		t.Fatalf("custom browsers = %#v, want one entry", saved.CustomBrowsers)
	}
	got := saved.CustomBrowsers[0]
	if got.ID != "custom-kagi" || got.DisplayName != "Kagi Browser" || got.LaunchReference != wantPath || got.Engine != BrowserEngineChromium {
		t.Fatalf("normalized custom browser = %#v", got)
	}
}

func TestServiceSaveRejectsInvalidCustomBrowsers(t *testing.T) {
	valid := CustomBrowser{
		ID:              "custom-kagi",
		DisplayName:     "Kagi Browser",
		LaunchReference: filepath.Join(string(filepath.Separator), "Applications", "Kagi Browser.app"),
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
			browsers:  []CustomBrowser{valid, {ID: " custom-kagi ", DisplayName: "Other", LaunchReference: filepath.Join(string(filepath.Separator), "Applications", "Other.app"), Engine: BrowserEngineRegular}},
			wantError: "duplicate id",
		},
		{
			name:      "duplicate paths",
			browsers:  []CustomBrowser{valid, {ID: "custom-other", DisplayName: "Other", LaunchReference: valid.LaunchReference + string(filepath.Separator), Engine: BrowserEngineRegular}},
			wantError: "duplicate application path",
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
