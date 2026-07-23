package bindings

import (
	"context"
	"fmt"

	configdomain "ssh-man/internal/domain/config"
	sessiondomain "ssh-man/internal/domain/session"
)

func (a *AppBindings) SaveConnectionConfiguration(input configdomain.ConnectionConfiguration) (configdomain.ConnectionConfiguration, error) {
	if configdomain.IsManagedSOCKSConfigurationID(input.ID) {
		return configdomain.ConnectionConfiguration{}, fmt.Errorf("edit the browser SOCKS port in the server configuration")
	}
	ctx := context.Background()
	var saved configdomain.ConnectionConfiguration
	err := a.app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
		if input.ID != "" {
			if err := a.requireConfigurationStopped(input.ID); err != nil {
				return err
			}
		}
		var err error
		saved, err = a.app.ConfigService.Save(ctx, input)
		if err != nil {
			return a.storageError("The tunnel could not be saved", err)
		}
		return nil
	})
	return saved, err
}

func (a *AppBindings) DeleteConnectionConfiguration(configurationID string) error {
	if configdomain.IsManagedSOCKSConfigurationID(configurationID) {
		return fmt.Errorf("the automatic browser proxy belongs to its server")
	}
	ctx := context.Background()
	return a.app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
		if err := a.requireConfigurationStopped(configurationID); err != nil {
			return err
		}
		if err := a.app.ConfigService.Delete(ctx, configurationID); err != nil {
			return a.storageError("The tunnel could not be deleted", err)
		}
		return nil
	})
}

func (a *AppBindings) requireConfigurationStopped(configurationID string) error {
	state, ok := a.app.SessionService.Get(configurationID)
	if ok && state.Status != sessiondomain.StatusStopped && state.Status != sessiondomain.StatusFailed {
		return fmt.Errorf("stop the configuration before changing or deleting it")
	}
	return nil
}
