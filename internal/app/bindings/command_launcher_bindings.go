package bindings

import (
	"context"
	"fmt"
	"strings"

	"ssh-man/internal/app/bootstrap"
	"ssh-man/internal/app/commandwindow"
)

type CommandLauncherBindings struct {
	servers explorerServerGetter
	launch  func(string) error
}

func NewCommandLauncherBindings(app *bootstrap.Application) *CommandLauncherBindings {
	return NewCommandLauncherBindingsWithDependencies(app.ServerService, commandwindow.Launch)
}

func NewCommandLauncherBindingsWithDependencies(servers explorerServerGetter, launch func(string) error) *CommandLauncherBindings {
	return &CommandLauncherBindings{servers: servers, launch: launch}
}

func (b *CommandLauncherBindings) Open(serverID string) error {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	if b.servers == nil {
		return fmt.Errorf("server storage is unavailable")
	}
	if _, err := b.servers.Get(context.Background(), serverID); err != nil {
		return fmt.Errorf("load server before opening quick command: %w", err)
	}
	if b.launch == nil {
		return fmt.Errorf("quick command launcher is unavailable")
	}
	return b.launch(serverID)
}
