package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	commandhistorydomain "ssh-man/internal/domain/commandhistory"
	configdomain "ssh-man/internal/domain/config"
	portlinkdomain "ssh-man/internal/domain/portlink"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	urlroutingdomain "ssh-man/internal/domain/urlrouting"
	"ssh-man/internal/platform/browser"
	"ssh-man/internal/platform/defaultbrowser"
	"ssh-man/internal/platform/paths"
	"ssh-man/internal/sqlite"
)

type Application struct {
	ConfigDir             string
	DatabasePath          string
	DB                    *sql.DB
	ServerService         *serverdomain.Service
	ConfigService         *configdomain.Service
	PortLinkService       *portlinkdomain.Service
	PreferencesService    *preferencesdomain.Service
	SessionService        *sessiondomain.Service
	BrowserService        *browser.Service
	CommandHistoryService *commandhistorydomain.Service
	URLRoutingService     *urlroutingdomain.Service
	DefaultBrowser        *defaultbrowser.Manager
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
	portLinkStore := sqlite.NewPortLinkStore(db)
	prefStore := sqlite.NewPreferencesStore(db)
	historyStore := sqlite.NewSessionHistoryStore(db)
	commandHistoryStore := sqlite.NewCommandHistoryStore(db)
	runtimeStore := sessiondomain.NewRuntimeStore()

	serverService := serverdomain.NewService(serverStore)
	configService := configdomain.NewService(configStore)
	portLinkService := portlinkdomain.NewService(portLinkStore)
	serverService.SetSOCKSPortAvailabilityCheck(configService.ValidateManagedSOCKSPort)
	if err := serverService.EnsureSOCKSPorts(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("assign server SOCKS ports: %w", err)
	}
	servers, err := serverService.List(context.Background())
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("list servers for automatic browser proxies: %w", err)
	}
	for _, server := range servers {
		if _, err := configService.EnsureManagedSOCKSConfiguration(context.Background(), server); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ensure automatic browser proxy for %q: %w", server.Name, err)
		}
	}
	preferencesService := preferencesdomain.NewService(prefStore)
	sessionService := sessiondomain.NewService(configStore, serverStore, historyStore, runtimeStore)
	browserService := browser.NewServiceWithSessions(configDir, configStore, sessionService, serverStore, preferencesService)
	urlRoutingService := urlroutingdomain.NewService(
		preferencesService,
		configService,
		serverService,
		sessionService,
		urlRoutingBrowserAdapter{service: browserService},
	)
	defaultBrowserManager := defaultbrowser.NewManager()
	commandHistoryService := commandhistorydomain.NewService(commandHistoryStore)

	return &Application{
		ConfigDir:             configDir,
		DatabasePath:          paths.DatabasePath(configDir),
		DB:                    db,
		ServerService:         serverService,
		ConfigService:         configService,
		PortLinkService:       portLinkService,
		PreferencesService:    preferencesService,
		SessionService:        sessionService,
		BrowserService:        browserService,
		CommandHistoryService: commandHistoryService,
		URLRoutingService:     urlRoutingService,
		DefaultBrowser:        defaultBrowserManager,
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
