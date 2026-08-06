package bindings

import (
	"errors"
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

func TestHostLauncherValidatesServerBeforeStartingProcess(t *testing.T) {
	launched := ""
	binding := NewHostLauncherBindingsWithDependencies(
		fakeExplorerServerGetter{server: serverdomain.Server{ID: "server-1"}},
		func(serverID string) error {
			launched = serverID
			return nil
		},
	)

	if err := binding.Open(" server-1 "); err != nil {
		t.Fatal(err)
	}
	if launched != "server-1" {
		t.Fatalf("launched server = %q", launched)
	}
}

func TestHostLauncherDoesNotStartForUnknownServer(t *testing.T) {
	launched := false
	binding := NewHostLauncherBindingsWithDependencies(
		fakeExplorerServerGetter{err: errors.New("not found")},
		func(string) error { launched = true; return nil },
	)

	err := binding.Open("missing")
	if err == nil || launched {
		t.Fatalf("Open() error = %v, launched = %v", err, launched)
	}
}
