package urlrouting

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

type fakePreferences struct {
	value preferencesdomain.UserPreference
}

func (f fakePreferences) Load(context.Context) (preferencesdomain.UserPreference, error) {
	return f.value, nil
}

type blockingPreferences struct {
	value   preferencesdomain.UserPreference
	loaded  chan struct{}
	release chan struct{}
}

func (f blockingPreferences) Load(context.Context) (preferencesdomain.UserPreference, error) {
	close(f.loaded)
	<-f.release
	return f.value, nil
}

type fakeConfigurations struct {
	items []configdomain.ConnectionConfiguration
}

func (f fakeConfigurations) ListAll(context.Context) ([]configdomain.ConnectionConfiguration, error) {
	return f.items, nil
}

type fakeServers struct {
	items []serverdomain.Server
}

func (f fakeServers) List(context.Context) ([]serverdomain.Server, error) {
	return f.items, nil
}

type fakeRuntimes struct {
	items []sessiondomain.RuntimeSession
}

func (f fakeRuntimes) List() []sessiondomain.RuntimeSession {
	return f.items
}

type browserCall struct {
	configurationID string
	browserID       string
	url             string
}

type fakeBrowsers struct {
	destinations []BrowserDestination
	regular      []browserCall
	proxy        []browserCall
}

func (f *fakeBrowsers) ListDestinations(context.Context) ([]BrowserDestination, error) {
	return append([]BrowserDestination(nil), f.destinations...), nil
}

func (f *fakeBrowsers) OpenURL(_ context.Context, browserID, rawURL string) error {
	f.regular = append(f.regular, browserCall{browserID: browserID, url: rawURL})
	return nil
}

func (f *fakeBrowsers) LaunchThroughSOCKSURL(_ context.Context, configurationID, browserID, rawURL string) error {
	f.proxy = append(f.proxy, browserCall{configurationID: configurationID, browserID: browserID, url: rawURL})
	return nil
}

func TestHandlePresentsMatchingRuleInsteadOfOpeningImmediately(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.URLRules = []preferencesdomain.URLRule{
		{ID: "work", Pattern: `^https://github\.com/workorg/`, Action: preferencesdomain.URLRuleActionBrowser, BrowserID: "brave-browser"},
		{ID: "later", Pattern: `github`, Action: preferencesdomain.URLRuleActionBrowser, BrowserID: "firefox"},
	}
	browsers := routingBrowsers()
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, browsers)
	service.probe = func(context.Context, int, string, int) error {
		t.Fatal("a URL without an explicit port must not be probed")
		return nil
	}

	result, err := service.Handle(context.Background(), "https://github.com/workorg/repo")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Kind != ResultNeedsChoice || result.Request == nil {
		t.Fatalf("result = %#v, want choice", result)
	}
	if result.Request.DefaultChoiceID != "browser:brave-browser" || result.Request.TimeoutMilliseconds != 5000 {
		t.Fatalf("request = %#v", result.Request)
	}
	if len(browsers.regular) != 0 || len(browsers.proxy) != 0 {
		t.Fatalf("URL opened before choice: regular=%#v proxy=%#v", browsers.regular, browsers.proxy)
	}

	if err := service.ResolveChoice(context.Background(), result.Request.ID, result.Request.DefaultChoiceID); err != nil {
		t.Fatalf("resolve default: %v", err)
	}
	want := []browserCall{{browserID: "brave-browser", url: "https://github.com/workorg/repo"}}
	if !reflect.DeepEqual(browsers.regular, want) {
		t.Fatalf("regular calls = %#v, want %#v", browsers.regular, want)
	}
}

