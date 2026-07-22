package bindings

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
)

type AppBindings struct {
	app                 *bootstrap.Application
	window              *appwindow.Controller
	setBrowserShortcuts func(string, string) error
	showBrowserSwitcher func() bool
}

func (a *AppBindings) SetBrowserShortcutsRegistrar(registrar func(string, string) error) {
	a.setBrowserShortcuts = registrar
}

func (a *AppBindings) SetBrowserSwitcherPresenter(presenter func() bool) {
	a.showBrowserSwitcher = presenter
}

type ServerWithConfigurations struct {
	Server         serverdomain.Server                    `json:"server"`
	Configurations []configdomain.ConnectionConfiguration `json:"configurations"`
}

type Diagnostics struct {
	AppDataPath  string `json:"appDataPath"`
	DatabasePath string `json:"databasePath"`
}

type SSHKeyOption struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type LoadInitialStateResult struct {
	Servers         []ServerWithConfigurations       `json:"servers"`
	Preferences     preferencesdomain.UserPreference `json:"preferences"`
	Sessions        []any                            `json:"sessions"`
	SSHKeys         []SSHKeyOption                   `json:"sshKeys"`
	Diagnostics     Diagnostics                      `json:"diagnostics"`
	CurrentUsername string                           `json:"currentUsername,omitempty"`
	Message         string                           `json:"message,omitempty"`
	Recoverable     bool                             `json:"recoverable,omitempty"`
}

func NewAppBindings() (*AppBindings, error) {
	return NewAppBindingsWithWindow(appwindow.New())
}

func NewAppBindingsWithWindow(window *appwindow.Controller) (*AppBindings, error) {
	if window == nil {
		window = appwindow.New()
	}
	app, err := bootstrap.New(context.Background())
	if err != nil {
		return nil, err
	}
	return NewAppBindingsWithApplication(app, window), nil
}

func NewAppBindingsWithApplication(app *bootstrap.Application, window *appwindow.Controller) *AppBindings {
	if window == nil {
		window = appwindow.New()
	}
	return &AppBindings{app: app, window: window}
}

func (a *AppBindings) SetContext(ctx context.Context) {
	a.window.SetContext(ctx)
}

func (a *AppBindings) Shutdown(ctx context.Context) error {
	return a.app.Shutdown(ctx)
}

// HideWindow hides the application UI without stopping active SSH sessions.
func (a *AppBindings) HideWindow() error {
	return a.window.Hide()
}

func (a *AppBindings) ShowBrowserSwitcher() error {
	if a.showBrowserSwitcher == nil || !a.showBrowserSwitcher() {
		return fmt.Errorf("browser switcher window is unavailable")
	}
	return nil
}

// Quit exits through the Wails lifecycle so OnShutdown can stop sessions and
// close the database cleanly.
func (a *AppBindings) Quit() error {
	return a.window.Quit()
}

func (a *AppBindings) storageError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s. App data: %s. Database: %s. %w", action, a.app.ConfigDir, a.app.DatabasePath, err)
}

func (a *AppBindings) LoadInitialState() (LoadInitialStateResult, error) {
	ctx := context.Background()
	currentUsername := ""
	if currentUser, err := user.Current(); err == nil {
		currentUsername = currentUser.Username
	}
	result := LoadInitialStateResult{
		Preferences:     preferencesdomain.Default(),
		Sessions:        []any{},
		SSHKeys:         discoverSSHKeyOptions(),
		Diagnostics:     Diagnostics{AppDataPath: a.app.ConfigDir, DatabasePath: a.app.DatabasePath},
		CurrentUsername: currentUsername,
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
		Servers:         items,
		Preferences:     pref,
		Sessions:        sessions,
		SSHKeys:         result.SSHKeys,
		Diagnostics:     result.Diagnostics,
		CurrentUsername: currentUsername,
		Message:         message,
		Recoverable:     prefErr != nil,
	}, nil
}

func discoverSSHKeyOptions() []SSHKeyOption {
	home, err := os.UserHomeDir()
	if err != nil {
		return []SSHKeyOption{}
	}
	paths, err := auth.DiscoverPrivateKeys(filepath.Join(home, ".ssh"))
	if err != nil {
		return []SSHKeyOption{}
	}

	options := make([]SSHKeyOption, 0, len(paths))
	for _, path := range paths {
		options = append(options, SSHKeyOption{Name: filepath.Base(path), Path: path})
	}
	return options
}
