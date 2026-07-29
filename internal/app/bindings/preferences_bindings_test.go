package bindings

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ssh-man/internal/app/bootstrap"
	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	urlroutingdomain "ssh-man/internal/domain/urlrouting"
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

type initiallyEmptyPreferenceStore struct {
	persisted *preferencesdomain.UserPreference
}

func (s *initiallyEmptyPreferenceStore) Load(context.Context) (preferencesdomain.UserPreference, error) {
	if s.persisted == nil {
		return preferencesdomain.Default(), nil
	}
	return *s.persisted, nil
}

func (s *initiallyEmptyPreferenceStore) Save(_ context.Context, pref preferencesdomain.UserPreference) error {
	s.persisted = &pref
	return nil
}

type failingShortcutPreferenceStore struct {
	mu               sync.Mutex
	pref             preferencesdomain.UserPreference
	firstSaveStarted chan struct{}
	releaseFirstSave chan struct{}
}

func newFailingShortcutPreferenceStore(pref preferencesdomain.UserPreference) *failingShortcutPreferenceStore {
	return &failingShortcutPreferenceStore{
		pref:             pref,
		firstSaveStarted: make(chan struct{}),
		releaseFirstSave: make(chan struct{}),
	}
}

func (s *failingShortcutPreferenceStore) Load(context.Context) (preferencesdomain.UserPreference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pref, nil
}

func (s *failingShortcutPreferenceStore) Save(_ context.Context, pref preferencesdomain.UserPreference) error {
	if pref.BrowserSwitcherShortcut == "Alt+B" {
		close(s.firstSaveStarted)
		<-s.releaseFirstSave
		return errors.New("first preference save failed")
	}
	s.mu.Lock()
	s.pref = pref
	s.mu.Unlock()
	return nil
}

func (s *failingShortcutPreferenceStore) current() preferencesdomain.UserPreference {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pref
}

type blockingAppearancePreferenceStore struct {
	mu                    sync.Mutex
	pref                  preferencesdomain.UserPreference
	appearanceSaveStarted chan struct{}
	releaseAppearanceSave chan struct{}
	blockAppearanceOnce   sync.Once
}

func newBlockingAppearancePreferenceStore(pref preferencesdomain.UserPreference) *blockingAppearancePreferenceStore {
	return &blockingAppearancePreferenceStore{
		pref:                  pref,
		appearanceSaveStarted: make(chan struct{}),
		releaseAppearanceSave: make(chan struct{}),
	}
}

func (s *blockingAppearancePreferenceStore) Load(context.Context) (preferencesdomain.UserPreference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pref, nil
}

func (s *blockingAppearancePreferenceStore) Save(_ context.Context, pref preferencesdomain.UserPreference) error {
	blocked := false
	if len(pref.BrowserAppearances) > 0 {
		s.blockAppearanceOnce.Do(func() {
			blocked = true
			close(s.appearanceSaveStarted)
		})
	}
	if blocked {
		<-s.releaseAppearanceSave
	}
	s.mu.Lock()
	s.pref = pref
	s.mu.Unlock()
	return nil
}

func (s *blockingAppearancePreferenceStore) current() preferencesdomain.UserPreference {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pref
}

type preferenceRoutingConfigurations struct{}

func (preferenceRoutingConfigurations) ListAll(context.Context) ([]configdomain.ConnectionConfiguration, error) {
	return nil, nil
}

type preferenceRoutingServers struct{}

func (preferenceRoutingServers) List(context.Context) ([]serverdomain.Server, error) {
	return nil, nil
}

type preferenceRoutingRuntimes struct{}

func (preferenceRoutingRuntimes) List() []sessiondomain.RuntimeSession {
	return nil
}

type preferenceRoutingBrowsers struct{}

func (preferenceRoutingBrowsers) ListDestinations(context.Context) ([]urlroutingdomain.BrowserDestination, error) {
	return []urlroutingdomain.BrowserDestination{{
		ID:      "custom-old",
		Name:    "Old custom browser",
		Command: `open -a "Old Browser" "<URL>"`,
	}}, nil
}

func (preferenceRoutingBrowsers) OpenURL(context.Context, string, string) error {
	return nil
}

func (preferenceRoutingBrowsers) LaunchThroughSOCKSURL(context.Context, string, string, string) error {
	return nil
}

