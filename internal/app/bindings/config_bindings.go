package bindings

import (
	"context"
	"fmt"

	configdomain "ssh-man/internal/domain/config"
)

func (a *AppBindings) SaveConnectionConfiguration(input configdomain.ConnectionConfiguration) (configdomain.ConnectionConfiguration, error) {
	return a.app.ConfigService.Save(context.Background(), input)
}

func (a *AppBindings) DeleteConnectionConfiguration(configurationID string) error {
	state, ok := a.app.SessionService.Get(configurationID)
	if ok && state.Status != "stopped" {
		return fmt.Errorf("stop the configuration before deleting it")
	}
	return a.app.ConfigService.Delete(context.Background(), configurationID)
}
