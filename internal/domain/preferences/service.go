package preferences

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"ssh-man/internal/keyboardshortcut"
)

type Store interface {
	Load(ctx context.Context) (UserPreference, error)
	Save(ctx context.Context, pref UserPreference) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Load(ctx context.Context) (UserPreference, error) {
	pref, err := s.store.Load(ctx)
	if err != nil {
		return UserPreference{}, err
	}
	pref.BrowserAppearances, err = normalizeBrowserAppearances(pref.BrowserAppearances)
	if err != nil {
		return UserPreference{}, err
	}
	if err := pref.Validate(); err != nil {
		return UserPreference{}, err
	}
	return pref, nil
}

func (s *Service) Save(ctx context.Context, pref UserPreference) (UserPreference, error) {
	if pref.Theme == "" {
		pref.Theme = ThemeDark
	}
	if pref.BrowserSwitcherShortcut == "" {
		pref.BrowserSwitcherShortcut = keyboardshortcut.DefaultBrowserSwitcher
	}
	if pref.BrowserSwitcherBackwardShortcut == "" {
		pref.BrowserSwitcherBackwardShortcut = keyboardshortcut.DefaultBrowserSwitcherBackward
	}
	canonicalShortcut, err := keyboardshortcut.Canonical(pref.BrowserSwitcherShortcut)
	if err != nil {
		return UserPreference{}, err
	}
	canonicalBackwardShortcut, err := keyboardshortcut.Canonical(pref.BrowserSwitcherBackwardShortcut)
	if err != nil {
		return UserPreference{}, err
	}
	pref.BrowserSwitcherShortcut = canonicalShortcut
	pref.BrowserSwitcherBackwardShortcut = canonicalBackwardShortcut
	pref.BrowserAppearances, err = normalizeBrowserAppearances(pref.BrowserAppearances)
	if err != nil {
		return UserPreference{}, err
	}
	pref.UpdatedAt = time.Now().UTC()
	if err := pref.Validate(); err != nil {
		return UserPreference{}, err
	}
	if err := s.store.Save(ctx, pref); err != nil {
		return UserPreference{}, err
	}
	return pref, nil
}

func normalizeBrowserAppearances(input map[string]BrowserAppearance) (map[string]BrowserAppearance, error) {
	normalized := make(map[string]BrowserAppearance, len(input))
	seenKeys := make(map[string]string, len(input))
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		normalizedKey := strings.TrimSpace(key)
		if previous, exists := seenKeys[normalizedKey]; exists {
			return nil, fmt.Errorf("browser appearance keys %q and %q are duplicates after trimming", previous, key)
		}
		seenKeys[normalizedKey] = key

		appearance := input[key]
		appearance.Icon = strings.TrimSpace(appearance.Icon)
		appearance.PrimaryColor = strings.ToUpper(strings.TrimSpace(appearance.PrimaryColor))
		if appearance.Icon == "" && appearance.PrimaryColor == "" {
			continue
		}
		normalized[normalizedKey] = appearance
	}

	return normalized, nil
}
