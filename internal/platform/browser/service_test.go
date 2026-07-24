package browser

import (
	"context"
	"strings"
	"testing"

	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

type stubRuntimeLookup struct {
	state sessiondomain.RuntimeSession
	ok    bool
}

func (s stubRuntimeLookup) Get(string) (sessiondomain.RuntimeSession, bool) {
	return s.state, s.ok
}

type stubConfigLookup struct {
	item configdomain.ConnectionConfiguration
}

func (s stubConfigLookup) Get(context.Context, string) (configdomain.ConnectionConfiguration, error) {
	return s.item, nil
}

type stubPreferenceLookup struct {
	pref preferencesdomain.UserPreference
}

func (s stubPreferenceLookup) Load(context.Context) (preferencesdomain.UserPreference, error) {
	return s.pref, nil
}

func TestLaunchThroughSOCKSRequiresConnectedSession(t *testing.T) {
	service := NewService(
		"/Users/test/Library/Application Support/ssh-man",
		stubConfigLookup{item: configdomain.ConnectionConfiguration{ID: "config-1", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubRuntimeLookup{state: sessiondomain.RuntimeSession{ConfigurationID: "config-1", Status: sessiondomain.StatusStopped}, ok: true},
		nil,
	)

	err := service.LaunchThroughSOCKS(context.Background(), "config-1", "google-chrome")
	if err == nil {
		t.Fatal("expected connected-session validation error")
	}
	if !strings.Contains(err.Error(), "start the socks configuration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLaunchThroughSOCKSRejectsNonSOCKSConfiguration(t *testing.T) {
	service := NewService(
		"/Users/test/Library/Application Support/ssh-man",
		stubConfigLookup{item: configdomain.ConnectionConfiguration{ID: "config-1", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9000}},
		stubRuntimeLookup{state: sessiondomain.RuntimeSession{ConfigurationID: "config-1", Status: sessiondomain.StatusConnected}, ok: true},
		nil,
	)

	err := service.LaunchThroughSOCKS(context.Background(), "config-1", "google-chrome")
	if err == nil {
		t.Fatal("expected socks-only validation error")
	}
	if !strings.Contains(err.Error(), "only available for socks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLaunchThroughSOCKSRequiresRuntimeBoundPort(t *testing.T) {
	service := NewService(
		"/Users/test/Library/Application Support/ssh-man",
		stubConfigLookup{item: configdomain.ConnectionConfiguration{ID: "config-1", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 0}},
		stubRuntimeLookup{state: sessiondomain.RuntimeSession{ConfigurationID: "config-1", Status: sessiondomain.StatusConnected}, ok: true},
		nil,
	)

	err := service.LaunchThroughSOCKS(context.Background(), "config-1", "google-chrome")
	if err == nil {
		t.Fatal("expected missing bound-port validation error")
	}
	if !strings.Contains(err.Error(), "local port is unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreviewLaunchThroughSOCKSIncludesCommand(t *testing.T) {
	service := NewService(
		"/Users/test/Library/Application Support/ssh-man",
		stubConfigLookup{item: configdomain.ConnectionConfiguration{ID: "config-1", ServerID: "server-1", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubRuntimeLookup{state: sessiondomain.RuntimeSession{ConfigurationID: "config-1", Status: sessiondomain.StatusConnected, BoundPort: 43123}, ok: true},
		nil,
	)
	service.discover = func(context.Context) ([]BrowserOption, error) {
		return []BrowserOption{{ID: "google-chrome", DisplayName: "Google Chrome", LaunchReference: "/usr/bin/google-chrome", SupportsProxyLaunch: true}}, nil
	}

	preview, err := service.PreviewLaunchThroughSOCKS(context.Background(), "config-1", "google-chrome")
	if err != nil {
		t.Fatalf("unexpected preview error: %v", err)
	}
	if preview.BrowserID != "google-chrome" {
		t.Fatalf("unexpected browser id: %#v", preview)
	}
	if !strings.Contains(preview.Command, "43123") {
		t.Fatalf("expected preview command to include runtime port, got %q", preview.Command)
	}
	if !strings.Contains(preview.Command, "ssh-man") || !strings.Contains(preview.Command, "server-1") {
		t.Fatalf("expected preview command to include persistent per-server profile path, got %q", preview.Command)
	}
}

func TestActivateRunningBrowserRevalidatesTargetBeforeActivatingPID(t *testing.T) {
	service := NewService("/tmp/ssh-man", stubConfigLookup{}, stubRuntimeLookup{}, nil)
	service.discover = func(context.Context) ([]BrowserOption, error) {
		return []BrowserOption{{ID: "google-chrome", DisplayName: "Google Chrome"}}, nil
	}
	service.listRunning = func(context.Context, string, []BrowserOption, []serverdomain.Server) ([]RunningTarget, error) {
		return []RunningTarget{{ID: "browser:202", PID: 202, BrowserName: "Google Chrome"}}, nil
	}
	activatedPID := 0
	service.activate = func(pid int) error {
		activatedPID = pid
		return nil
	}

	if err := service.ActivateRunning(context.Background(), "browser:202"); err != nil {
		t.Fatalf("activate running browser: %v", err)
	}
	if activatedPID != 202 {
		t.Fatalf("activated PID = %d, want 202", activatedPID)
	}
	if err := service.ActivateRunning(context.Background(), "browser:999"); err == nil {
		t.Fatal("expected stale target to be rejected")
	}
}

func TestOpenURLUsesSelectedRegularBrowserAndPreservesURL(t *testing.T) {
	service := NewService("/tmp/ssh-man", stubConfigLookup{}, stubRuntimeLookup{}, nil)
	service.discover = func(context.Context) ([]BrowserOption, error) {
		return []BrowserOption{
			{ID: "safari", DisplayName: "Safari", LaunchReference: "/Applications/Safari.app"},
			{ID: "firefox", DisplayName: "Firefox", LaunchReference: "/Applications/Firefox.app"},
		}, nil
	}
	var gotOption BrowserOption
	var gotURL string
	service.openURL = func(option BrowserOption, rawURL string) error {
		gotOption, gotURL = option, rawURL
		return nil
	}

	rawURL := "https://github.com/workorg/repo?q=a&b=c"
	if err := service.OpenURL(context.Background(), "firefox", rawURL); err != nil {
		t.Fatalf("open URL: %v", err)
	}
	if gotOption.ID != "firefox" || gotURL != rawURL {
		t.Fatalf("open call = %#v %q", gotOption, gotURL)
	}
}

func TestOpenURLFallsBackToFirstInstalledBrowserWhenPreferenceIsEmpty(t *testing.T) {
	service := NewService("/tmp/ssh-man", stubConfigLookup{}, stubRuntimeLookup{}, nil)
	service.discover = func(context.Context) ([]BrowserOption, error) {
		return []BrowserOption{
			{ID: "safari", DisplayName: "Safari"},
			{ID: "firefox", DisplayName: "Firefox"},
		}, nil
	}
	var gotID string
	service.openURL = func(option BrowserOption, _ string) error {
		gotID = option.ID
		return nil
	}

	if err := service.OpenURL(context.Background(), "", "https://example.com"); err != nil {
		t.Fatalf("open URL: %v", err)
	}
	if gotID != "safari" {
		t.Fatalf("opened browser %q, want first installed browser", gotID)
	}
}

func TestLaunchThroughSOCKSURLPassesURLToProxyLauncher(t *testing.T) {
	service := NewService(
		"/tmp/ssh-man",
		stubConfigLookup{item: configdomain.ConnectionConfiguration{
			ID:             "proxy",
			ServerID:       "bts",
			ConnectionType: configdomain.ConnectionTypeSOCKSProxy,
		}},
		stubRuntimeLookup{
			state: sessiondomain.RuntimeSession{
				ConfigurationID: "proxy",
				Status:          sessiondomain.StatusConnected,
				BoundPort:       41001,
			},
			ok: true,
		},
		nil,
	)
	service.discover = func(context.Context) ([]BrowserOption, error) {
		return []BrowserOption{
			{ID: "google-chrome", DisplayName: "Google Chrome", SupportsProxyLaunch: true},
		}, nil
	}
	var gotServerID, gotURL string
	var gotPort int
	service.launchProxy = func(_ string, serverID string, _ BrowserOption, socksPort int, rawURL string) error {
		gotServerID, gotPort, gotURL = serverID, socksPort, rawURL
		return nil
	}

	rawURL := "http://localhost:3000/dashboard"
	if err := service.LaunchThroughSOCKSURL(context.Background(), "proxy", "", rawURL); err != nil {
		t.Fatalf("launch proxy URL: %v", err)
	}
	if gotServerID != "bts" || gotPort != 41001 || gotURL != rawURL {
		t.Fatalf("proxy launch = server %q port %d url %q", gotServerID, gotPort, gotURL)
	}
}

func TestBrowserURLMethodsRejectUnsafeOrUnsupportedURLs(t *testing.T) {
	service := NewService("/tmp/ssh-man", stubConfigLookup{}, stubRuntimeLookup{}, nil)
	service.discover = func(context.Context) ([]BrowserOption, error) {
		return []BrowserOption{{ID: "safari", DisplayName: "Safari"}}, nil
	}
	service.openURL = func(BrowserOption, string) error {
		t.Fatal("platform opener should not be called")
		return nil
	}

	for _, rawURL := range []string{"file:///tmp/secret", "javascript:alert(1)", "http://user:password@example.com"} {
		if err := service.OpenURL(context.Background(), "safari", rawURL); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
}

func TestDiscoverMergesAvailableCustomBrowsersWithoutOverridingBuiltIns(t *testing.T) {
	pref := preferencesdomain.Default()
	pref.CustomBrowsers = []preferencesdomain.CustomBrowser{
		{ID: "custom-kagi", DisplayName: "Kagi Browser", LaunchReference: "/Applications/Kagi Browser.app", Engine: preferencesdomain.BrowserEngineChromium},
		{ID: "firefox", DisplayName: "Replacement Firefox", LaunchReference: "/Applications/Replacement.app", Engine: preferencesdomain.BrowserEngineRegular},
	}
	service := NewService("/tmp/ssh-man", stubConfigLookup{}, stubRuntimeLookup{}, nil, stubPreferenceLookup{pref: pref})
	service.discover = func(context.Context) ([]BrowserOption, error) {
		return []BrowserOption{{ID: "firefox", DisplayName: "Firefox", LaunchReference: "/Applications/Firefox.app", Engine: BrowserEngineFirefox}}, nil
	}
	service.discoverCustom = func(custom []preferencesdomain.CustomBrowser) []BrowserOption {
		return []BrowserOption{
			{ID: custom[0].ID, DisplayName: custom[0].DisplayName, LaunchReference: custom[0].LaunchReference, Engine: BrowserEngineChromium, SupportsProxyLaunch: true, Custom: true},
			{ID: custom[1].ID, DisplayName: custom[1].DisplayName, LaunchReference: custom[1].LaunchReference, Engine: BrowserEngineRegular, Custom: true},
		}
	}

	got, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover browsers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("browsers = %#v, want built-in Firefox plus custom Kagi", got)
	}
	if got[0].DisplayName != "Firefox" || got[1].ID != "custom-kagi" || !got[1].Custom || !got[1].SupportsProxyLaunch {
		t.Fatalf("merged browsers = %#v", got)
	}
}
