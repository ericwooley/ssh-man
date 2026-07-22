package bindings

import (
	"context"
	"errors"
	"testing"

	"ssh-man/internal/app/bootstrap"
	preferencesdomain "ssh-man/internal/domain/preferences"
)

type preferenceMemoryStore struct {
	pref preferencesdomain.UserPreference
}

func (s *preferenceMemoryStore) Load(context.Context) (preferencesdomain.UserPreference, error) {
	return s.pref, nil
}

func (s *preferenceMemoryStore) Save(_ context.Context, pref preferencesdomain.UserPreference) error {
	s.pref = pref
	return nil
}

func TestSavePreferencesUpdatesGlobalShortcutsBeforePersisting(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)
	registered := [][2]string{}
	bindings.SetBrowserShortcutsRegistrar(func(forward, backward string) error {
		registered = append(registered, [2]string{forward, backward})
		return nil
	})

	input := store.pref
	input.BrowserSwitcherShortcut = "control+alt+b"
	input.BrowserSwitcherBackwardShortcut = "control+option+shift+y"
	saved, err := bindings.SavePreferences(input)
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if saved.BrowserSwitcherShortcut != "Ctrl+Alt+B" {
		t.Fatalf("saved forward shortcut = %q", saved.BrowserSwitcherShortcut)
	}
	if saved.BrowserSwitcherBackwardShortcut != "Ctrl+Alt+Shift+Y" {
		t.Fatalf("saved backward shortcut = %q", saved.BrowserSwitcherBackwardShortcut)
	}
	if len(registered) != 1 || registered[0] != [2]string{"Ctrl+Alt+B", "Ctrl+Alt+Shift+Y"} {
		t.Fatalf("registered shortcuts = %#v", registered)
	}
}

func TestSavePreferencesRestoresPreviousShortcutsWhenRegistrationFails(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)
	registered := [][2]string{}
	bindings.SetBrowserShortcutsRegistrar(func(forward, backward string) error {
		registered = append(registered, [2]string{forward, backward})
		if forward == "Alt+B" {
			return errors.New("shortcut already in use")
		}
		return nil
	})

	input := store.pref
	input.BrowserSwitcherShortcut = "Alt+B"
	if _, err := bindings.SavePreferences(input); err == nil {
		t.Fatal("expected shortcut registration error")
	}
	want := [][2]string{{"Alt+B", "Alt+Z"}, {"Alt+X", "Alt+Z"}}
	if len(registered) != len(want) || registered[0] != want[0] || registered[1] != want[1] {
		t.Fatalf("registered shortcuts = %#v", registered)
	}
	if store.pref.BrowserSwitcherShortcut != "Alt+X" || store.pref.BrowserSwitcherBackwardShortcut != "Alt+Z" {
		t.Fatalf("persisted shortcuts changed to %#v", store.pref)
	}
}

func TestSavePreferencesRejectsDuplicateBrowserShortcutsBeforeRegistration(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)
	registrationCalls := 0
	bindings.SetBrowserShortcutsRegistrar(func(string, string) error {
		registrationCalls++
		return nil
	})

	input := store.pref
	input.BrowserSwitcherShortcut = "option+x"
	input.BrowserSwitcherBackwardShortcut = "Alt+X"
	if _, err := bindings.SavePreferences(input); err == nil {
		t.Fatal("expected duplicate shortcut error")
	}
	if registrationCalls != 0 {
		t.Fatalf("registration calls = %d, want 0", registrationCalls)
	}
}

func TestSaveBrowserAppearancePersistsAndResetsWithoutReregisteringShortcuts(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)
	registrationCalls := 0
	bindings.SetBrowserShortcutsRegistrar(func(string, string) error {
		registrationCalls++
		return nil
	})

	key := "proxy:server-1:google-chrome"
	saved, err := bindings.SaveBrowserAppearance(" "+key+" ", preferencesdomain.BrowserAppearance{
		Icon:         " icon:x ",
		PrimaryColor: "#22c55e",
	})
	if err != nil {
		t.Fatalf("save browser appearance: %v", err)
	}
	if got := saved.BrowserAppearances[key]; got.Icon != "icon:x" || got.PrimaryColor != "#22C55E" {
		t.Fatalf("saved browser appearance = %#v", got)
	}
	if registrationCalls != 0 {
		t.Fatalf("shortcut registration calls = %d, want 0", registrationCalls)
	}

	saved, err = bindings.SaveBrowserAppearance(key, preferencesdomain.BrowserAppearance{})
	if err != nil {
		t.Fatalf("reset browser appearance: %v", err)
	}
	if _, exists := saved.BrowserAppearances[key]; exists {
		t.Fatalf("browser appearance was not reset: %#v", saved.BrowserAppearances)
	}
}

func TestSaveBrowserAppearanceRejectsInvalidInputWithoutChangingPreferences(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)

	_, err := bindings.SaveBrowserAppearance("regular:google-chrome", preferencesdomain.BrowserAppearance{Icon: "icon:unknown"})
	if err == nil {
		t.Fatal("expected invalid browser appearance error")
	}
	if len(store.pref.BrowserAppearances) != 0 {
		t.Fatalf("preferences changed after invalid appearance: %#v", store.pref.BrowserAppearances)
	}
}
