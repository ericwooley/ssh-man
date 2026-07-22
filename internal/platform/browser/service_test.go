package browser

import (
	"context"
	"strings"
	"testing"

	configdomain "ssh-man/internal/domain/config"
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
