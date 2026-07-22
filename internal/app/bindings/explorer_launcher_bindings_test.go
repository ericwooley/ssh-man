package bindings

import (
	"context"
	"errors"
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

type fakeExplorerServerGetter struct {
	server serverdomain.Server
	err    error
}

func (f fakeExplorerServerGetter) Get(context.Context, string) (serverdomain.Server, error) {
	return f.server, f.err
}

func TestExplorerLauncherValidatesServerBeforeStartingProcess(t *testing.T) {
	launched := ""
	binding := NewExplorerLauncherBindingsWithDependencies(
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

func TestExplorerLauncherDoesNotStartForUnknownServer(t *testing.T) {
	launched := false
	binding := NewExplorerLauncherBindingsWithDependencies(
		fakeExplorerServerGetter{err: errors.New("not found")},
		func(string) error { launched = true; return nil },
	)

	err := binding.Open("missing")

	if err == nil || launched {
		t.Fatalf("Open() error = %v, launched = %v", err, launched)
	}
}
