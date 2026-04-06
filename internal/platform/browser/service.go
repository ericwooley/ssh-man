package browser

import (
	"context"
	"fmt"

	configdomain "ssh-man/internal/domain/config"
	sessiondomain "ssh-man/internal/domain/session"
)

type BrowserOption struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"displayName"`
	LaunchReference     string `json:"-"`
	SupportsProxyLaunch bool   `json:"supportsProxyLaunch"`
}

type RuntimeLookup interface {
	Get(id string) (sessiondomain.RuntimeSession, bool)
}

type ConfigLookup interface {
	Get(ctx context.Context, id string) (configdomain.ConnectionConfiguration, error)
}

type Service struct {
	configs  ConfigLookup
	runtimes RuntimeLookup
}

func NewService(configs ConfigLookup, runtimes RuntimeLookup) *Service {
	return &Service{configs: configs, runtimes: runtimes}
}

func (s *Service) Discover(context.Context) ([]BrowserOption, error) {
	return discoverBrowsers()
}

func (s *Service) LaunchThroughSOCKS(ctx context.Context, configurationID string, browserID string) error {
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
		return launchBrowser(option, configuration.SocksPort)
	}

	return fmt.Errorf("the selected browser is no longer available")
}
