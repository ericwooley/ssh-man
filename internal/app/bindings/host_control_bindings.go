package bindings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
)

type hostControlClient interface {
	Call(context.Context, control.Request, any) error
}

func (bindings *HostBindings) callControl(request control.Request, output any) error {
	if bindings.control == nil {
		return fmt.Errorf("the main SSH Man process is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), hostOpenTimeout)
	defer cancel()
	if err := bindings.control.Call(ctx, request, output); err != nil {
		var protocolMismatch *control.ProtocolMismatchError
		if errors.As(err, &protocolMismatch) {
			return fmt.Errorf("restart SSH Man to manage tunnels after this update")
		}
		return err
	}
	return nil
}

func (bindings *HostBindings) controllerState() (control.State, error) {
	var state control.State
	if err := bindings.callControl(control.Request{Command: "state"}, &state); err != nil {
		return control.State{}, fmt.Errorf("load live server details: %w", err)
	}
	return state, nil
}

func (bindings *HostBindings) scopedControllerState() (control.State, control.ServerRecord, error) {
	state, err := bindings.controllerState()
	if err != nil {
		return control.State{}, control.ServerRecord{}, err
	}
	serverID := bindings.currentServerID()
	for _, record := range state.Servers {
		if record.Server.ID == serverID {
			return state, record, nil
		}
	}
	return control.State{}, control.ServerRecord{}, fmt.Errorf("the selected server is no longer available")
}

func managedSOCKSConfigurationForHost(server serverdomain.Server) configdomain.ConnectionConfiguration {
	return configdomain.ConnectionConfiguration{
		ID:                   configdomain.ManagedSOCKSConfigurationID(server.ID),
		ServerID:             server.ID,
		Label:                "Browser proxy",
		ConnectionType:       configdomain.ConnectionTypeSOCKSProxy,
		SocksPort:            server.SocksPort,
		AutoReconnectEnabled: true,
	}
}

func scopedHostConfigurations(record control.ServerRecord) []configdomain.ConnectionConfiguration {
	configurations := make([]configdomain.ConnectionConfiguration, 0, len(record.Configurations)+1)
	configurations = append(configurations, managedSOCKSConfigurationForHost(record.Server))
	for _, configuration := range record.Configurations {
		if configdomain.IsManagedSOCKSConfigurationID(configuration.ID) {
			continue
		}
		configurations = append(configurations, configuration)
	}
	return configurations
}

func scopedHostSessions(
	state control.State,
	configurations []configdomain.ConnectionConfiguration,
) []sessiondomain.RuntimeSession {
	configurationIDs := make(map[string]struct{}, len(configurations))
	for _, configuration := range configurations {
		configurationIDs[configuration.ID] = struct{}{}
	}
	sessions := make([]sessiondomain.RuntimeSession, 0, len(configurations))
	for _, session := range state.Sessions {
		if _, ok := configurationIDs[session.ConfigurationID]; ok {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func (bindings *HostBindings) LoadAppState() (LoadInitialStateResult, error) {
	state, record, err := bindings.scopedControllerState()
	if err != nil {
		return LoadInitialStateResult{}, err
	}
	configurations := scopedHostConfigurations(record)
	sessions := scopedHostSessions(state, configurations)
	sessionValues := make([]any, 0, len(sessions))
	for _, session := range sessions {
		sessionValues = append(sessionValues, session)
	}
	return LoadInitialStateResult{
		Servers: []ServerWithConfigurations{{
			Server:         record.Server,
			Configurations: configurations,
		}},
		Preferences: state.Preferences,
		Sessions:    sessionValues,
		SSHKeys:     discoverSSHKeyOptions(),
		Diagnostics: Diagnostics{
			AppDataPath:  state.Diagnostics.AppDataPath,
			DatabasePath: state.Diagnostics.DatabasePath,
		},
		CurrentUsername: state.CurrentUsername,
		Message:         state.Message,
		Recoverable:     state.Recoverable,
	}, nil
}

func (bindings *HostBindings) ListRuntimeSessions() ([]sessiondomain.RuntimeSession, error) {
	state, record, err := bindings.scopedControllerState()
	if err != nil {
		return nil, err
	}
	return scopedHostSessions(state, scopedHostConfigurations(record)), nil
}

func (bindings *HostBindings) configurationBelongsToHost(configurationID string) error {
	_, record, err := bindings.scopedControllerState()
	if err != nil {
		return err
	}
	for _, configuration := range scopedHostConfigurations(record) {
		if configuration.ID == configurationID {
			return nil
		}
	}
	return fmt.Errorf("the selected tunnel does not belong to this server")
}

func (bindings *HostBindings) SaveServer(input serverdomain.Server) (serverdomain.Server, error) {
	if input.ID != bindings.currentServerID() {
		return serverdomain.Server{}, fmt.Errorf("the selected server cannot be changed from this window")
	}
	var saved serverdomain.Server
	if err := bindings.callControl(control.Request{Command: "server.save", Server: &input}, &saved); err != nil {
		return serverdomain.Server{}, err
	}
	bindings.replaceServer(saved)
	return saved, nil
}

func (bindings *HostBindings) replaceServer(server serverdomain.Server) {
	remoteFor := bindings.remoteFor
	var nextDiscoverer portDiscoverer
	var nextForwarder portForwarder
	if remoteFor != nil {
		nextDiscoverer, nextForwarder = remoteFor(server)
	}
	bindings.mu.Lock()
	previousForwarder := bindings.forwarder
	bindings.server = server
	bindings.passphrase = ""
	bindings.available = map[int][]string{}
	if remoteFor != nil {
		bindings.discoverer = nextDiscoverer
		bindings.forwarder = nextForwarder
	}
	bindings.mu.Unlock()
	if remoteFor != nil && previousForwarder != nil {
		_ = previousForwarder.Close()
	}
}

func (bindings *HostBindings) DeleteServer(serverID string) error {
	if serverID != bindings.currentServerID() {
		return fmt.Errorf("the selected server cannot be deleted from this window")
	}
	return bindings.callControl(control.Request{Command: "server.delete", ServerID: serverID}, nil)
}

func (bindings *HostBindings) SaveConnectionConfiguration(
	input configdomain.ConnectionConfiguration,
) (configdomain.ConnectionConfiguration, error) {
	serverID := bindings.currentServerID()
	if input.ID != "" {
		if err := bindings.configurationBelongsToHost(input.ID); err != nil {
			return configdomain.ConnectionConfiguration{}, err
		}
	}
	input.ServerID = serverID
	var saved configdomain.ConnectionConfiguration
	if err := bindings.callControl(control.Request{
		Command:       "configuration.save",
		Configuration: &input,
	}, &saved); err != nil {
		return configdomain.ConnectionConfiguration{}, err
	}
	return saved, nil
}

func (bindings *HostBindings) DeleteConnectionConfiguration(configurationID string) error {
	if err := bindings.configurationBelongsToHost(configurationID); err != nil {
		return err
	}
	return bindings.callControl(control.Request{
		Command:         "configuration.delete",
		ConfigurationID: configurationID,
	}, nil)
}

func (bindings *HostBindings) StartConfiguration(configurationID string) (sessiondomain.RuntimeSession, error) {
	if err := bindings.configurationBelongsToHost(configurationID); err != nil {
		return sessiondomain.RuntimeSession{}, err
	}
	var session sessiondomain.RuntimeSession
	if err := bindings.callControl(control.Request{
		Command:         "session.start",
		ConfigurationID: configurationID,
	}, &session); err != nil {
		return sessiondomain.RuntimeSession{}, err
	}
	return session, nil
}

func (bindings *HostBindings) StartServerConfigurations(serverID string) ([]sessiondomain.RuntimeSession, error) {
	if serverID != bindings.currentServerID() {
		return nil, fmt.Errorf("the selected server cannot be started from this window")
	}
	var result control.BulkResult
	if err := bindings.callControl(control.Request{
		Command:  "session.start_server",
		ServerID: serverID,
	}, &result); err != nil {
		return nil, err
	}
	if len(result.Failures) > 0 {
		messages := make([]string, 0, len(result.Failures))
		for _, failure := range result.Failures {
			messages = append(messages, failure.Message)
		}
		return result.Sessions, errors.New(strings.Join(messages, ". "))
	}
	return result.Sessions, nil
}

func (bindings *HostBindings) StopConfiguration(configurationID string) (sessiondomain.RuntimeSession, error) {
	return bindings.controlSessionAction("session.stop", configurationID)
}

func (bindings *HostBindings) RetryConfiguration(configurationID string) (sessiondomain.RuntimeSession, error) {
	return bindings.controlSessionAction("session.retry", configurationID)
}

func (bindings *HostBindings) controlSessionAction(
	command string,
	configurationID string,
) (sessiondomain.RuntimeSession, error) {
	if err := bindings.configurationBelongsToHost(configurationID); err != nil {
		return sessiondomain.RuntimeSession{}, err
	}
	var session sessiondomain.RuntimeSession
	if err := bindings.callControl(control.Request{
		Command:         command,
		ConfigurationID: configurationID,
	}, &session); err != nil {
		return sessiondomain.RuntimeSession{}, err
	}
	return session, nil
}

func (bindings *HostBindings) SubmitKeyUnlock(
	configurationID string,
	secret string,
) (sessiondomain.RuntimeSession, error) {
	if err := bindings.configurationBelongsToHost(configurationID); err != nil {
		return sessiondomain.RuntimeSession{}, err
	}
	var session sessiondomain.RuntimeSession
	if err := bindings.callControl(control.Request{
		Command:         "session.unlock",
		ConfigurationID: configurationID,
		Secret:          secret,
	}, &session); err != nil {
		return sessiondomain.RuntimeSession{}, err
	}
	return session, nil
}

func (bindings *HostBindings) ListSessionHistory(
	configurationID string,
) ([]sessiondomain.SessionHistoryEntry, error) {
	if err := bindings.configurationBelongsToHost(configurationID); err != nil {
		return nil, err
	}
	var history []sessiondomain.SessionHistoryEntry
	if err := bindings.callControl(control.Request{
		Command:         "session.history",
		ConfigurationID: configurationID,
	}, &history); err != nil {
		return nil, err
	}
	if history == nil {
		history = []sessiondomain.SessionHistoryEntry{}
	}
	return history, nil
}

func (bindings *HostBindings) DiscoverBrowsers() ([]browser.BrowserOption, error) {
	var browsers []browser.BrowserOption
	if err := bindings.callControl(control.Request{Command: "browser.list"}, &browsers); err != nil {
		return nil, err
	}
	if browsers == nil {
		browsers = []browser.BrowserOption{}
	}
	return browsers, nil
}

func (bindings *HostBindings) PreviewBrowserLaunchThroughSocks(
	configurationID string,
	browserID string,
) (browser.LaunchPreview, error) {
	if err := bindings.configurationBelongsToHost(configurationID); err != nil {
		return browser.LaunchPreview{}, err
	}
	var preview browser.LaunchPreview
	if err := bindings.callControl(control.Request{
		Command:         "browser.preview",
		ConfigurationID: configurationID,
		BrowserID:       browserID,
	}, &preview); err != nil {
		return browser.LaunchPreview{}, err
	}
	return preview, nil
}

func (bindings *HostBindings) LaunchBrowserThroughSocks(configurationID string, browserID string) error {
	if err := bindings.configurationBelongsToHost(configurationID); err != nil {
		return err
	}
	return bindings.callControl(control.Request{
		Command:         "browser.launch",
		ConfigurationID: configurationID,
		BrowserID:       browserID,
	}, nil)
}

var _ hostControlClient = (*control.Client)(nil)