func TestValidateURLRulePatternUsesGoRegexDialect(t *testing.T) {
	var bindings AppBindings
	if result := bindings.ValidateURLRulePattern("regex", "(?=work)"); result.Valid {
		t.Fatal("ValidateURLRulePattern() valid = true, want false for unsupported lookahead")
	}
	if result := bindings.ValidateURLRulePattern("regex", "(?P<name>work)"); !result.Valid {
		t.Fatalf("ValidateURLRulePattern() Go named group = %#v, want valid", result)
	}
	if result := bindings.ValidateURLRulePattern("contains", "(?=work)"); !result.Valid {
		t.Fatalf("ValidateURLRulePattern() literal = %#v, want valid", result)
	}
}

func TestBrowserRoutingPreferencesChangedTracksRoutingFieldsOnly(t *testing.T) {
	tests := []struct {
		name   string
		change func(*preferencesdomain.UserPreference)
		want   bool
	}{
		{name: "default browser", change: func(pref *preferencesdomain.UserPreference) { pref.DefaultBrowserID = "safari" }, want: true},
		{name: "proxy browser", change: func(pref *preferencesdomain.UserPreference) { pref.ProxyBrowserID = "chrome" }, want: true},
		{name: "disabled browsers", change: func(pref *preferencesdomain.UserPreference) { pref.DisabledBrowserIDs = []string{"chrome"} }, want: true},
		{name: "custom browsers", change: func(pref *preferencesdomain.UserPreference) {
			pref.CustomBrowsers = []preferencesdomain.CustomBrowser{{ID: "work", DisplayName: "Work", Command: `open "<URL>"`}}
		}, want: true},
		{name: "URL rules", change: func(pref *preferencesdomain.UserPreference) {
			pref.URLRules = []preferencesdomain.URLRule{{ID: "work", Pattern: "work", Action: preferencesdomain.URLRuleActionBrowser, BrowserID: "safari"}}
		}, want: true},
		{name: "port assignments", change: func(pref *preferencesdomain.UserPreference) {
			pref.URLPortAssignments = []preferencesdomain.URLPortAssignment{{ID: "port", Port: 3000, ServerID: "server", BrowserID: "chrome"}}
		}, want: true},
		{name: "theme", change: func(pref *preferencesdomain.UserPreference) { pref.Theme = preferencesdomain.ThemeLight }, want: false},
		{name: "appearance", change: func(pref *preferencesdomain.UserPreference) {
			pref.BrowserAppearances = map[string]preferencesdomain.BrowserAppearance{"regular:safari": {Icon: "icon:star"}}
		}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := preferencesdomain.Default()
			next := previous
			test.change(&next)
			if got := browserRoutingPreferencesChanged(previous, next); got != test.want {
				t.Fatalf("browserRoutingPreferencesChanged() = %v, want %v", got, test.want)
			}
		})
	}
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

func TestSavePreferencesAcceptsFirstSaveFromEmptyPreferenceStore(t *testing.T) {
	store := &initiallyEmptyPreferenceStore{}
	preferenceService := preferencesdomain.NewService(store)
	app := &bootstrap.Application{PreferencesService: preferenceService}
	bindings := NewAppBindingsWithApplication(app, nil)

	input, err := preferenceService.Load(context.Background())
	if err != nil {
		t.Fatalf("load initial preferences: %v", err)
	}
	input.Theme = preferencesdomain.ThemeLight

	saved, err := bindings.SavePreferences(input)
	if err != nil {
		t.Fatalf("first preference save: %v", err)
	}
	if saved.Theme != preferencesdomain.ThemeLight {
		t.Fatalf("saved theme = %q, want light", saved.Theme)
	}
	if saved.UpdatedAt.IsZero() {
		t.Fatal("saved preferences did not receive a persisted revision")
	}
}

