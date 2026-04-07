package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
	"ssh-man/internal/platform/paths"
	"ssh-man/internal/sqlite"
)

type Application struct {
	ConfigDir          string
	DatabasePath       string
	DB                 *sql.DB
	ServerService      *serverdomain.Service
	ConfigService      *configdomain.Service
	PreferencesService *preferencesdomain.Service
	SessionService     *sessiondomain.Service
	BrowserService     *browser.Service
}

func New(context.Context) (*Application, error) {
	configDir, err := paths.ConfigDir()
	if err != nil {
		return nil, err
	}
	db, err := sqlite.OpenDatabase(configDir)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	serverStore := sqlite.NewServerStore(db)
	configStore := sqlite.NewConfigStore(db)
	prefStore := sqlite.NewPreferencesStore(db)
	historyStore := sqlite.NewSessionHistoryStore(db)
	runtimeStore := sessiondomain.NewRuntimeStore()

	serverService := serverdomain.NewService(serverStore)
	configService := configdomain.NewService(configStore)
	preferencesService := preferencesdomain.NewService(prefStore)
	sessionService := sessiondomain.NewService(configStore, serverStore, historyStore, runtimeStore)
	browserService := browser.NewService(configStore, runtimeStore)

	return &Application{
		ConfigDir:          configDir,
		DatabasePath:       paths.DatabasePath(configDir),
		DB:                 db,
		ServerService:      serverService,
		ConfigService:      configService,
		PreferencesService: preferencesService,
		SessionService:     sessionService,
		BrowserService:     browserService,
	}, nil
}

func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}

	for _, state := range a.SessionService.List() {
		if state.Status == sessiondomain.StatusStopped {
			continue
		}
		_, _ = a.SessionService.Stop(ctx, state.ConfigurationID)
	}

	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			return fmt.Errorf("close database %q: %w", a.DatabasePath, err)
		}
	}

	return nil
}
