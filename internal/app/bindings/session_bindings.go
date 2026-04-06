package bindings

import (
	"context"
	"fmt"
)

func (a *AppBindings) StartConfiguration(configurationID string) (any, error) {
	state, err := a.app.SessionService.Start(context.Background(), configurationID)
	if err != nil {
		return nil, fmt.Errorf("couldn't start the tunnel. %w", err)
	}
	return state, nil
}

func (a *AppBindings) StopConfiguration(configurationID string) (any, error) {
	state, err := a.app.SessionService.Stop(context.Background(), configurationID)
	if err != nil {
		return nil, fmt.Errorf("couldn't stop the tunnel cleanly. %w", err)
	}
	return state, nil
}

func (a *AppBindings) RetryConfiguration(configurationID string) (any, error) {
	state, err := a.app.SessionService.Retry(context.Background(), configurationID)
	if err != nil {
		return nil, fmt.Errorf("couldn't retry the tunnel. %w", err)
	}
	return state, nil
}

func (a *AppBindings) SubmitKeyUnlock(configurationID string, secret string) (any, error) {
	state, err := a.app.SessionService.SubmitKeyUnlock(context.Background(), configurationID, secret)
	if err != nil {
		return nil, fmt.Errorf("couldn't unlock the SSH key for this tunnel. %w", err)
	}
	return state, nil
}
