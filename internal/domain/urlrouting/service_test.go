package urlrouting

import (
	"context"
	"errors"
	"reflect"
	"testing"

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
	regular []browserCall
	proxy   []browserCall
}

func (f *fakeBrowsers) OpenURL(_ context.Context, browserID, rawURL string) error {
	f.regular = append(f.regular, browserCall{browserID: browserID, url: rawURL})
	return nil
}

func (f *fakeBrowsers) LaunchThroughSOCKSURL(_ context.Context, configurationID, browserID, rawURL string) error {
	f.proxy = append(f.proxy, browserCall{configurationID: configurationID, browserID: browserID, url: rawURL})
	return nil
}

func TestHandleUsesFirstMatchingRuleBeforeLocalhostDiscovery(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.URLRules = []preferencesdomain.URLRule{
		{ID: "work", Pattern: `^https://github\.com/workorg/`, Action: preferencesdomain.URLRuleActionBrowser, BrowserID: "brave-browser"},
		{ID: "later", Pattern: `github`, Action: preferencesdomain.URLRuleActionBrowser, BrowserID: "firefox"},
	}
	browsers := &fakeBrowsers{}
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, browsers)
	service.probe = func(context.Context, int, string, int) error {
		t.Fatal("localhost probe should not run for a matching rule")
		return nil
	}

	result, err := service.Handle(context.Background(), "https://github.com/workorg/repo")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Kind != ResultOpened {
		t.Fatalf("result = %#v, want opened", result)
	}
	want := []browserCall{{browserID: "brave-browser", url: "https://github.com/workorg/repo"}}
	if !reflect.DeepEqual(browsers.regular, want) {
		t.Fatalf("regular calls = %#v, want %#v", browsers.regular, want)
	}
}

func TestHandleExecutesMatchingCommandWithURLPlaceholder(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.URLRules = []preferencesdomain.URLRule{{
		ID:      "work-container",
		Pattern: `^https://github\.com/workorg/`,
		Action:  preferencesdomain.URLRuleActionCommand,
		Command: `open -a "Zen" "ext+container:name=Work&url=<URL>"`,
	}}
	service := NewService(fakePreferences{value: pref}, fakeConfigurations{}, fakeServers{}, fakeRuntimes{}, &fakeBrowsers{})
	var gotTemplate, gotURL string
	service.runCommand = func(template, rawURL string) error {
		gotTemplate, gotURL = template, rawURL
		return nil
	}

	result, err := service.Handle(context.Background(), "https://github.com/workorg/repo?q=a&b=c")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Kind != ResultCommandExecuted || gotTemplate != pref.URLRules[0].Command || gotURL != "https://github.com/workorg/repo?q=a&b=c" {
		t.Fatalf("unexpected result/call: %#v template=%q url=%q", result, gotTemplate, gotURL)
	}
}

func TestHandleRoutesLocalhostToOnlyReachableConnectedServer(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.DefaultBrowserID = "safari"
	pref.ProxyBrowserID = "google-chrome"
	configs, servers, runtimes := routingFixtures()
	browsers := &fakeBrowsers{}
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		browsers,
	)
	service.probe = func(_ context.Context, socksPort int, host string, port int) error {
		if host != "127.0.0.1" || port != 3000 {
			t.Fatalf("probe target = %s:%d, want 127.0.0.1:3000", host, port)
		}
		if socksPort == 41001 {
			return nil
		}
		return errors.New("closed")
	}

	result, err := service.Handle(context.Background(), "http://localhost:3000/dashboard")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Kind != ResultOpened {
		t.Fatalf("result = %#v, want opened", result)
	}
	want := []browserCall{{
		configurationID: configdomain.ManagedSOCKSConfigurationID("bts"),
		browserID:       "google-chrome",
		url:             "http://localhost:3000/dashboard",
	}}
	if !reflect.DeepEqual(browsers.proxy, want) {
		t.Fatalf("proxy calls = %#v, want %#v", browsers.proxy, want)
	}
}

func TestHandleRequestsChoiceWhenMultipleServersReachPort(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.ProxyBrowserID = "firefox"
	configs, servers, runtimes := routingFixtures()
	service := NewService(
		fakePreferences{value: pref},
		fakeConfigurations{items: configs},
		fakeServers{items: servers},
		fakeRuntimes{items: runtimes},
		&fakeBrowsers{},
	)
	service.probe = func(context.Context, int, string, int) error { return nil }

	result, err := service.Handle(context.Background(), "http://127.0.0.1:3000/")
	if err != nil {
		t.Fatalf("handle url: %v", err)
	}
	if result.Kind != ResultNeedsChoice || result.Request.ID == "" {
		t.Fatalf("result = %#v, want choice request", result)
	}
	if len(result.Request.Choices) != 2 ||
		result.Request.Choices[0].ServerName != "BTS" ||
		result.Request.Choices[1].ServerName != "Staging" {
		t.Fatalf("choices = %#v", result.Request.Choices)
	}
	if pending, ok := service.Pending(); !ok || pending.ID != result.Request.ID {
		t.Fatalf("pending = %#v, %v", pending, ok)
	}
}

func TestResolveChoiceRejectsStaleOrUnlistedTargets(t *testing.T) {
	pref := preferencesdomain.Default()
	configs, servers, runtimes := routingFixtures()
	browsers := &fakeBrowsers{}
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
	if err := service.ResolveChoice(context.Background(), result.Request.ID, result.Request.Choices[1].ID); err != nil {
		t.Fatalf("resolve valid choice: %v", err)
	}
	if len(browsers.proxy) != 1 || browsers.proxy[0].configurationID != configdomain.ManagedSOCKSConfigurationID("staging") {
		t.Fatalf("proxy calls = %#v", browsers.proxy)
	}
}

func TestHandleFallsBackForNonLocalOrUnreachableURLs(t *testing.T) {
	for _, rawURL := range []string{"https://example.com/path", "http://localhost:3999"} {
		t.Run(rawURL, func(t *testing.T) {
			pref := preferencesdomain.Default()
			pref.DefaultBrowserID = "safari"
			configs, servers, runtimes := routingFixtures()
			browsers := &fakeBrowsers{}
			service := NewService(
				fakePreferences{value: pref},
				fakeConfigurations{items: configs},
				fakeServers{items: servers},
				fakeRuntimes{items: runtimes},
				browsers,
			)
			service.probe = func(context.Context, int, string, int) error { return errors.New("closed") }

			if _, err := service.Handle(context.Background(), rawURL); err != nil {
				t.Fatalf("handle url: %v", err)
			}
			if len(browsers.regular) != 1 || browsers.regular[0].browserID != "safari" || browsers.regular[0].url != rawURL {
				t.Fatalf("regular calls = %#v", browsers.regular)
			}
		})
	}
}

func TestHandleRejectsNonHTTPURLs(t *testing.T) {
	service := NewService(
		fakePreferences{value: preferencesdomain.Default()},
		fakeConfigurations{},
		fakeServers{},
		fakeRuntimes{},
		&fakeBrowsers{},
	)
	for _, rawURL := range []string{"", "file:///tmp/private", "javascript:alert(1)", "not a url"} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := service.Handle(context.Background(), rawURL); err == nil {
				t.Fatal("expected invalid URL error")
			}
		})
	}
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
