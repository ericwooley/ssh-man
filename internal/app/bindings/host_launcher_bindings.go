package bindings

import (
	"context"
	"fmt"
	"strings"

	"ssh-man/internal/app/bootstrap"
	"ssh-man/internal/app/hostwindow"
)

type HostLauncherBindings struct {
	servers explorerServerGetter
	launch  func(string) error
}

func NewHostLauncherBindings(app *bootstrap.Application) *HostLauncherBindings {
	return NewHostLauncherBindingsWithDependencies(app.ServerService, hostwindow.Launch)
}

func NewHostLauncherBindingsWithDependencies(servers explorerServerGetter, launch func(string) error) *HostLauncherBindings {
	return &HostLauncherBindings{servers: servers, launch: launch}
}

func (bindings *HostLauncherBindings) Open(serverID string) error {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	if bindings.servers == nil {
		return fmt.Errorf("server storage is unavailable")
	}
	if _, err := bindings.servers.Get(context.Background(), serverID); err != nil {
		return fmt.Errorf("load server before opening host window: %w", err)
	}
	if bindings.launch == nil {
		return fmt.Errorf("host window launcher is unavailable")
	}
	return bindings.launch(serverID)
}
