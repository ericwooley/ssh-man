package bindings

import (
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

func TestCommandLauncherValidatesServerBeforeStartingProcess(t *testing.T) {
	launched := ""
	binding := NewCommandLauncherBindingsWithDependencies(
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