func TestHandlePresentsMatchingCommandAsDefault(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:      "work-container",
		Pattern: `^https://github\.com/workorg/`,
		Action:  preferencesdomain.URLRuleActionCommand,
		Command: `open -a "Zen" "ext+container:name=Work&url=<URL>"`,
	}}
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, routingBrowsers())
	var gotTemplate, gotURL string
	service.runCommand = func(template, rawURL string) error {
		gotTemplate, gotURL = template, rawURL
		return nil
	}

	result, err := service.Handle(context.Background(), "https://github.com/workorg/repo?q=a&b=c")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Kind != ResultNeedsChoice || result.Request == nil || result.Request.DefaultChoiceID != "command:work-container" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if gotTemplate != "" || gotURL != "" {
		t.Fatal("command executed before the chooser resolved")
	}
	if err := service.ResolveChoice(context.Background(), result.Request.ID, result.Request.DefaultChoiceID); err != nil {
		t.Fatalf("resolve command: %v", err)
	}
	if gotTemplate != pref.URLRules[0].Command || gotURL != "https://github.com/workorg/repo?q=a&b=c" {
		t.Fatalf("template=%q url=%q", gotTemplate, gotURL)
	}
}

func TestHandleExcludesLegacyInvalidCommandRuleUntilRepaired(t *testing.T) {
	for _, openDirect := range []bool{false, true} {
		t.Run(fmt.Sprintf("open_direct_%t", openDirect), func(t *testing.T) {
			pref := preferencesdomain.Default()
			pref.DefaultBrowserID = "brave-browser"
			pref.URLRules = []preferencesdomain.URLRule{{
				ID:         "legacy-shell",
				MatchMode:  preferencesdomain.URLRuleMatchContains,
				Pattern:    "github.com/workorg",
				Action:     preferencesdomain.URLRuleActionCommand,
				Command:    `/bin/zsh -lc "open <URL>"`,
				OpenDirect: openDirect,
			}}
			service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, routingBrowsers())
			commandRuns := 0
			service.runCommand = func(string, string) error {
				commandRuns++
				return nil
			}

			result, err := service.Handle(context.Background(), "https://github.com/workorg/repo")
			if err != nil {
				t.Fatalf("handle URL: %v", err)
			}
			if result.Kind != ResultNeedsChoice || result.Request == nil {
				t.Fatalf("result = %#v, want regular browser chooser", result)
			}
			if result.Request.DefaultChoiceID != "browser:brave-browser" {
				t.Fatalf("default choice = %q, want browser:brave-browser", result.Request.DefaultChoiceID)
			}
			if choiceIndex(result.Request.Choices, "command:legacy-shell") >= 0 {
				t.Fatalf("legacy command choice remained available: %#v", result.Request.Choices)
			}
			if commandRuns != 0 {
				t.Fatalf("legacy command runs = %d, want 0", commandRuns)
			}
		})
	}
}

func TestWithPendingInvalidationPreventsCapturedCommandResolution(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:      "work-container",
		Pattern: "github.com/workorg",
		Action:  preferencesdomain.URLRuleActionCommand,
		Command: `open -a "Zen" "<URL>"`,
	}}
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, routingBrowsers())
	commandRuns := 0
	service.runCommand = func(string, string) error {
		commandRuns++
		return nil
	}

	result, err := service.Handle(context.Background(), "https://github.com/workorg/repo")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Request == nil {
		t.Fatal("expected pending URL route")
	}

	saveCalled := false
	if err := service.WithPendingInvalidation(func() error {
		saveCalled = true
		return nil
	}); err != nil {
		t.Fatalf("invalidate after save: %v", err)
	}
	if !saveCalled {
		t.Fatal("preference save callback was not called")
	}
	if err := service.ResolveChoice(context.Background(), result.Request.ID, result.Request.DefaultChoiceID); err == nil {
		t.Fatal("expected invalidated route resolution to fail")
	}
	if commandRuns != 0 {
		t.Fatalf("command runs = %d, want 0", commandRuns)
	}
}

