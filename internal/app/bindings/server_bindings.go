package bindings

import (
	"context"
	"fmt"

	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

func (a *AppBindings) SaveServer(input serverdomain.Server) (serverdomain.Server, error) {
	ctx := context.Background()
	var saved serverdomain.Server
	err := a.app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
		if input.ID != "" {
			if err := a.requireServerStopped(ctx, input.ID); err != nil {
				return err
			}
		}
		var err error
		saved, err = a.app.ServerService.Save(ctx, input)
		if err != nil {
			return a.storageError("The server could not be saved", err)
		}
		if _, err := a.app.ConfigService.EnsureManagedSOCKSConfiguration(ctx, saved); err != nil {
			return a.storageError("The automatic browser proxy could not be saved", err)
		}
		return nil
	})
	return saved, err
}

func (a *AppBindings) DeleteServer(serverID string) error {
	ctx := context.Background()
	return a.app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
		if err := a.requireServerStopped(ctx, serverID); err != nil {
			return err
		}
		if err := a.app.ServerService.Delete(ctx, serverID); err != nil {
			return a.storageError("The server could not be deleted", err)
		}
		return nil
	})
}

func (a *AppBindings) requireServerStopped(ctx context.Context, serverID string) error {
	configs, err := a.app.ConfigService.ListByServer(ctx, serverID)
	if err != nil {
		return err
	}
	for _, item := range configs {
		state, ok := a.app.SessionService.Get(item.ID)
		if ok && state.Status != sessiondomain.StatusStopped && state.Status != sessiondomain.StatusFailed {
			return fmt.Errorf("stop %q before changing or deleting the server", item.Label)
		}
	}
	return nil
}
