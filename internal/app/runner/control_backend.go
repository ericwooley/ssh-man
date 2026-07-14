package runner

import (
	"context"
	"fmt"
	"os/user"
	"sort"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
)

func newControlBackend(app *bootstrap.Application, window *appwindow.Controller, show func() error) control.Backend {
	return control.Backend{
		State: func(ctx context.Context) (control.State, error) {
			return controlState(ctx, app)
		},
		SaveServer: func(ctx context.Context, server serverdomain.Server) (serverdomain.Server, error) {
			var saved serverdomain.Server
			err := app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
				if server.ID != "" {
					if err := requireServerStopped(ctx, app, server.ID); err != nil {
						return err
					}
				}
				var err error
				saved, err = app.ServerService.Save(ctx, server)
				return err
			})
			return saved, err
		},
		DeleteServer: func(ctx context.Context, serverID string) error {
			return app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
				if err := requireServerStopped(ctx, app, serverID); err != nil {
					return err
				}
				return app.ServerService.Delete(ctx, serverID)
			})
		},
		SaveConfiguration: func(ctx context.Context, configuration configdomain.ConnectionConfiguration) (configdomain.ConnectionConfiguration, error) {
			var saved configdomain.ConnectionConfiguration
			err := app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
				if configuration.ID != "" {
					if err := requireConfigurationStopped(app, configuration.ID); err != nil {
						return err
					}
				}
				var err error
				saved, err = app.ConfigService.Save(ctx, configuration)
				return err
			})
			return saved, err
		},
		DeleteConfiguration: func(ctx context.Context, configurationID string) error {
			return app.SessionService.WithExclusiveMutation(ctx, func(ctx context.Context) error {
				if err := requireConfigurationStopped(app, configurationID); err != nil {
					return err
				}
				return app.ConfigService.Delete(ctx, configurationID)
			})
		},
		Start: func(ctx context.Context, configurationID string) (sessiondomain.RuntimeSession, error) {
			return app.SessionService.Start(ctx, configurationID)
		},
		StartServer: func(ctx context.Context, serverID string) control.BulkResult {
			states, err := app.SessionService.StartAll(ctx, serverID)
			if states == nil {
				states = []sessiondomain.RuntimeSession{}
			}
			result := control.BulkResult{Sessions: states, Failures: []control.Failure{}}
			if err != nil {
				result.Failures = append(result.Failures, control.Failure{ID: serverID, Message: err.Error()})
			}
			return result
		},
		Stop: func(ctx context.Context, configurationID string) (sessiondomain.RuntimeSession, error) {
			return app.SessionService.Stop(ctx, configurationID)
		},
		StopServer: func(ctx context.Context, serverID string) control.BulkResult {
			return stopServer(ctx, app, serverID)
		},
		Retry: func(ctx context.Context, configurationID string) (sessiondomain.RuntimeSession, error) {
			return app.SessionService.Retry(ctx, configurationID)
		},
		Unlock: func(ctx context.Context, configurationID string, secret string) (sessiondomain.RuntimeSession, error) {
			return app.SessionService.SubmitKeyUnlock(ctx, configurationID, secret)
		},
		History: func(ctx context.Context, configurationID string) ([]sessiondomain.SessionHistoryEntry, error) {
			entries, err := app.SessionService.ListHistory(ctx, configurationID)
			if entries == nil {
				entries = []sessiondomain.SessionHistoryEntry{}
			}
			return entries, err
		},
		DiscoverBrowsers: func(ctx context.Context) ([]browser.BrowserOption, error) {
			return app.BrowserService.Discover(ctx)
		},
		PreviewBrowser: func(ctx context.Context, configurationID string, browserID string) (browser.LaunchPreview, error) {
			return app.BrowserService.PreviewLaunchThroughSOCKS(ctx, configurationID, browserID)
		},
		LaunchBrowser: func(ctx context.Context, configurationID string, browserID string) error {
			return app.BrowserService.LaunchThroughSOCKS(ctx, configurationID, browserID)
		},
		SavePreferences: func(ctx context.Context, preferences preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error) {
			return app.PreferencesService.Save(ctx, preferences)
		},
		Show: show,
		Hide: window.Hide,
		Quit: window.Quit,
	}
}

func controlState(ctx context.Context, app *bootstrap.Application) (control.State, error) {
	servers, err := app.ServerService.List(ctx)
	if err != nil {
		return control.State{}, fmt.Errorf("list servers: %w", err)
	}
	preferences, err := app.PreferencesService.Load(ctx)
	if err != nil {
		return control.State{}, fmt.Errorf("load preferences: %w", err)
	}

	records := make([]control.ServerRecord, 0, len(servers))
	for _, server := range servers {
		configurations, err := app.ConfigService.ListByServer(ctx, server.ID)
		if err != nil {
			return control.State{}, fmt.Errorf("list tunnels for %q: %w", server.Name, err)
		}
		if configurations == nil {
			configurations = []configdomain.ConnectionConfiguration{}
		}
		records = append(records, control.ServerRecord{Server: server, Configurations: configurations})
	}
	sessions := app.SessionService.List()
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ConfigurationID < sessions[j].ConfigurationID
	})
	currentUsername := ""
	if currentUser, err := user.Current(); err == nil {
		currentUsername = currentUser.Username
	}

	return control.State{
		Servers:         records,
		Preferences:     preferences,
		Sessions:        sessions,
		Diagnostics:     control.Diagnostics{AppDataPath: app.ConfigDir, DatabasePath: app.DatabasePath},
		CurrentUsername: currentUsername,
	}, nil
}

func requireConfigurationStopped(app *bootstrap.Application, configurationID string) error {
	state, ok := app.SessionService.Get(configurationID)
	if ok && state.Status != sessiondomain.StatusStopped && state.Status != sessiondomain.StatusFailed {
		return fmt.Errorf("stop the tunnel before changing or deleting it")
	}
	return nil
}

func requireServerStopped(ctx context.Context, app *bootstrap.Application, serverID string) error {
	configurations, err := app.ConfigService.ListByServer(ctx, serverID)
	if err != nil {
		return err
	}
	for _, configuration := range configurations {
		if err := requireConfigurationStopped(app, configuration.ID); err != nil {
			return fmt.Errorf("stop %q before changing or deleting the server", configuration.Label)
		}
	}
	return nil
}

func stopServer(ctx context.Context, app *bootstrap.Application, serverID string) control.BulkResult {
	result := control.BulkResult{Sessions: []sessiondomain.RuntimeSession{}, Failures: []control.Failure{}}
	configurations, err := app.ConfigService.ListByServer(ctx, serverID)
	if err != nil {
		result.Failures = append(result.Failures, control.Failure{ID: serverID, Message: err.Error()})
		return result
	}
	for _, configuration := range configurations {
		state, ok := app.SessionService.Get(configuration.ID)
		if !ok || state.Status == sessiondomain.StatusStopped {
			continue
		}
		stopped, err := app.SessionService.Stop(ctx, configuration.ID)
		if err != nil {
			result.Failures = append(result.Failures, control.Failure{ID: configuration.ID, Label: configuration.Label, Message: err.Error()})
			continue
		}
		result.Sessions = append(result.Sessions, stopped)
	}
	return result
}
