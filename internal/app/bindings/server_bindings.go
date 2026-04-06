package bindings

import (
	"context"
	"fmt"

	serverdomain "ssh-man/internal/domain/server"
)

func (a *AppBindings) SaveServer(input serverdomain.Server) (serverdomain.Server, error) {
	return a.app.ServerService.Save(context.Background(), input)
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
	return a.app.ServerService.Delete(context.Background(), serverID)
}