func TestWithPendingInvalidationPreservesRouteWhenSaveFails(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:      "work-container",
		Pattern: "github.com/workorg",
		Action:  preferencesdomain.URLRuleActionCommand,
		Command: `open -a "Zen" "<URL>"`,
	}}
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, routingBrowsers())
	commandRuns := 0
	service.runCommand = func(string, string) error {
		commandRuns++
		return nil
	}

	result, err := service.Handle(context.Background(), "https://github.com/workorg/repo")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	saveErr := errors.New("save failed")
	if err := service.WithPendingInvalidation(func() error { return saveErr }); !errors.Is(err, saveErr) {
		t.Fatalf("save error = %v, want %v", err, saveErr)
	}
	if err := service.ResolveChoice(context.Background(), result.Request.ID, result.Request.DefaultChoiceID); err != nil {
		t.Fatalf("resolve preserved route: %v", err)
	}
	if commandRuns != 1 {
		t.Fatalf("command runs = %d, want 1", commandRuns)
	}
}

func TestWithPendingInvalidationSerializesRouteCreation(t *testing.T) {
	pref := preferencesdomain.Default()
	preferences := blockingPreferences{
		value:   pref,
		loaded:  make(chan struct{}),
		release: make(chan struct{}),
	}
	service := NewService(preferences, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, routingBrowsers())
	handleDone := make(chan error, 1)
	go func() {
		_, err := service.Handle(context.Background(), "https://example.com")
		handleDone <- err
	}()
	<-preferences.loaded

	saveStarted := make(chan struct{})
	invalidationDone := make(chan error, 1)
	go func() {
		invalidationDone <- service.WithPendingInvalidation(func() error {
			close(saveStarted)
			return nil
		})
	}()

	select {
	case <-saveStarted:
		t.Fatal("preference save started before in-flight route creation completed")
	case <-time.After(25 * time.Millisecond):
	}
	close(preferences.release)
	if err := <-handleDone; err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if err := <-invalidationDone; err != nil {
		t.Fatalf("invalidate pending route: %v", err)
	}
	if _, ok := service.Pending(); ok {
		t.Fatal("route created from the old preference snapshot survived the save")
	}
}

func TestHandleMatchesLiteralRuleModes(t *testing.T) {
	tests := []struct {
		name    string
		mode    preferencesdomain.URLRuleMatchMode
		pattern string
		rawURL  string
	}{
		{
			name:    "starts with",
			mode:    preferencesdomain.URLRuleMatchStartsWith,
			pattern: "https://github.com/workorg/",
			rawURL:  "https://github.com/workorg/repo",
		},
		{
			name:    "ends with",
			mode:    preferencesdomain.URLRuleMatchEndsWith,
			pattern: "/dashboard",
			rawURL:  "https://example.com/dashboard",
		},
		{
			name:    "contains",
			mode:    preferencesdomain.URLRuleMatchContains,
			pattern: "ticket=",
			rawURL:  "https://example.com/issues?ticket=123",
		},
		{
			name:    "regex",
			mode:    preferencesdomain.URLRuleMatchRegex,
			pattern: `^https://(docs|wiki)\.`,
			rawURL:  "https://docs.example.com/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pref := preferencesdomain.Default()
			pref.DefaultBrowserID = "safari"
			pref.URLRules = []preferencesdomain.URLRule{{
				ID:        "rule",
				MatchMode: test.mode,
				Pattern:   test.pattern,
				Action:    preferencesdomain.URLRuleActionBrowser,
				BrowserID: "firefox",
			}}
			service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, routingBrowsers())

			result, err := service.Handle(context.Background(), test.rawURL)
			if err != nil {
				t.Fatalf("handle url: %v", err)
			}
			if result.Request == nil || result.Request.DefaultChoiceID != "browser:firefox" {
				t.Fatalf("result = %#v, want Firefox default", result)
			}
		})
	}
}

func TestHandleOpensDirectBrowserRuleWithoutPresentingChooser(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:         "work",
		MatchMode:  preferencesdomain.URLRuleMatchStartsWith,
		Pattern:    "https://github.com/workorg/",
		Action:     preferencesdomain.URLRuleActionBrowser,
		BrowserID:  "brave-browser",
		OpenDirect: true,
	}}
	browsers := routingBrowsers()
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, browsers)
	presented := false
	service.SetPresenter(func(RouteRequest) { presented = true })

	rawURL := "https://github.com/workorg/repo"
	result, err := service.Handle(context.Background(), rawURL)
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if result.Kind != ResultOpenedDirectly || result.Request != nil {
		t.Fatalf("result = %#v, want direct open", result)
	}
	if presented {
		t.Fatal("direct rule presented a chooser")
	}
	if _, ok := service.Pending(); ok {
		t.Fatal("direct rule left a pending chooser")
	}
	want := []browserCall{{browserID: "brave-browser", url: rawURL}}
	if !reflect.DeepEqual(browsers.regular, want) {
		t.Fatalf("regular calls = %#v, want %#v", browsers.regular, want)
	}
}

