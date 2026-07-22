package bindings

import (
	"context"

	preferencesdomain "ssh-man/internal/domain/preferences"
	"ssh-man/internal/keyboardshortcut"
)

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
	if err := input.Validate(); err != nil {
		return preferencesdomain.UserPreference{}, err
	}

	previous, previousErr := a.app.PreferencesService.Load(context.Background())
	shortcutChanged := previousErr != nil ||
		previous.BrowserSwitcherShortcut != forward ||
		previous.BrowserSwitcherBackwardShortcut != backward
	if shortcutChanged && a.setBrowserShortcuts != nil {
		if err := a.setBrowserShortcuts(forward, backward); err != nil {
			if previousErr == nil {
				_ = a.setBrowserShortcuts(previous.BrowserSwitcherShortcut, previous.BrowserSwitcherBackwardShortcut)
			}
			return preferencesdomain.UserPreference{}, err
		}
	}

	pref, err := a.app.PreferencesService.Save(context.Background(), input)
	if err != nil {
		if shortcutChanged && previousErr == nil && a.setBrowserShortcuts != nil {
			_ = a.setBrowserShortcuts(previous.BrowserSwitcherShortcut, previous.BrowserSwitcherBackwardShortcut)
		}
		return preferencesdomain.UserPreference{}, a.storageError("The preference could not be saved", err)
	}
	return pref, nil
}

func (a *AppBindings) SaveBrowserAppearance(appearanceKey string, input preferencesdomain.BrowserAppearance) (preferencesdomain.UserPreference, error) {
	ctx := context.Background()
	pref, err := a.app.PreferencesService.Load(ctx)
	if err != nil {
		return preferencesdomain.UserPreference{}, a.storageError("Browser appearance could not be loaded", err)
	}
	nextAppearances := make(map[string]preferencesdomain.BrowserAppearance, len(pref.BrowserAppearances)+1)
	for key, appearance := range pref.BrowserAppearances {
		nextAppearances[key] = appearance
	}
	nextAppearances[appearanceKey] = input
	pref.BrowserAppearances = nextAppearances

	saved, err := a.app.PreferencesService.Save(ctx, pref)
	if err != nil {
		return preferencesdomain.UserPreference{}, a.storageError("Browser appearance could not be saved", err)
	}
	return saved, nil
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
