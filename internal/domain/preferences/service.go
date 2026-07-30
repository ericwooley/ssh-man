package preferences

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ssh-man/internal/domain/browsercommand"
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

var maximumPreferenceRevision = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)

func nextPreferenceRevision(previous, now time.Time) (time.Time, error) {
	next := now.UTC()
	previous = previous.UTC()
	if next.After(previous) {
		return next, nil
	}
	if !previous.Before(maximumPreferenceRevision) {
		return time.Time{}, fmt.Errorf("preference revision cannot advance past %s", maximumPreferenceRevision.Format(time.RFC3339Nano))
	}
	return previous.Add(time.Nanosecond), nil
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
	pref.DisabledBrowserIDs, err = normalizeBrowserIDs(pref.DisabledBrowserIDs)
	if err != nil {
		return UserPreference{}, err
	}
	pref.CustomBrowsers, err = normalizeCustomBrowsers(pref.CustomBrowsers, true)
	if err != nil {
		return UserPreference{}, err
	}
	pref.URLRules, err = normalizeURLRules(pref.URLRules, true)
	if err != nil {
		return UserPreference{}, err
	}
	pref.URLPortAssignments, err = normalizeURLPortAssignments(pref.URLPortAssignments)
	if err != nil {
		return UserPreference{}, err
	}
	if err := pref.validate(true); err != nil {
		return UserPreference{}, err
	}
	return pref, nil
}

func (s *Service) Save(ctx context.Context, pref UserPreference) (UserPreference, error) {
	stored, err := s.store.Load(ctx)
	if err != nil {
		return UserPreference{}, err
	}
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
	pref.DisabledBrowserIDs, err = normalizeBrowserIDs(pref.DisabledBrowserIDs)
	if err != nil {
		return UserPreference{}, err
	}
	pref.CustomBrowsers, err = normalizeCustomBrowsers(pref.CustomBrowsers, true)
	if err != nil {
		return UserPreference{}, err
	}
	pref.URLRules, err = normalizeURLRules(pref.URLRules, true)
	if err != nil {
		return UserPreference{}, err
	}
	pref.URLPortAssignments, err = normalizeURLPortAssignments(pref.URLPortAssignments)
	if err != nil {
		return UserPreference{}, err
	}
	if err := validateChangedCommandTemplates(stored, pref); err != nil {
		return UserPreference{}, err
	}
	pref.UpdatedAt, err = nextPreferenceRevision(pref.UpdatedAt, time.Now())
	if err != nil {
		return UserPreference{}, err
	}
	if err := pref.validate(true); err != nil {
		return UserPreference{}, err
	}
	if err := s.store.Save(ctx, pref); err != nil {
		return UserPreference{}, err
	}
	return pref, nil
}

func validateChangedCommandTemplates(stored, next UserPreference) error {
	storedBrowserCommands := make(map[string]string, len(stored.CustomBrowsers))
	for _, browser := range stored.CustomBrowsers {
		storedBrowserCommands[strings.TrimSpace(browser.ID)] = strings.TrimSpace(browser.Command)
	}
	for index, browser := range next.CustomBrowsers {
		if browser.Command == "" {
			continue
		}
		if err := browsercommand.Validate(browser.Command); err != nil {
			if storedBrowserCommands[browser.ID] == browser.Command {
				continue
			}
			return fmt.Errorf("custom browser %d: command: %w", index+1, err)
		}
	}

	storedRuleCommands := make(map[string]string, len(stored.URLRules))
	for _, rule := range stored.URLRules {
		if rule.Action == URLRuleActionCommand {
			storedRuleCommands[strings.TrimSpace(rule.ID)] = strings.TrimSpace(rule.Command)
		}
	}
	for index, rule := range next.URLRules {
		if rule.Action != URLRuleActionCommand {
			continue
		}
		if err := browsercommand.Validate(rule.Command); err != nil {
			if storedRuleCommands[rule.ID] == rule.Command {
				continue
			}
			return fmt.Errorf("URL rule %d: command: %w", index+1, err)
		}
	}
	return nil
}