func TestHandleFallsBackToChooserWhenDirectRuleBrowserIsUnavailable(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:         "removed-work-browser",
		MatchMode:  preferencesdomain.URLRuleMatchContains,
		Pattern:    "work.example",
		Action:     preferencesdomain.URLRuleActionBrowser,
		BrowserID:  "removed-browser",
		OpenDirect: true,
	}}
	browsers := routingBrowsers()
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, browsers)
	presented := false
	service.SetPresenter(func(RouteRequest) { presented = true })

	result, err := service.Handle(context.Background(), "https://work.example/ticket")
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if result.Kind != ResultNeedsChoice || result.Request == nil {
		t.Fatalf("result = %#v, want fallback chooser", result)
	}
	if !presented {
		t.Fatal("unavailable direct rule did not present the fallback chooser")
	}
	if len(result.Request.Choices) == 0 || !choiceExists(result.Request.Choices, result.Request.DefaultChoiceID) {
		t.Fatalf("fallback request = %#v, want an available default choice", result.Request)
	}
	if len(browsers.regular) != 0 {
		t.Fatalf("regular browser calls = %#v, want no browser opened before chooser resolution", browsers.regular)
	}
}

func TestHandleOpensDirectCustomBrowserWithURLAsArgvData(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:         "work",
		MatchMode:  preferencesdomain.URLRuleMatchContains,
		Pattern:    "example.com",
		Action:     preferencesdomain.URLRuleActionBrowser,
		BrowserID:  "custom-work",
		OpenDirect: true,
	}}
	browsers := routingBrowsers()
	browsers.destinations = append(browsers.destinations, BrowserDestination{
		ID:      "custom-work",
		Name:    "Work profile",
		Command: `open <URL>`,
	})
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, browsers)
	var gotArguments []string
	service.runCommand = func(template, rawURL string) error {
		var err error
		gotArguments, err = commandTemplateArguments(template, rawURL)
		return err
	}

	rawURL := "https://example.com/?q=$((1+1))&value=$(printf%20unsafe)"
	result, err := service.Handle(context.Background(), rawURL)
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if result.Kind != ResultOpenedDirectly || result.Request != nil {
		t.Fatalf("result = %#v, want direct open", result)
	}
	want := []string{"/usr/bin/open", rawURL}
	if !reflect.DeepEqual(gotArguments, want) {
		t.Fatalf("command arguments = %#v, want %#v", gotArguments, want)
	}
}

func TestHandleOpensCustomBrowserCommandAsBrowserChoice(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "custom-work"
	browsers := routingBrowsers()
	browsers.destinations = append(browsers.destinations, BrowserDestination{
		ID:      "custom-work",
		Name:    "Work profile",
		Command: `open -a "Zen" "ext+container:name=Work&url=<URL>"`,
		Icon:    "icon:briefcase",
	})
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, browsers)
	var gotTemplate, gotURL string
	service.runCommand = func(template, rawURL string) error {
		gotTemplate, gotURL = template, rawURL
		return nil
	}

	result, err := service.Handle(context.Background(), "https://example.com/")
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if result.Request == nil || result.Request.DefaultChoiceID != "browser:custom-work" {
		t.Fatalf("result = %#v, want custom browser default", result)
	}
	customChoice := result.Request.Choices[choiceIndex(result.Request.Choices, "browser:custom-work")]
	if customChoice.Kind != RouteChoiceCommand || customChoice.Icon != "icon:briefcase" {
		t.Fatalf("custom browser choice = %#v", customChoice)
	}
	if err := service.ResolveChoice(context.Background(), result.Request.ID, result.Request.DefaultChoiceID); err != nil {
		t.Fatalf("resolve custom browser: %v", err)
	}
	if gotTemplate != browsers.destinations[len(browsers.destinations)-1].Command || gotURL != "https://example.com/" {
		t.Fatalf("command = %q URL = %q", gotTemplate, gotURL)
	}
}

