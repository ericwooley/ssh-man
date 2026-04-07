package bindings

import (
	"context"
	"fmt"

	configdomain "ssh-man/internal/domain/config"
)

func (a *AppBindings) SaveConnectionConfiguration(input configdomain.ConnectionConfiguration) (configdomain.ConnectionConfiguration, error) {
	item, err := a.app.ConfigService.Save(context.Background(), input)
	if err != nil {
		return configdomain.ConnectionConfiguration{}, a.storageError("The tunnel could not be saved", err)
	}
	return item, nil
}

func (a *AppBindings) DeleteConnectionConfiguration(configurationID string) error {
	state, ok := a.app.SessionService.Get(configurationID)
	if ok && state.Status != "stopped" {
		return fmt.Errorf("stop the configuration before deleting it")
	}
	if err := a.app.ConfigService.Delete(context.Background(), configurationID); err != nil {
		return a.storageError("The tunnel could not be deleted", err)
	}
	return nil
}
