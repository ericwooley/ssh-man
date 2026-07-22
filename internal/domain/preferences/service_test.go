package preferences

import (
	"context"
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
