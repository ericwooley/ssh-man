package bindings

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ssh-man/internal/app/bootstrap"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

type initialStateServerStore struct{}

func (initialStateServerStore) List(context.Context) ([]serverdomain.Server, error) {
	return []serverdomain.Server{}, nil
}

func (initialStateServerStore) Get(context.Context, string) (serverdomain.Server, error) {
	return serverdomain.Server{}, nil
}

func (initialStateServerStore) Save(context.Context, serverdomain.Server) error {
	return nil
}

func (initialStateServerStore) Delete(context.Context, string) error {
	return nil
}

type initialStatePreferencesStore struct{}

func (initialStatePreferencesStore) Load(context.Context) (preferencesdomain.UserPreference, error) {
	return preferencesdomain.Default(), nil
}

func (initialStatePreferencesStore) Save(context.Context, preferencesdomain.UserPreference) error {
	return nil
}

func TestLoadInitialStateIncludesInjectedBuildVersion(t *testing.T) {
	application := &bootstrap.Application{
		ConfigDir:          "/tmp/ssh-man",
		DatabasePath:       "/tmp/ssh-man/ssh-man.db",
		ServerService:      serverdomain.NewService(initialStateServerStore{}),
		PreferencesService: preferencesdomain.NewService(initialStatePreferencesStore{}),
		SessionService:     sessiondomain.NewService(nil, nil, nil, sessiondomain.NewRuntimeStore()),
	}
	bindings := newAppBindingsWithApplication(application, nil, "1.2.3")

	state, err := bindings.LoadInitialState()
	if err != nil {
		t.Fatalf("LoadInitialState() error = %v", err)
	}
	if state.Diagnostics.Version != "1.2.3" {
		t.Fatalf("diagnostics version = %q, want %q", state.Diagnostics.Version, "1.2.3")
	}
	if !state.Preferences.AutomaticUpdates {
		t.Fatal("automatic updates should be enabled in default initial state")
	}

	encoded, err := json.Marshal(state.Diagnostics)
	if err != nil {
		t.Fatalf("marshal diagnostics: %v", err)
	}
	if !strings.Contains(string(encoded), `"version":"1.2.3"`) {
		t.Fatalf("diagnostics JSON = %s, want version field", encoded)
	}
	if !strings.Contains(string(encoded), `"automaticUpdatesSupported":`) {
		t.Fatalf("diagnostics JSON = %s, want automaticUpdatesSupported field", encoded)
	}
}