func TestSavePreferencesInvalidatesPendingRouteForBrowserConfigurationChange(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	preferenceService := preferencesdomain.NewService(store)
	routingService := urlroutingdomain.NewService(
		preferenceService,
		preferenceRoutingConfigurations{},
		preferenceRoutingServers{},
		preferenceRoutingRuntimes{},
		preferenceRoutingBrowsers{},
	)
	app := &bootstrap.Application{
		PreferencesService: preferenceService,
		URLRoutingService:  routingService,
	}
	bindings := NewAppBindingsWithApplication(app, nil)

	result, err := routingService.Handle(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if result.Request == nil {
		t.Fatal("expected pending URL route")
	}

	input := store.pref
	input.DisabledBrowserIDs = []string{"custom-old"}
	if _, err := bindings.SavePreferences(input); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if _, ok := routingService.Pending(); ok {
		t.Fatal("pending URL route survived browser preference save")
	}
	if err := routingService.ResolveChoice(context.Background(), result.Request.ID, result.Request.DefaultChoiceID); err == nil {
		t.Fatal("expected stale route resolution to fail")
	}
}

func TestSavePreferencesKeepsPendingRouteForAppearanceOnlyChange(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	preferenceService := preferencesdomain.NewService(store)
	routingService := urlroutingdomain.NewService(
		preferenceService,
		preferenceRoutingConfigurations{},
		preferenceRoutingServers{},
		preferenceRoutingRuntimes{},
		preferenceRoutingBrowsers{},
	)
	app := &bootstrap.Application{
		PreferencesService: preferenceService,
		URLRoutingService:  routingService,
	}
	bindings := NewAppBindingsWithApplication(app, nil)

	result, err := routingService.Handle(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	input := store.pref
	input.BrowserAppearances = map[string]preferencesdomain.BrowserAppearance{
		"regular:custom-old": {Icon: "icon:star"},
	}
	if _, err := bindings.SavePreferences(input); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if pending, ok := routingService.Pending(); !ok || pending.ID != result.Request.ID {
		t.Fatalf("appearance-only save cleared pending route: pending=%#v ok=%v", pending, ok)
	}
}

func TestSaveBrowserAppearanceSerializesWithRoutingSaveAndRejectsStaleRoutingSnapshot(t *testing.T) {
	store := newBlockingAppearancePreferenceStore(preferencesdomain.Default())
	preferenceService := preferencesdomain.NewService(store)
	routingService := urlroutingdomain.NewService(
		preferenceService,
		preferenceRoutingConfigurations{},
		preferenceRoutingServers{},
		preferenceRoutingRuntimes{},
		preferenceRoutingBrowsers{},
	)
	app := &bootstrap.Application{
		PreferencesService: preferenceService,
		URLRoutingService:  routingService,
	}
	bindings := NewAppBindingsWithApplication(app, nil)

	appearanceDone := make(chan error, 1)
	go func() {
		_, err := bindings.SaveBrowserAppearance(
			"regular:custom-old",
			preferencesdomain.BrowserAppearance{Icon: "icon:star"},
		)
		appearanceDone <- err
	}()
	<-store.appearanceSaveStarted

	routingInput := preferencesdomain.Default()
	routingInput.DefaultBrowserID = "custom-old"
	routingDone := make(chan error, 1)
	go func() {
		_, err := bindings.SavePreferences(routingInput)
		routingDone <- err
	}()

	select {
	case err := <-routingDone:
		close(store.releaseAppearanceSave)
		if err := <-appearanceDone; err != nil {
			t.Fatalf("appearance save: %v", err)
		}
		t.Fatalf("routing save completed before serialized appearance save: %v", err)
	case <-time.After(250 * time.Millisecond):
		close(store.releaseAppearanceSave)
		if err := <-appearanceDone; err != nil {
			t.Fatalf("appearance save: %v", err)
		}
		if err := <-routingDone; err == nil {
			t.Fatal("expected routing save with stale preference snapshot to fail")
		}
		current := store.current()
		if got := current.DefaultBrowserID; got != "" {
			t.Fatalf("default browser after rejected stale save = %q, want unchanged", got)
		}
		if got := current.BrowserAppearances["regular:custom-old"].Icon; got != "icon:star" {
			t.Fatalf("browser appearance after rejected stale save = %q, want icon:star", got)
		}
	}
}

func TestSavePreferencesRejectsStaleSnapshotWithoutErasingBrowserAppearance(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	preferenceService := preferencesdomain.NewService(store)
	app := &bootstrap.Application{PreferencesService: preferenceService}
	bindings := NewAppBindingsWithApplication(app, nil)

	staleRoutingInput := store.pref
	staleRoutingInput.DefaultBrowserID = "custom-old"
	if _, err := bindings.SaveBrowserAppearance(
		"regular:custom-old",
		preferencesdomain.BrowserAppearance{Icon: "icon:star"},
	); err != nil {
		t.Fatalf("save browser appearance: %v", err)
	}

	if _, err := bindings.SavePreferences(staleRoutingInput); !errors.Is(err, errPreferencesChanged) {
		t.Fatalf("stale full preference snapshot error = %v, want preference conflict", err)
	}
	current := store.pref
	if got := current.BrowserAppearances["regular:custom-old"].Icon; got != "icon:star" {
		t.Fatalf("browser appearance after stale full save = %q, want icon:star", got)
	}
	if got := current.DefaultBrowserID; got != "" {
		t.Fatalf("default browser after rejected stale full save = %q, want unchanged", got)
	}
}

func TestSaveBrowserAppearanceAfterRoutingSavePreservesBothUpdates(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	preferenceService := preferencesdomain.NewService(store)
	app := &bootstrap.Application{PreferencesService: preferenceService}
	bindings := NewAppBindingsWithApplication(app, nil)

	routingInput := store.pref
	routingInput.DefaultBrowserID = "custom-old"
	if _, err := bindings.SavePreferences(routingInput); err != nil {
		t.Fatalf("save routing preferences: %v", err)
	}
	if _, err := bindings.SaveBrowserAppearance(
		"regular:custom-old",
		preferencesdomain.BrowserAppearance{Icon: "icon:star"},
	); err != nil {
		t.Fatalf("save browser appearance: %v", err)
	}

	current := store.pref
	if got := current.DefaultBrowserID; got != "custom-old" {
		t.Fatalf("default browser after appearance save = %q, want custom-old", got)
	}
	if got := current.BrowserAppearances["regular:custom-old"].Icon; got != "icon:star" {
		t.Fatalf("browser appearance after routing save = %q, want icon:star", got)
	}
}

func TestSaveBrowserAppearanceKeepsPendingRoute(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	preferenceService := preferencesdomain.NewService(store)
	routingService := urlroutingdomain.NewService(
		preferenceService,
		preferenceRoutingConfigurations{},
		preferenceRoutingServers{},
		preferenceRoutingRuntimes{},
		preferenceRoutingBrowsers{},
	)
	app := &bootstrap.Application{
		PreferencesService: preferenceService,
		URLRoutingService:  routingService,
	}
	bindings := NewAppBindingsWithApplication(app, nil)

	result, err := routingService.Handle(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if _, err := bindings.SaveBrowserAppearance(
		"regular:custom-old",
		preferencesdomain.BrowserAppearance{Icon: "icon:star"},
	); err != nil {
		t.Fatalf("save browser appearance: %v", err)
	}
	if pending, ok := routingService.Pending(); !ok || pending.ID != result.Request.ID {
		t.Fatalf("appearance save cleared pending route: pending=%#v ok=%v", pending, ok)
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

func TestSavePreferencesSerializesFailedShortcutRollbackWithNextSave(t *testing.T) {
	initial := preferencesdomain.Default()
	store := newFailingShortcutPreferenceStore(initial)
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	routingService := urlroutingdomain.NewService(
		app.PreferencesService,
		preferenceRoutingConfigurations{},
		preferenceRoutingServers{},
		preferenceRoutingRuntimes{},
		preferenceRoutingBrowsers{},
	)
	app.URLRoutingService = routingService
	bindings := NewAppBindingsWithApplication(app, nil)

	var activeMu sync.Mutex
	activeShortcut := initial.BrowserSwitcherShortcut
	secondRegistered := make(chan struct{})
	var secondRegisteredOnce sync.Once
	bindings.SetBrowserShortcutsRegistrar(func(forward, _ string) error {
		if forward == initial.BrowserSwitcherShortcut {
			select {
			case <-secondRegistered:
			case <-time.After(100 * time.Millisecond):
			}
		}
		activeMu.Lock()
		activeShortcut = forward
		activeMu.Unlock()
		if forward == "Alt+C" {
			secondRegisteredOnce.Do(func() { close(secondRegistered) })
		}
		return nil
	})

	firstInput := initial
	firstInput.BrowserSwitcherShortcut = "Alt+B"
	firstDone := make(chan error, 1)
	go func() {
		_, err := bindings.SavePreferences(firstInput)
		firstDone <- err
	}()
	<-store.firstSaveStarted

	secondInput := initial
	secondInput.BrowserSwitcherShortcut = "Alt+C"
	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := bindings.SavePreferences(secondInput)
		secondDone <- err
	}()
	<-secondStarted
	close(store.releaseFirstSave)

	if err := <-firstDone; err == nil {
		t.Fatal("expected first preference save to fail")
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second preference save: %v", err)
	}
	activeMu.Lock()
	gotActive := activeShortcut
	activeMu.Unlock()
	if gotActive != "Alt+C" {
		t.Fatalf("active shortcut after successful second save = %q, want Alt+C", gotActive)
	}
	if got := store.current().BrowserSwitcherShortcut; got != "Alt+C" {
		t.Fatalf("persisted shortcut after successful second save = %q, want Alt+C", got)
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

func TestSavePreferencesUsesOwnerSaverAfterNormalization(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)
	var forwarded preferencesdomain.UserPreference
	bindings.SetPreferencesSaver(func(input preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error) {
		forwarded = input
		return input, nil
	})

	input := store.pref
	input.BrowserSwitcherShortcut = "option+b"
	saved, err := bindings.SavePreferences(input)
	if err != nil {
		t.Fatal(err)
	}
	if forwarded.BrowserSwitcherShortcut != "Alt+B" || saved.BrowserSwitcherShortcut != "Alt+B" {
		t.Fatalf("forwarded shortcut = %q, saved shortcut = %q", forwarded.BrowserSwitcherShortcut, saved.BrowserSwitcherShortcut)
	}
	if store.pref.BrowserSwitcherShortcut != "Alt+X" {
		t.Fatalf("companion store changed locally to %q", store.pref.BrowserSwitcherShortcut)
	}
}

func TestSavePreferencesAllowsUnchangedLegacyCommandDuringUnrelatedSave(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.Theme = preferencesdomain.ThemeDark
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:        "legacy-rule",
		MatchMode: preferencesdomain.URLRuleMatchContains,
		Pattern:   "example.com",
		Action:    preferencesdomain.URLRuleActionCommand,
		Command:   `/bin/zsh -lc "open <URL>"`,
	}}
	store := &preferenceMemoryStore{pref: pref}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)

	input := pref
	input.Theme = preferencesdomain.ThemeLight
	saved, err := bindings.SavePreferences(input)
	if err != nil {
		t.Fatalf("save unrelated theme change: %v", err)
	}
	if saved.Theme != preferencesdomain.ThemeLight ||
		len(saved.URLRules) != 1 ||
		saved.URLRules[0].Command != pref.URLRules[0].Command {
		t.Fatalf("saved preferences = %#v, want unchanged legacy rule", saved)
	}
}

func TestSaveBrowserAppearanceUsesOwnerSaverAfterNormalization(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)
	var forwardedKey string
	var forwardedAppearance preferencesdomain.BrowserAppearance
	bindings.SetBrowserAppearanceSaver(func(key string, appearance preferencesdomain.BrowserAppearance) (preferencesdomain.UserPreference, error) {
		forwardedKey = key
		forwardedAppearance = appearance
		return store.pref, nil
	})

	if _, err := bindings.SaveBrowserAppearance(
		" regular:google-chrome ",
		preferencesdomain.BrowserAppearance{Icon: " icon:star ", PrimaryColor: "#22c55e"},
	); err != nil {
		t.Fatal(err)
	}
	if forwardedKey != "regular:google-chrome" {
		t.Fatalf("forwarded appearance key = %q", forwardedKey)
	}
	if forwardedAppearance.Icon != "icon:star" || forwardedAppearance.PrimaryColor != "#22C55E" {
		t.Fatalf("forwarded appearance = %#v", forwardedAppearance)
	}
	if len(store.pref.BrowserAppearances) != 0 {
		t.Fatalf("companion store changed locally: %#v", store.pref.BrowserAppearances)
	}
}

func TestSavePreferencesNotifiesObserverAfterSuccessfulSave(t *testing.T) {
	store := &preferenceMemoryStore{pref: preferencesdomain.Default()}
	app := &bootstrap.Application{PreferencesService: preferencesdomain.NewService(store)}
	bindings := NewAppBindingsWithApplication(app, nil)
	var observed []bool
	bindings.SetPreferencesSavedObserver(func(pref preferencesdomain.UserPreference) {
		observed = append(observed, pref.AutomaticUpdates)
	})

	input := store.pref
	input.AutomaticUpdates = false
	if _, err := bindings.SavePreferences(input); err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	if len(observed) != 1 || observed[0] {
		t.Fatalf("observed automatic update preferences = %#v", observed)
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
