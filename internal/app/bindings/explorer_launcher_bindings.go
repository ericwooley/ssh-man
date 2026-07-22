package bindings

import (
	"context"
	"fmt"
	"strings"

	"ssh-man/internal/app/bootstrap"
	"ssh-man/internal/app/explorerwindow"
	serverdomain "ssh-man/internal/domain/server"
)

type explorerServerGetter interface {
	Get(context.Context, string) (serverdomain.Server, error)
}

// ExplorerLauncherBindings is kept separate from AppBindings so companion
// window launching remains a small, explicit side-effect boundary.
type ExplorerLauncherBindings struct {
	servers explorerServerGetter
	launch  func(string) error
}

func NewExplorerLauncherBindings(app *bootstrap.Application) *ExplorerLauncherBindings {
	return NewExplorerLauncherBindingsWithDependencies(app.ServerService, explorerwindow.Launch)
}

func NewExplorerLauncherBindingsWithDependencies(servers explorerServerGetter, launch func(string) error) *ExplorerLauncherBindings {
	return &ExplorerLauncherBindings{servers: servers, launch: launch}
}

func (b *ExplorerLauncherBindings) Open(serverID string) error {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	if b.servers == nil {
		return fmt.Errorf("server storage is unavailable")
	}
	if _, err := b.servers.Get(context.Background(), serverID); err != nil {
		return fmt.Errorf("load server before opening explorer: %w", err)
	}
	if b.launch == nil {
		return fmt.Errorf("server explorer launcher is unavailable")
	}
	return b.launch(serverID)
}
