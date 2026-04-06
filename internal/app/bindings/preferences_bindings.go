package bindings

import (
	"context"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func (a *AppBindings) SavePreferences(input preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error) {
	return a.app.PreferencesService.Save(context.Background(), input)
}
