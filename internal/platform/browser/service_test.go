package browser

import (
	"context"
	"strings"
	"testing"

	configdomain "ssh-man/internal/domain/config"
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
		stubConfigLookup{item: configdomain.ConnectionConfiguration{ID: "config-1", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080}},
		stubRuntimeLookup{state: sessiondomain.RuntimeSession{ConfigurationID: "config-1", Status: sessiondomain.StatusStopped}, ok: true},
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
		stubConfigLookup{item: configdomain.ConnectionConfiguration{ID: "config-1", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 9000}},
		stubRuntimeLookup{state: sessiondomain.RuntimeSession{ConfigurationID: "config-1", Status: sessiondomain.StatusConnected}, ok: true},
	)

	err := service.LaunchThroughSOCKS(context.Background(), "config-1", "google-chrome")
	if err == nil {
		t.Fatal("expected socks-only validation error")
	}
	if !strings.Contains(err.Error(), "only available for socks") {
		t.Fatalf("unexpected error: %v", err)
	}
}