func normalizeCustomBrowsers(input []CustomBrowser, allowLegacyCommandSyntax bool) ([]CustomBrowser, error) {
	normalized := make([]CustomBrowser, 0, len(input))
	seenIDs := make(map[string]string, len(input))
	seenCommands := make(map[string]string, len(input))
	seenPaths := make(map[string]string, len(input))
	for _, browser := range input {
		originalID := browser.ID
		originalLaunch := browser.Command
		if originalLaunch == "" {
			originalLaunch = browser.LaunchReference
		}
		browser.ID = strings.TrimSpace(browser.ID)
		browser.DisplayName = strings.TrimSpace(browser.DisplayName)
		browser.Command = strings.TrimSpace(browser.Command)
		browser.Icon = strings.TrimSpace(browser.Icon)
		browser.LaunchReference = strings.TrimSpace(browser.LaunchReference)
		if browser.Command != "" {
			browser.LaunchReference = ""
			browser.Engine = ""
		} else if browser.LaunchReference != "" {
			browser.LaunchReference = filepath.Clean(browser.LaunchReference)
		}
		browser.Engine = BrowserEngine(strings.TrimSpace(string(browser.Engine)))
		if previous, exists := seenIDs[browser.ID]; exists {
			return nil, fmt.Errorf("custom browser duplicate id after trimming: %q and %q", previous, originalID)
		}
		seenIDs[browser.ID] = originalID
		if browser.Command != "" {
			if previous, exists := seenCommands[browser.Command]; exists {
				return nil, fmt.Errorf("custom browser duplicate command after trimming: %q and %q", previous, originalLaunch)
			}
			seenCommands[browser.Command] = originalLaunch
		} else {
			if previous, exists := seenPaths[browser.LaunchReference]; exists {
				return nil, fmt.Errorf("custom browser duplicate application path after cleaning: %q and %q", previous, originalLaunch)
			}
			seenPaths[browser.LaunchReference] = originalLaunch
		}
		if err := browser.validate(allowLegacyCommandSyntax); err != nil {
			return nil, err
		}
		normalized = append(normalized, browser)
	}
	if normalized == nil {
		return []CustomBrowser{}, nil
	}
	return normalized, nil
}

func normalizeURLRules(input []URLRule, allowLegacyCommandSyntax bool) ([]URLRule, error) {
	normalized := make([]URLRule, 0, len(input))
	for _, rule := range input {
		rule.ID = strings.TrimSpace(rule.ID)
		rule.MatchMode = URLRuleMatchMode(strings.TrimSpace(string(rule.MatchMode)))
		if rule.MatchMode == "" {
			rule.MatchMode = URLRuleMatchRegex
		}
		rule.Pattern = strings.TrimSpace(rule.Pattern)
		rule.BrowserID = strings.TrimSpace(rule.BrowserID)
		rule.Command = strings.TrimSpace(rule.Command)
		if err := rule.validate(allowLegacyCommandSyntax); err != nil {
			return nil, err
		}
		normalized = append(normalized, rule)
	}
	if normalized == nil {
		return []URLRule{}, nil
	}
	return normalized, nil
}

func normalizeBrowserIDs(input []string) ([]string, error) {
	normalized := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, value := range input {
		browserID := strings.TrimSpace(value)
		if err := validateBrowserID("disabled browser", browserID, true); err != nil {
			return nil, err
		}
		if _, exists := seen[browserID]; exists {
			continue
		}
		seen[browserID] = struct{}{}
		normalized = append(normalized, browserID)
	}
	sort.Strings(normalized)
	if normalized == nil {
		return []string{}, nil
	}
	return normalized, nil
}

func normalizeURLPortAssignments(input []URLPortAssignment) ([]URLPortAssignment, error) {
	normalized := make([]URLPortAssignment, 0, len(input))
	for _, assignment := range input {
		assignment.ID = strings.TrimSpace(assignment.ID)
		assignment.ServerID = strings.TrimSpace(assignment.ServerID)
		assignment.BrowserID = strings.TrimSpace(assignment.BrowserID)
		if err := assignment.Validate(); err != nil {
			return nil, err
		}
		normalized = append(normalized, assignment)
	}
	if normalized == nil {
		return []URLPortAssignment{}, nil
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