func TestHandleExcludesLegacyInvalidCustomBrowserUntilRepaired(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "custom-legacy"
	browsers := routingBrowsers()
	browsers.destinations = append(browsers.destinations, BrowserDestination{
		ID:      "custom-legacy",
		Name:    "Legacy profile",
		Command: `/bin/zsh -lc "open <URL>"`,
	})
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, browsers)

	result, err := service.Handle(context.Background(), "https://example.com/")
	if err != nil {
		t.Fatalf("handle URL: %v", err)
	}
	if result.Kind != ResultNeedsChoice || result.Request == nil {
		t.Fatalf("result = %#v, want regular browser chooser", result)
	}
	if result.Request.DefaultChoiceID == "browser:custom-legacy" ||
		choiceIndex(result.Request.Choices, "browser:custom-legacy") >= 0 {
		t.Fatalf("legacy custom browser remained available: %#v", result.Request)
	}
}

func TestHandleProbesEveryConnectedHostForAnyExplicitURLPort(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.ProxyBrowserID = "google-chrome"
	configs, servers, runtimes := routingFixtures()
	browsers := routingBrowsers()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		browsers,
	)
	probes := map[int]string{}
	var probesMu sync.Mutex
	service.probe = func(_ context.Context, socksPort int, host string, port int) error {
		if port != 8443 {
			t.Fatalf("probe port = %d, want 8443", port)
		}
		probesMu.Lock()
		probes[socksPort] = host
		probesMu.Unlock()
		if socksPort == 41001 {
			return nil
		}
		return errors.New("closed")
	}

	result, err := service.Handle(context.Background(), "https://service.internal:8443/dashboard")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if len(probes) != 2 || probes[41001] != "service.internal" || probes[41002] != "service.internal" {
		t.Fatalf("probes = %#v", probes)
	}
	if result.Request.DefaultChoiceID != "proxy:server-socks:bts:google-chrome" {
		t.Fatalf("default choice = %q", result.Request.DefaultChoiceID)
	}
	if !hasChoice(result.Request.Choices, "proxy:server-socks:bts:firefox") {
		t.Fatalf("reachable browser/host combinations missing: %#v", result.Request.Choices)
	}
}

func TestHandleKeepsUnmatchedProxyBrowsersAfterMatchingChoices(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.ProxyBrowserID = "google-chrome"
	configs, servers, runtimes := routingFixtures()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		routingBrowsers(),
	)
	service.probe = func(_ context.Context, socksPort int, _ string, _ int) error {
		if socksPort == 41001 {
			return nil
		}
		return errors.New("closed")
	}

	result, err := service.Handle(context.Background(), "http://localhost:3000/")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Request.DefaultChoiceID != "proxy:server-socks:bts:google-chrome" {
		t.Fatalf("default choice = %q", result.Request.DefaultChoiceID)
	}
	matchingIndex := choiceIndex(result.Request.Choices, "proxy:server-socks:bts:google-chrome")
	unmatchedIndex := choiceIndex(result.Request.Choices, "proxy:server-socks:staging:google-chrome")
	if matchingIndex < 0 || unmatchedIndex < 0 {
		t.Fatalf("choices = %#v, want matching and unmatched proxy browsers", result.Request.Choices)
	}
	if matchingIndex >= unmatchedIndex {
		t.Fatalf("matching choice index = %d, unmatched choice index = %d", matchingIndex, unmatchedIndex)
	}
}

