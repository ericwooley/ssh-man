package bindings

import (
	"context"
	"fmt"

	serverdomain "ssh-man/internal/domain/server"
)

func (a *AppBindings) SaveServer(input serverdomain.Server) (serverdomain.Server, error) {
	item, err := a.app.ServerService.Save(context.Background(), input)
	if err != nil {
		return serverdomain.Server{}, a.storageError("The server could not be saved", err)
	}
	return item, nil
}

func (a *AppBindings) DeleteServer(serverID string) error {
	configs, err := a.app.ConfigService.ListByServer(context.Background(), serverID)
	if err != nil {
		return err
	}
	for _, item := range configs {
		state, ok := a.app.SessionService.Get(item.ID)
		if ok && state.Status != "stopped" {
			return fmt.Errorf("stop %q before deleting the server", item.Label)
		}
	}
	if err := a.app.ServerService.Delete(context.Background(), serverID); err != nil {
		return a.storageError("The server could not be deleted", err)
	}
	return nil
}
