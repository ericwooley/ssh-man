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
		DB:                 db,
		ServerService:      serverService,
		ConfigService:      configService,
		PreferencesService: preferencesService,
		SessionService:     sessionService,
		BrowserService:     browserService,
	}, nil
}
