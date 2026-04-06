package bindings

import (
	"context"
	"fmt"

	"ssh-man/internal/app/bootstrap"
	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
)

type AppBindings struct {
	app *bootstrap.Application
}

type ServerWithConfigurations struct {
	Server         serverdomain.Server                    `json:"server"`
	Configurations []configdomain.ConnectionConfiguration `json:"configurations"`
}

type LoadInitialStateResult struct {
	Servers     []ServerWithConfigurations       `json:"servers"`
	Preferences preferencesdomain.UserPreference `json:"preferences"`
	Sessions    []any                            `json:"sessions"`
	Message     string                           `json:"message,omitempty"`
	Recoverable bool                             `json:"recoverable,omitempty"`
}

func NewAppBindings() (*AppBindings, error) {
	app, err := bootstrap.New(context.Background())
	if err != nil {
		return nil, err
	}
	return &AppBindings{app: app}, nil
}

func (a *AppBindings) LoadInitialState() (LoadInitialStateResult, error) {
	ctx := context.Background()
	result := LoadInitialStateResult{Preferences: preferencesdomain.Default(), Sessions: []any{}}

	servers, err := a.app.ServerService.List(ctx)
	if err != nil {
		result.Message = fmt.Sprintf("Saved data could not be loaded. Check that the app data directory is writable and that the SQLite database is healthy. %v", err)
		result.Recoverable = true
		return result, nil
	}
	pref, prefErr := a.app.PreferencesService.Load(ctx)
	if prefErr != nil {
		pref = preferencesdomain.Default()
	}

	items := make([]ServerWithConfigurations, 0, len(servers))
	for _, item := range servers {
		configs, err := a.app.ConfigService.ListByServer(ctx, item.ID)
		if err != nil {
			result.Message = fmt.Sprintf("Some saved tunnel details could not be loaded for %q. Check the local database and retry. %v", item.Name, err)
			result.Recoverable = true
			result.Servers = items
			result.Preferences = pref
			return result, nil
		}
		items = append(items, ServerWithConfigurations{Server: item, Configurations: configs})
	}

	runtimeStates := a.app.SessionService.List()
	sessions := make([]any, 0, len(runtimeStates))
	for _, runtimeState := range runtimeStates {
		sessions = append(sessions, runtimeState)
	}

	message := ""
	if prefErr != nil {
		message = "Preferences could not be loaded; defaults are in use."
	}

	return LoadInitialStateResult{Servers: items, Preferences: pref, Sessions: sessions, Message: message, Recoverable: prefErr != nil}, nil
}
