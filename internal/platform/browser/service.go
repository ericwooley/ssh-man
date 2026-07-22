package browser

import (
	"context"
	"fmt"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

type BrowserOption struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"displayName"`
	LaunchReference     string `json:"-"`
	SupportsProxyLaunch bool   `json:"supportsProxyLaunch"`
}

type LaunchPreview struct {
	BrowserID       string `json:"browserId"`
	BrowserName     string `json:"browserName"`
	Command         string `json:"command"`
	Supported       bool   `json:"supported"`
	ConfigurationID string `json:"configurationId"`
}

type RunningTargetKind string

const (
	RunningTargetProxy   RunningTargetKind = "proxy"
	RunningTargetRegular RunningTargetKind = "regular"
)

type RunningTarget struct {
	ID          string            `json:"id"`
	PID         int               `json:"pid"`
	BrowserID   string            `json:"browserId"`
	BrowserName string            `json:"browserName"`
	Kind        RunningTargetKind `json:"kind"`
	ServerID    string            `json:"serverId,omitempty"`
	ServerName  string            `json:"serverName,omitempty"`
}

type RuntimeLookup interface {
	Get(id string) (sessiondomain.RuntimeSession, bool)
}

type ConfigLookup interface {
	Get(ctx context.Context, id string) (configdomain.ConnectionConfiguration, error)
}

type ServerLookup interface {
	List(ctx context.Context) ([]serverdomain.Server, error)
}

type Service struct {
	appDataDir  string
	configs     ConfigLookup
	runtimes    RuntimeLookup
	discover    func(context.Context) ([]BrowserOption, error)
	servers     ServerLookup
	listRunning func(context.Context, string, []BrowserOption, []serverdomain.Server) ([]RunningTarget, error)
	activate    func(int) error
}

func NewService(appDataDir string, configs ConfigLookup, runtimes RuntimeLookup, servers ServerLookup) *Service {
	return &Service{
		appDataDir: appDataDir,
		configs:    configs,
		runtimes:   runtimes,
		servers:    servers,
		discover: func(context.Context) ([]BrowserOption, error) {
			return discoverBrowsers()
		},
		listRunning: listRunningBrowserTargets,
		activate:    activateRunningBrowser,
	}
}

func (s *Service) Discover(ctx context.Context) ([]BrowserOption, error) {
	return s.discover(ctx)
}

func (s *Service) ListRunning(ctx context.Context) ([]RunningTarget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	browsers, err := s.Discover(ctx)
	if err != nil {
		return nil, err
	}
	servers := []serverdomain.Server{}
	if s.servers != nil {
		servers, err = s.servers.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list servers for running browsers: %w", err)
		}
	}
	return s.listRunning(ctx, s.appDataDir, browsers, servers)
}

func (s *Service) ActivateRunning(ctx context.Context, targetID string) error {
	targets, err := s.ListRunning(ctx)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if target.ID == targetID {
			return s.activate(target.PID)
		}
	}
	return fmt.Errorf("the selected browser is no longer running")
}

func (s *Service) LaunchThroughSOCKS(ctx context.Context, configurationID string, browserID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runtimeState, ok := s.runtimes.Get(configurationID)
	if !ok || runtimeState.Status != sessiondomain.StatusConnected {
		return fmt.Errorf("start the socks configuration before launching a browser")
	}

	configuration, err := s.configs.Get(ctx, configurationID)
	if err != nil {
		return fmt.Errorf("load socks configuration: %w", err)
	}
	if configuration.ConnectionType != configdomain.ConnectionTypeSOCKSProxy {
		return fmt.Errorf("browser launch is only available for socks configurations")
	}

	browsers, err := s.Discover(ctx)
	if err != nil {
		return err
	}
	for _, option := range browsers {
		if option.ID != browserID {
			continue
		}
		if runtimeState.BoundPort < 1 {
			return fmt.Errorf("the SOCKS tunnel is connected, but its local port is unavailable")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return launchBrowser(s.appDataDir, configuration.ServerID, option, runtimeState.BoundPort)
	}

	return fmt.Errorf("the selected browser is no longer available")
}

func (s *Service) PreviewLaunchThroughSOCKS(ctx context.Context, configurationID string, browserID string) (LaunchPreview, error) {
	if err := ctx.Err(); err != nil {
		return LaunchPreview{}, err
	}
	runtimeState, ok := s.runtimes.Get(configurationID)
	if !ok || runtimeState.Status != sessiondomain.StatusConnected {
		return LaunchPreview{}, fmt.Errorf("start the socks configuration before launching a browser")
	}

	configuration, err := s.configs.Get(ctx, configurationID)
	if err != nil {
		return LaunchPreview{}, fmt.Errorf("load socks configuration: %w", err)
	}
	if configuration.ConnectionType != configdomain.ConnectionTypeSOCKSProxy {
		return LaunchPreview{}, fmt.Errorf("browser launch is only available for socks configurations")
	}
	if runtimeState.BoundPort < 1 {
		return LaunchPreview{}, fmt.Errorf("the SOCKS tunnel is connected, but its local port is unavailable")
	}

	browsers, err := s.Discover(ctx)
	if err != nil {
		return LaunchPreview{}, err
	}
	for _, option := range browsers {
		if option.ID != browserID {
			continue
		}
		return LaunchPreview{
			BrowserID:       option.ID,
			BrowserName:     option.DisplayName,
			Command:         previewLaunchCommand(s.appDataDir, configuration.ServerID, option, runtimeState.BoundPort),
			Supported:       option.SupportsProxyLaunch,
			ConfigurationID: configurationID,
		}, nil
	}

	return LaunchPreview{}, fmt.Errorf("the selected browser is no longer available")
}
