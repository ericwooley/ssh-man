package browser

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

type BrowserEngine string

const (
	BrowserEngineChromium BrowserEngine = "chromium"
	BrowserEngineFirefox  BrowserEngine = "firefox"
	BrowserEngineRegular  BrowserEngine = "regular"
)

type BrowserOption struct {
	ID                  string        `json:"id"`
	DisplayName         string        `json:"displayName"`
	LaunchReference     string        `json:"-"`
	ExecutableReference string        `json:"-"`
	Engine              BrowserEngine `json:"engine"`
	SupportsProxyLaunch bool          `json:"supportsProxyLaunch"`
	Custom              bool          `json:"custom"`
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

type PreferenceLookup interface {
	Load(ctx context.Context) (preferencesdomain.UserPreference, error)
}

type Service struct {
	appDataDir     string
	configs        ConfigLookup
	runtimes       RuntimeLookup
	discover       func(context.Context) ([]BrowserOption, error)
	discoverCustom func([]preferencesdomain.CustomBrowser) []BrowserOption
	preferences    PreferenceLookup
	servers        ServerLookup
	listRunning    func(context.Context, string, []BrowserOption, []serverdomain.Server) ([]RunningTarget, error)
	activate       func(int) error
	launchProxy    func(string, string, BrowserOption, int, string) error
	openURL        func(BrowserOption, string) error
}

func NewService(appDataDir string, configs ConfigLookup, runtimes RuntimeLookup, servers ServerLookup, preferenceLookups ...PreferenceLookup) *Service {
	var preferences PreferenceLookup
	if len(preferenceLookups) > 0 {
		preferences = preferenceLookups[0]
	}
	return &Service{
		appDataDir:     appDataDir,
		configs:        configs,
		runtimes:       runtimes,
		servers:        servers,
		preferences:    preferences,
		discoverCustom: discoverCustomBrowsers,
		discover: func(context.Context) ([]BrowserOption, error) {
			return discoverBrowsers()
		},
		listRunning: listRunningBrowserTargets,
		activate:    activateRunningBrowser,
		launchProxy: launchBrowser,
		openURL:     openBrowserURL,
	}
}

func (s *Service) Discover(ctx context.Context) ([]BrowserOption, error) {
	builtIns, err := s.discover(ctx)
	if err != nil {
		return nil, err
	}
	if s.preferences == nil {
		return builtIns, nil
	}
	pref, err := s.preferences.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load custom browsers: %w", err)
	}
	return mergeBrowserOptions(builtIns, s.discoverCustom(pref.CustomBrowsers)), nil
}

func mergeBrowserOptions(builtIns, custom []BrowserOption) []BrowserOption {
	result := make([]BrowserOption, 0, len(builtIns)+len(custom))
	ids := make(map[string]struct{}, len(builtIns)+len(custom))
	launchReferences := make(map[string]struct{}, len(builtIns)+len(custom))
	appendUnique := func(option BrowserOption) {
		id := strings.TrimSpace(option.ID)
		launchReference := strings.TrimSpace(option.LaunchReference)
		if id == "" || launchReference == "" {
			return
		}
		if _, exists := ids[id]; exists {
			return
		}
		if _, exists := launchReferences[launchReference]; exists {
			return
		}
		option.ID = id
		option.LaunchReference = launchReference
		ids[id] = struct{}{}
		launchReferences[launchReference] = struct{}{}
		result = append(result, option)
	}
	for _, option := range builtIns {
		appendUnique(option)
	}
	for _, option := range custom {
		appendUnique(option)
	}
	return result
}

func browserEngine(option BrowserOption) BrowserEngine {
	if option.Engine != "" {
		return option.Engine
	}
	if option.ID == "firefox" || option.ID == "zen" {
		return BrowserEngineFirefox
	}
	if option.ID == "safari" {
		return BrowserEngineRegular
	}
	return BrowserEngineChromium
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
	return s.launchThroughSOCKS(ctx, configurationID, browserID, "")
}

func (s *Service) LaunchThroughSOCKSURL(ctx context.Context, configurationID string, browserID string, rawURL string) error {
	if err := validateWebURL(rawURL); err != nil {
		return err
	}
	return s.launchThroughSOCKS(ctx, configurationID, browserID, rawURL)
}

func (s *Service) launchThroughSOCKS(ctx context.Context, configurationID string, browserID string, rawURL string) error {
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
	option, ok := selectBrowser(browsers, browserID, true)
	if ok {
		if runtimeState.BoundPort < 1 {
			return fmt.Errorf("the SOCKS tunnel is connected, but its local port is unavailable")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return s.launchProxy(s.appDataDir, configuration.ServerID, option, runtimeState.BoundPort, rawURL)
	}

	return fmt.Errorf("the selected browser is no longer available")
}

func (s *Service) OpenURL(ctx context.Context, browserID string, rawURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateWebURL(rawURL); err != nil {
		return err
	}
	browsers, err := s.Discover(ctx)
	if err != nil {
		return err
	}
	option, ok := selectBrowser(browsers, browserID, false)
	if !ok {
		return fmt.Errorf("the selected browser is no longer available")
	}
	return s.openURL(option, rawURL)
}

func selectBrowser(browsers []BrowserOption, browserID string, proxyOnly bool) (BrowserOption, bool) {
	if browserID != "" {
		for _, option := range browsers {
			if option.ID == browserID && (!proxyOnly || option.SupportsProxyLaunch) {
				return option, true
			}
		}
		return BrowserOption{}, false
	}
	for _, option := range browsers {
		if !proxyOnly || option.SupportsProxyLaunch {
			return option, true
		}
	}
	return BrowserOption{}, false
}

func validateWebURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must use http or https")
	}
	if parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("URL must include a host and must not include credentials")
	}
	return nil
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
