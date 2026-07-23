package preferences

import (
	"context"
	"fmt"
	"path/filepath"
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
	pref.DefaultBrowserID = strings.TrimSpace(pref.DefaultBrowserID)
	pref.ProxyBrowserID = strings.TrimSpace(pref.ProxyBrowserID)
	pref.CustomBrowsers, err = normalizeCustomBrowsers(pref.CustomBrowsers)
	if err != nil {
		return UserPreference{}, err
	}
	pref.URLRules, err = normalizeURLRules(pref.URLRules)
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
	pref.DefaultBrowserID = strings.TrimSpace(pref.DefaultBrowserID)
	pref.ProxyBrowserID = strings.TrimSpace(pref.ProxyBrowserID)
	pref.CustomBrowsers, err = normalizeCustomBrowsers(pref.CustomBrowsers)
	if err != nil {
		return UserPreference{}, err
	}
	pref.URLRules, err = normalizeURLRules(pref.URLRules)
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

func normalizeCustomBrowsers(input []CustomBrowser) ([]CustomBrowser, error) {
	normalized := make([]CustomBrowser, 0, len(input))
	seenIDs := make(map[string]string, len(input))
	seenPaths := make(map[string]string, len(input))
	for _, browser := range input {
		originalID := browser.ID
		originalPath := browser.LaunchReference
		browser.ID = strings.TrimSpace(browser.ID)
		browser.DisplayName = strings.TrimSpace(browser.DisplayName)
		browser.LaunchReference = strings.TrimSpace(browser.LaunchReference)
		if browser.LaunchReference != "" {
			browser.LaunchReference = filepath.Clean(browser.LaunchReference)
		}
		browser.Engine = BrowserEngine(strings.TrimSpace(string(browser.Engine)))
		if previous, exists := seenIDs[browser.ID]; exists {
			return nil, fmt.Errorf("custom browser duplicate id after trimming: %q and %q", previous, originalID)
		}
		seenIDs[browser.ID] = originalID
		if previous, exists := seenPaths[browser.LaunchReference]; exists {
			return nil, fmt.Errorf("custom browser duplicate application path after cleaning: %q and %q", previous, originalPath)
		}
		seenPaths[browser.LaunchReference] = originalPath
		if err := browser.Validate(); err != nil {
			return nil, err
		}
		normalized = append(normalized, browser)
	}
	if normalized == nil {
		return []CustomBrowser{}, nil
	}
	return normalized, nil
}

func normalizeURLRules(input []URLRule) ([]URLRule, error) {
	normalized := make([]URLRule, 0, len(input))
	for _, rule := range input {
		rule.ID = strings.TrimSpace(rule.ID)
		rule.Pattern = strings.TrimSpace(rule.Pattern)
		rule.BrowserID = strings.TrimSpace(rule.BrowserID)
		rule.Command = strings.TrimSpace(rule.Command)
		if err := rule.Validate(); err != nil {
			return nil, err
		}
		normalized = append(normalized, rule)
	}
	if normalized == nil {
		return []URLRule{}, nil
	}
	return normalized, nil
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
