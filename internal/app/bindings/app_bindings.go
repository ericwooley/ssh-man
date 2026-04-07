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
	ctx context.Context
}

type ServerWithConfigurations struct {
	Server         serverdomain.Server                    `json:"server"`
	Configurations []configdomain.ConnectionConfiguration `json:"configurations"`
}

type Diagnostics struct {
	AppDataPath  string `json:"appDataPath"`
	DatabasePath string `json:"databasePath"`
}

type LoadInitialStateResult struct {
	Servers     []ServerWithConfigurations       `json:"servers"`
	Preferences preferencesdomain.UserPreference `json:"preferences"`
	Sessions    []any                            `json:"sessions"`
	Diagnostics Diagnostics                      `json:"diagnostics"`
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

func (a *AppBindings) SetContext(ctx context.Context) {
	a.ctx = ctx
}

func (a *AppBindings) Shutdown(ctx context.Context) error {
	return a.app.Shutdown(ctx)
}

func (a *AppBindings) storageError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s. App data: %s. Database: %s. %w", action, a.app.ConfigDir, a.app.DatabasePath, err)
}

func (a *AppBindings) LoadInitialState() (LoadInitialStateResult, error) {
	ctx := context.Background()
	result := LoadInitialStateResult{
		Preferences: preferencesdomain.Default(),
		Sessions:    []any{},
		Diagnostics: Diagnostics{AppDataPath: a.app.ConfigDir, DatabasePath: a.app.DatabasePath},
	}

	servers, err := a.app.ServerService.List(ctx)
	if err != nil {
		result.Message = a.storageError("Saved data could not be loaded", err).Error()
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
			result.Message = a.storageError(fmt.Sprintf("Some saved tunnel details could not be loaded for %q", item.Name), err).Error()
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
		message = a.storageError("Preferences could not be loaded; defaults are in use", prefErr).Error()
	}

	return LoadInitialStateResult{
		Servers:     items,
		Preferences: pref,
		Sessions:    sessions,
		Diagnostics: result.Diagnostics,
		Message:     message,
		Recoverable: prefErr != nil,
	}, nil
}