func TestHandleUsesPortAssignmentForDefaultBrowserHostCombination(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.ProxyBrowserID = "google-chrome"
	pref.URLPortAssignments = []preferencesdomain.URLPortAssignment{{
		ID:        "docs",
		Port:      3000,
		ServerID:  "staging",
		BrowserID: "firefox",
	}}
	configs, servers, runtimes := routingFixtures()
	browsers := routingBrowsers()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		browsers,
	)
	service.probe = func(context.Context, int, string, int) error { return nil }

	result, err := service.Handle(context.Background(), "http://localhost:3000/")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Request.DefaultChoiceID != "proxy:server-socks:staging:firefox" {
		t.Fatalf("default choice = %q", result.Request.DefaultChoiceID)
	}
	if err := service.ResolveChoice(context.Background(), result.Request.ID, result.Request.DefaultChoiceID); err != nil {
		t.Fatalf("resolve assigned route: %v", err)
	}
	want := []browserCall{{
		configurationID: configdomain.ManagedSOCKSConfigurationID("staging"),
		browserID:       "firefox",
		url:             "http://localhost:3000/",
	}}
	if !reflect.DeepEqual(browsers.proxy, want) {
		t.Fatalf("proxy calls = %#v, want %#v", browsers.proxy, want)
	}
}

func TestHandleDoesNotDefaultToUnreachablePortAssignment(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.ProxyBrowserID = "google-chrome"
	pref.URLPortAssignments = []preferencesdomain.URLPortAssignment{{
		ID:        "docs",
		Port:      3000,
		ServerID:  "staging",
		BrowserID: "firefox",
	}}
	configs, servers, runtimes := routingFixtures()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		routingBrowsers(),
	)
	service.probe = func(_ context.Context, socksPort int, _ string, _ int) error {
		if socksPort == 41001 {
			return nil
		}
		return errors.New("closed")
	}

	result, err := service.Handle(context.Background(), "http://localhost:3000/")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Request.DefaultChoiceID != "proxy:server-socks:bts:google-chrome" {
		t.Fatalf("default choice = %q, want reachable proxy", result.Request.DefaultChoiceID)
	}
	if !hasChoice(result.Request.Choices, "proxy:server-socks:staging:firefox") {
		t.Fatalf("choices = %#v, want unmatched assigned proxy to remain selectable", result.Request.Choices)
	}
}

func TestHandleUsesFallbackWhenSeveralHostsReachUnassignedPort(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.ProxyBrowserID = "firefox"
	configs, servers, runtimes := routingFixtures()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		routingBrowsers(),
	)
	service.probe = func(context.Context, int, string, int) error { return nil }

	result, err := service.Handle(context.Background(), "http://127.0.0.1:3000/")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Request.DefaultChoiceID != "browser:safari" {
		t.Fatalf("default choice = %q, want browser:safari", result.Request.DefaultChoiceID)
	}
	if len(result.Request.Choices) < 7 {
		t.Fatalf("choices = %#v, want regular browsers and proxy combinations", result.Request.Choices)
	}
}

func TestHandleKeepsProxyBrowsersWithoutProbingURLWithoutExplicitPort(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	configs, servers, runtimes := routingFixtures()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		routingBrowsers(),
	)
	service.probe = func(context.Context, int, string, int) error {
		t.Fatal("unexpected probe")
		return nil
	}

	result, err := service.Handle(context.Background(), "https://example.com/path")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Request.DefaultChoiceID != "browser:safari" {
		t.Fatalf("default choice = %q", result.Request.DefaultChoiceID)
	}
	regularIndex := choiceIndex(result.Request.Choices, "browser:safari")
	proxyIndex := choiceIndex(result.Request.Choices, "proxy:server-socks:bts:google-chrome")
	if regularIndex < 0 || proxyIndex < 0 {
		t.Fatalf("choices = %#v, want regular and proxy browsers", result.Request.Choices)
	}
	if regularIndex >= proxyIndex {
		t.Fatalf("regular choice index = %d, proxy choice index = %d", regularIndex, proxyIndex)
	}
}

