package bindings

import (
	"context"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func (a *AppBindings) SavePreferences(input preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error) {
	pref, err := a.app.PreferencesService.Save(context.Background(), input)
	if err != nil {
		return preferencesdomain.UserPreference{}, a.storageError("The preference could not be saved", err)
	}
	return pref, nil
}
