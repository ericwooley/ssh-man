package bindings

import (
	"strings"
	"testing"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	configdomain "ssh-man/internal/domain/config"
	sessiondomain "ssh-man/internal/domain/session"
)

func TestConfigurationBindingsRejectMutationWhileTunnelIsActive(t *testing.T) {
	runtimes := sessiondomain.NewRuntimeStore()
	runtimes.Set(sessiondomain.RuntimeSession{
		ConfigurationID: "config-1",
		Status:          sessiondomain.StatusConnected,
	}, nil, "")
	application := &bootstrap.Application{
		SessionService: sessiondomain.NewService(nil, nil, nil, runtimes),
	}
	bindings := NewAppBindingsWithApplication(application, appwindow.New())

	if _, err := bindings.SaveConnectionConfiguration(configdomain.ConnectionConfiguration{ID: "config-1"}); err == nil || !strings.Contains(err.Error(), "stop the configuration") {
		t.Fatalf("SaveConnectionConfiguration() error = %v, want active-tunnel guard", err)
	}
	if err := bindings.DeleteConnectionConfiguration("config-1"); err == nil || !strings.Contains(err.Error(), "stop the configuration") {
		t.Fatalf("DeleteConnectionConfiguration() error = %v, want active-tunnel guard", err)
	}
}

func TestConfigurationBindingsRejectDirectManagedProxyMutation(t *testing.T) {
	application := &bootstrap.Application{
		SessionService: sessiondomain.NewService(nil, nil, nil, sessiondomain.NewRuntimeStore()),
	}
	bindings := NewAppBindingsWithApplication(application, appwindow.New())
	managedID := configdomain.ManagedSOCKSConfigurationID("server-1")

	if _, err := bindings.SaveConnectionConfiguration(configdomain.ConnectionConfiguration{ID: managedID}); err == nil || !strings.Contains(err.Error(), "server configuration") {
		t.Fatalf("SaveConnectionConfiguration() error = %v, want server-owned proxy guard", err)
	}
	if err := bindings.DeleteConnectionConfiguration(managedID); err == nil || !strings.Contains(err.Error(), "belongs to its server") {
		t.Fatalf("DeleteConnectionConfiguration() error = %v, want server-owned proxy guard", err)
	}
}
