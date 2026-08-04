package bindings

import (
	"context"
	"errors"
	"slices"
	"strings"

	preferencesdomain "ssh-man/internal/domain/preferences"
	"ssh-man/internal/keyboardshortcut"
)

var errPreferencesChanged = errors.New("preference update conflict")

type URLRulePatternValidationResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

func browserRoutingPreferencesChanged(previous, next preferencesdomain.UserPreference) bool {
	return previous.DefaultBrowserID != next.DefaultBrowserID ||
		previous.ProxyBrowserID != next.ProxyBrowserID ||
		!slices.Equal(previous.DisabledBrowserIDs, next.DisabledBrowserIDs) ||
		!slices.Equal(previous.CustomBrowsers, next.CustomBrowsers) ||
		!slices.Equal(previous.URLRules, next.URLRules) ||
		!slices.Equal(previous.URLPortAssignments, next.URLPortAssignments)
}

func (a *AppBindings) ValidateURLRulePattern(matchMode string, pattern string) URLRulePatternValidationResult {
	if err := preferencesdomain.ValidateURLRulePattern(preferencesdomain.URLRuleMatchMode(matchMode), pattern); err != nil {
		return URLRulePatternValidationResult{
			Valid:   false,
			Message: "Enter a valid regular expression.",
		}
	}
	return URLRulePatternValidationResult{Valid: true}
}

func (a *AppBindings) withPreferenceUpdate(update func() (bool, error)) error {
	if a.app.URLRoutingService != nil {
		return a.app.URLRoutingService.WithPreferenceUpdate(update)
	}
	_, err := update()
	return err
}

func (a *AppBindings) SavePreferences(input preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error) {
	if input.BrowserSwitcherShortcut == "" {
		input.BrowserSwitcherShortcut = keyboardshortcut.DefaultBrowserSwitcher
	}
	if input.BrowserSwitcherBackwardShortcut == "" {
		input.BrowserSwitcherBackwardShortcut = keyboardshortcut.DefaultBrowserSwitcherBackward
	}
	forward, err := keyboardshortcut.Canonical(input.BrowserSwitcherShortcut)
	if err != nil {
		return preferencesdomain.UserPreference{}, err
	}
	backward, err := keyboardshortcut.Canonical(input.BrowserSwitcherBackwardShortcut)
	if err != nil {
		return preferencesdomain.UserPreference{}, err
	}
	input.BrowserSwitcherShortcut = forward
	input.BrowserSwitcherBackwardShortcut = backward
	if err := input.ValidateAllowingLegacyCommandSyntax(); err != nil {
		return preferencesdomain.UserPreference{}, err
	}
	if a.savePreferences != nil {
		saved, err := a.savePreferences(input)
		if err == nil {
			a.notifyPreferencesSaved(saved)
		}
		return saved, err
	}

	var previous preferencesdomain.UserPreference
	var previousErr error
	var shortcutChanged bool
	var pref preferencesdomain.UserPreference
	saveErr := a.withPreferenceUpdate(func() (bool, error) {
		previous, previousErr = a.app.PreferencesService.Load(context.Background())
		if previousErr == nil && !input.UpdatedAt.Equal(previous.UpdatedAt) {
			return false, errPreferencesChanged
		}
		shortcutChanged = previousErr != nil ||
			previous.BrowserSwitcherShortcut != forward ||
			previous.BrowserSwitcherBackwardShortcut != backward
		if shortcutChanged && a.setBrowserShortcuts != nil {
			if err := a.setBrowserShortcuts(forward, backward); err != nil {
				if previousErr == nil {
					err = errors.Join(err, a.setBrowserShortcuts(
						previous.BrowserSwitcherShortcut,
						previous.BrowserSwitcherBackwardShortcut,
					))
				}
				return false, err
			}
		}
		var err error
		pref, err = a.app.PreferencesService.Save(context.Background(), input)
		if err != nil && shortcutChanged && previousErr == nil && a.setBrowserShortcuts != nil {
			err = errors.Join(err, a.setBrowserShortcuts(
				previous.BrowserSwitcherShortcut,
				previous.BrowserSwitcherBackwardShortcut,
			))
		}
		return previousErr != nil || browserRoutingPreferencesChanged(previous, input), err
	})
	if saveErr != nil {
		if errors.Is(saveErr, errPreferencesChanged) {
			return preferencesdomain.UserPreference{}, saveErr
		}
		return preferencesdomain.UserPreference{}, a.storageError("The preference could not be saved", saveErr)
	}
	a.notifyPreferencesSaved(pref)
	return pref, nil
}

func (a *AppBindings) notifyPreferencesSaved(pref preferencesdomain.UserPreference) {
	if a.preferencesSaved != nil {
		a.preferencesSaved(pref)
	}
}

func (a *AppBindings) SaveBrowserAppearance(appearanceKey string, input preferencesdomain.BrowserAppearance) (preferencesdomain.UserPreference, error) {
	appearanceKey = strings.TrimSpace(appearanceKey)
	input.Icon = strings.TrimSpace(input.Icon)
	input.PrimaryColor = strings.ToUpper(strings.TrimSpace(input.PrimaryColor))
	if a.saveBrowserAppearance != nil {
		return a.saveBrowserAppearance(appearanceKey, input)
	}

	ctx := context.Background()
	var pref preferencesdomain.UserPreference
	var loadErr error
	saveErr := a.withPreferenceUpdate(func() (bool, error) {
		pref, loadErr = a.app.PreferencesService.Load(ctx)
		if loadErr != nil {
			return false, loadErr
		}
		nextAppearances := make(map[string]preferencesdomain.BrowserAppearance, len(pref.BrowserAppearances)+1)
		for key, appearance := range pref.BrowserAppearances {
			nextAppearances[key] = appearance
		}
		if input.Icon == "" && input.PrimaryColor == "" {
			delete(nextAppearances, appearanceKey)
		} else {
			nextAppearances[appearanceKey] = input
		}
		pref.BrowserAppearances = nextAppearances
		var err error
		pref, err = a.app.PreferencesService.Save(ctx, pref)
		return false, err
	})
	if saveErr != nil {
		if loadErr != nil {
			return preferencesdomain.UserPreference{}, a.storageError("Browser appearance could not be loaded", loadErr)
		}
		return preferencesdomain.UserPreference{}, a.storageError("The preference could not be saved", saveErr)
	}
	return pref, nil
}

func (a *AppBindings) RegisterBrowserShortcuts() error {
	if a.setBrowserShortcuts == nil {
		return nil
	}
	pref, err := a.app.PreferencesService.Load(context.Background())
	if err != nil {
		return err
	}
	return a.setBrowserShortcuts(pref.BrowserSwitcherShortcut, pref.BrowserSwitcherBackwardShortcut)
}