func TestHandleOffersStoppedManagedProxyAndUsesItsPortAssignment(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.URLPortAssignments = []preferencesdomain.URLPortAssignment{{
		ID:        "docs",
		Port:      3000,
		ServerID:  "staging",
		BrowserID: "firefox",
	}}
	configs, servers, _ := routingFixtures()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{},
		routingBrowsers(),
	)
	service.probe = func(context.Context, int, string, int) error {
		t.Fatal("a stopped proxy must not be probed")
		return nil
	}

	result, err := service.Handle(context.Background(), "http://localhost:3000/")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	wantChoice := "proxy:server-socks:staging:firefox"
	if result.Request == nil || !hasChoice(result.Request.Choices, wantChoice) {
		t.Fatalf("choices = %#v, want stopped managed proxy %q", result.Request, wantChoice)
	}
	if result.Request.DefaultChoiceID != wantChoice {
		t.Fatalf("default choice = %q, want %q", result.Request.DefaultChoiceID, wantChoice)
	}
}

func TestResolveChoiceRejectsStaleOrUnlistedTargets(t *testing.T) {
	pref := preferencesdomain.Default()
	configs, servers, runtimes := routingFixtures()
	browsers := routingBrowsers()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		browsers,
	)
	service.probe = func(context.Context, int, string, int) error { return nil }
	result, err := service.Handle(context.Background(), "http://localhost:3000")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}

	if err := service.ResolveChoice(context.Background(), "stale", result.Request.Choices[0].ID); err == nil {
		t.Fatal("expected stale request to fail")
	}
	if err := service.ResolveChoice(context.Background(), result.Request.ID, "not-a-choice"); err == nil {
		t.Fatal("expected unknown choice to fail")
	}
}

func TestHandleRejectsNonHTTPURLs(t *testing.T) {
	service := NewService(
		fakePreferences{value: preferencesdomain.Default()},
		fakeConfigurations{},
		fakeServers{},
		fakeRuntimes{},
		routingBrowsers(),
	)
	for _, rawURL := range []string{"", "file:///tmp/private", "javascript:alert(1)", "not a url"} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := service.Handle(context.Background(), rawURL); err == nil {
				t.Fatal("expected invalid URL error")
			}
		})
	}
}

func routingBrowsers() *fakeBrowsers {
	return &fakeBrowsers{destinations: []BrowserDestination{
		{ID: "google-chrome", Name: "Google Chrome", SupportsProxy: true},
		{ID: "firefox", Name: "Firefox", SupportsProxy: true},
		{ID: "brave-browser", Name: "Brave", SupportsProxy: true},
		{ID: "safari", Name: "Safari"},
	}}
}

func hasChoice(choices []RouteChoice, id string) bool {
	for _, choice := range choices {
		if choice.ID == id {
			return true
		}
	}
	return false
}

func choiceIndex(choices []RouteChoice, id string) int {
	for index, choice := range choices {
		if choice.ID == id {
			return index
		}
	}
	return -1
}

func routingFixtures() ([]configdomain.ConnectionConfiguration, []serverdomain.Server, []sessiondomain.RuntimeSession) {
	configs := []configdomain.ConnectionConfiguration{
		{ID: configdomain.ManagedSOCKSConfigurationID("bts"), ServerID: "bts", ConnectionType: configdomain.ConnectionTypeSOCKSProxy},
		{ID: configdomain.ManagedSOCKSConfigurationID("staging"), ServerID: "staging", ConnectionType: configdomain.ConnectionTypeSOCKSProxy},
		{ID: "user-socks", ServerID: "bts", ConnectionType: configdomain.ConnectionTypeSOCKSProxy},
	}
	servers := []serverdomain.Server{
		{ID: "staging", Name: "Staging"},
		{ID: "bts", Name: "BTS"},
	}
	runtimes := []sessiondomain.RuntimeSession{
		{ConfigurationID: configdomain.ManagedSOCKSConfigurationID("bts"), Status: sessiondomain.StatusConnected, BoundPort: 41001},
		{ConfigurationID: configdomain.ManagedSOCKSConfigurationID("staging"), Status: sessiondomain.StatusConnected, BoundPort: 41002},
		{ConfigurationID: "user-socks", Status: sessiondomain.StatusConnected, BoundPort: 41003},
	}
	return configs, servers, runtimes
}
