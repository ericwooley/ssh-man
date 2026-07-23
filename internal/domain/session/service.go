package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
	"ssh-man/internal/ssh/tunnel"
)

type ConfigStore interface {
	Get(ctx context.Context, id string) (configdomain.ConnectionConfiguration, error)
	ListByServer(ctx context.Context, serverID string) ([]configdomain.ConnectionConfiguration, error)
	ListAll(ctx context.Context) ([]configdomain.ConnectionConfiguration, error)
}

type ServerStore interface {
	Get(ctx context.Context, id string) (serverdomain.Server, error)
}

type HistoryStore interface {
	Add(ctx context.Context, entry SessionHistoryEntry) error
	ListByConfiguration(ctx context.Context, configurationID string) ([]SessionHistoryEntry, error)
}

type Factory interface {
	New(server serverdomain.Server, config configdomain.ConnectionConfiguration, passphrase string, onDisconnect func(error)) Runner
}

type tunnelFactory struct{}

func (t tunnelFactory) New(server serverdomain.Server, config configdomain.ConnectionConfiguration, passphrase string, onDisconnect func(error)) Runner {
	return tunnel.NewSession(server, config, passphrase, onDisconnect)
}

type Service struct {
	configs          ConfigStore
	servers          ServerStore
	history          HistoryStore
	runtimes         *RuntimeStore
	factory          Factory
	reconnectDelay   func(int) time.Duration
	reconnectTimeout time.Duration
	operationGate    sync.RWMutex
	operationLocks   sync.Map
}

func NewService(configs ConfigStore, servers ServerStore, history HistoryStore, runtimes *RuntimeStore) *Service {
	return &Service{
		configs:          configs,
		servers:          servers,
		history:          history,
		runtimes:         runtimes,
		factory:          tunnelFactory{},
		reconnectDelay:   nextReconnectDelay,
		reconnectTimeout: 15 * time.Second,
	}
}

func (s *Service) SetFactory(factory Factory) {
	if factory != nil {
		s.factory = factory
	}
}

func (s *Service) List() []RuntimeSession {
	return s.runtimes.List()
}

func (s *Service) Get(id string) (RuntimeSession, bool) {
	return s.runtimes.Get(id)
}

func (s *Service) Snapshot() []RuntimeSession {
	return s.runtimes.List()
}

func (s *Service) ListHistory(ctx context.Context, configurationID string) ([]SessionHistoryEntry, error) {
	if s.history == nil {
		return nil, nil
	}

	entries, err := s.history.ListByConfiguration(ctx, configurationID)
	if err != nil {
		return nil, fmt.Errorf("load session history: %w", err)
	}

	return entries, nil
}

func (s *Service) Start(ctx context.Context, configurationID string) (RuntimeSession, error) {
	state, _, err := s.startIfInactive(ctx, configurationID)
	return state, err
}

func (s *Service) Retry(ctx context.Context, configurationID string) (RuntimeSession, error) {
	if err := s.acquireRuntimeOperation(ctx); err != nil {
		return RuntimeSession{}, err
	}
	defer s.operationGate.RUnlock()

	operationLock := s.operationLock(configurationID)
	operationLock.Lock()
	defer operationLock.Unlock()

	_, passphrase, _ := s.runtimes.Runner(configurationID)
	return s.start(ctx, configurationID, passphrase)
}

func (s *Service) StartAll(ctx context.Context, serverID string) ([]RuntimeSession, error) {
	configurations, err := s.configs.ListByServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("load server configurations: %w", err)
	}

	states := make([]RuntimeSession, 0, len(configurations))
	startErrors := make([]error, 0)
	for _, configuration := range configurations {
		if configdomain.IsManagedSOCKSConfigurationID(configuration.ID) {
			continue
		}
		state, started, err := s.startIfInactive(ctx, configuration.ID)
		if err != nil {
			startErrors = append(startErrors, fmt.Errorf("%s: %w", configuration.Label, err))
			continue
		}
		if started {
			states = append(states, state)
		}
	}
	return states, errors.Join(startErrors...)
}

func (s *Service) StartOnLaunch(ctx context.Context) error {
	configurations, err := s.configs.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("load start-on-launch configurations: %w", err)
	}

	startErrors := make([]error, 0)
	for _, configuration := range configurationsStartingOnLaunch(configurations) {
		if _, _, err := s.startIfInactive(ctx, configuration.ID); err != nil {
			startErrors = append(startErrors, fmt.Errorf("%s: %w", configuration.Label, err))
		}
	}
	return errors.Join(startErrors...)
}

func (s *Service) StartManagedSOCKSProxies(ctx context.Context) ([]RuntimeSession, error) {
	configurations, err := s.configs.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("load automatic browser proxies: %w", err)
	}
	managed := make([]configdomain.ConnectionConfiguration, 0)
	for _, configuration := range configurations {
		if configdomain.IsManagedSOCKSConfigurationID(configuration.ID) {
			managed = append(managed, configuration)
		}
	}

	type startResult struct {
		state   RuntimeSession
		started bool
		err     error
	}
	results := make([]startResult, len(managed))
	var wait sync.WaitGroup
	for index, configuration := range managed {
		index, configuration := index, configuration
		wait.Add(1)
		go func() {
			defer wait.Done()
			state, started, startErr := s.startIfInactive(ctx, configuration.ID)
			if startErr != nil {
				startErr = fmt.Errorf("%s: %w", configuration.Label, startErr)
			}
			results[index] = startResult{state: state, started: started, err: startErr}
		}()
	}
	wait.Wait()

	states := make([]RuntimeSession, 0, len(results))
	startErrors := make([]error, 0)
	for _, result := range results {
		if result.started {
			states = append(states, result.state)
		}
		if result.err != nil {
			startErrors = append(startErrors, result.err)
		}
	}
	return states, errors.Join(startErrors...)
}

func configurationsStartingOnLaunch(configurations []configdomain.ConnectionConfiguration) []configdomain.ConnectionConfiguration {
	selected := make([]configdomain.ConnectionConfiguration, 0, len(configurations))
	for _, configuration := range configurations {
		if configuration.StartOnLaunch {
			selected = append(selected, configuration)
		}
	}
	return selected
}

func canBulkStart(state RuntimeSession, exists bool) bool {
	if !exists {
		return true
	}

	return state.Status == StatusStopped || state.Status == StatusFailed
}

func (s *Service) startIfInactive(ctx context.Context, configurationID string) (RuntimeSession, bool, error) {
	if err := s.acquireRuntimeOperation(ctx); err != nil {
		return RuntimeSession{}, false, err
	}
	defer s.operationGate.RUnlock()

	operationLock := s.operationLock(configurationID)
	operationLock.Lock()
	defer operationLock.Unlock()

	current, exists := s.runtimes.Get(configurationID)
	if !canBulkStart(current, exists) {
		return current, false, nil
	}

	state, err := s.start(ctx, configurationID, "")
	return state, true, err
}

func (s *Service) operationLock(configurationID string) *sync.Mutex {
	operationLock, _ := s.operationLocks.LoadOrStore(configurationID, &sync.Mutex{})
	return operationLock.(*sync.Mutex)
}

func (s *Service) setRuntimeIfTokenCurrent(state RuntimeSession, runner Runner, passphrase string, runtimeToken string) bool {
	operationLock := s.operationLock(state.ConfigurationID)
	operationLock.Lock()
	defer operationLock.Unlock()

	currentToken, ok := s.runtimes.Token(state.ConfigurationID)
	if !ok || currentToken != runtimeToken {
		return false
	}
	s.runtimes.SetWithToken(state, runner, passphrase, runtimeToken)
	return true
}

// WithExclusiveMutation coordinates saved server/configuration changes with
// every operation that can create, replace, or stop a live tunnel. Callers
// must perform their active-session guard and persistence mutation inside the
// callback so a start or reconnect cannot slip between those two steps.
func (s *Service) WithExclusiveMutation(ctx context.Context, mutate func(context.Context) error) error {
	if mutate == nil {
		return fmt.Errorf("mutation callback is required")
	}
	s.operationGate.Lock()
	defer s.operationGate.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return mutate(ctx)
}

func (s *Service) acquireRuntimeOperation(ctx context.Context) error {
	s.operationGate.RLock()
	if err := ctx.Err(); err != nil {
		s.operationGate.RUnlock()
		return err
	}
	return nil
}

func (s *Service) SubmitKeyUnlock(ctx context.Context, configurationID string, passphrase string) (RuntimeSession, error) {
	if err := s.acquireRuntimeOperation(ctx); err != nil {
		return RuntimeSession{}, err
	}
	defer s.operationGate.RUnlock()

	operationLock := s.operationLock(configurationID)
	operationLock.Lock()
	defer operationLock.Unlock()

	return s.start(ctx, configurationID, passphrase)
}

func (s *Service) Stop(ctx context.Context, configurationID string) (RuntimeSession, error) {
	if err := s.acquireRuntimeOperation(ctx); err != nil {
		return RuntimeSession{}, err
	}
	defer s.operationGate.RUnlock()

	operationLock := s.operationLock(configurationID)
	operationLock.Lock()
	defer operationLock.Unlock()

	current, exists := s.runtimes.Get(configurationID)
	if !exists {
		state := stoppedState(configurationID, "Configuration is already stopped")
		s.runtimes.SetWithToken(state, nil, "", historyID())
		return state, nil
	}

	runner, _, _ := s.runtimes.Runner(configurationID)
	if runner != nil {
		if err := runner.Stop(); err != nil {
			return current, fmt.Errorf("stop session: %w", err)
		}
	}
	state := stoppedState(configurationID, "Tunnel stopped")
	s.runtimes.SetWithToken(state, nil, "", historyID())
	_ = s.recordHistory(ctx, configurationID, current.StartedAt, OutcomeStopped, state.StatusDetail)
	return state, nil
}

func (s *Service) stopExistingRunner(configurationID string) error {
	runner, _, ok := s.runtimes.Runner(configurationID)
	if !ok || runner == nil {
		return nil
	}
	return runner.Stop()
}

func (s *Service) start(ctx context.Context, configurationID string, passphrase string) (RuntimeSession, error) {
	configuration, server, err := s.loadConfigurationAndServer(ctx, configurationID)
	if err != nil {
		return RuntimeSession{}, err
	}
	if err := s.stopExistingRunner(configurationID); err != nil {
		return RuntimeSession{}, fmt.Errorf("stop existing tunnel: %w", err)
	}

	runtimeToken := historyID()

	starting := RuntimeSession{ConfigurationID: configurationID, Status: StatusStarting, StatusDetail: "Starting tunnel", StartedAt: time.Now().UTC(), LastStateChangeAt: time.Now().UTC()}
	s.runtimes.SetWithToken(starting, nil, passphrase, runtimeToken)

	runner := s.factory.New(server, configuration, passphrase, func(disconnectErr error) {
		s.handleDisconnect(configuration, passphrase, runtimeToken, disconnectErr)
	})
	startErr := runner.Start()
	if ctxErr := ctx.Err(); ctxErr != nil {
		_ = runner.Stop()
		state := stoppedState(configurationID, "Tunnel start canceled")
		s.runtimes.SetWithToken(state, nil, "", historyID())
		return state, fmt.Errorf("start tunnel: %w", ctxErr)
	}
	if startErr != nil {
		if errors.Is(startErr, auth.ErrPassphraseRequired) {
			state := RuntimeSession{ConfigurationID: configurationID, Status: StatusNeedsAttention, StatusDetail: "Unlock the SSH key to continue", StartedAt: starting.StartedAt, LastStateChangeAt: time.Now().UTC(), LastError: startErr.Error(), NeedsUserInput: true}
			s.runtimes.SetWithToken(state, nil, passphrase, runtimeToken)
			_ = s.recordHistory(ctx, configurationID, starting.StartedAt, OutcomeFailedAuth, state.StatusDetail)
			return state, nil
		}
		detail := tunnel.DescribeStartError(startErr, server, configuration)
		state := RuntimeSession{ConfigurationID: configurationID, Status: StatusFailed, StatusDetail: detail, StartedAt: starting.StartedAt, LastStateChangeAt: time.Now().UTC(), LastError: startErr.Error()}
		s.runtimes.SetWithToken(state, nil, passphrase, runtimeToken)
		outcome := OutcomeFailedRuntime
		if auth.IsAuthenticationRejected(startErr) {
			outcome = OutcomeFailedAuth
		}
		_ = s.recordHistory(ctx, configurationID, starting.StartedAt, outcome, state.StatusDetail)
		return state, nil
	}

	connectedPort := runner.BoundPort()
	connected := RuntimeSession{ConfigurationID: configurationID, Status: StatusConnected, BoundPort: connectedPort, StatusDetail: fmt.Sprintf("Listening on localhost:%d", connectedPort), StartedAt: starting.StartedAt, LastStateChangeAt: time.Now().UTC()}
	s.runtimes.SetWithToken(connected, runner, passphrase, runtimeToken)
	_ = s.recordHistory(ctx, configurationID, starting.StartedAt, OutcomeConnected, connected.StatusDetail)
	return connected, nil
}

func (s *Service) handleDisconnect(configuration configdomain.ConnectionConfiguration, passphrase string, runtimeToken string, disconnectErr error) {
	currentToken, ok := s.runtimes.Token(configuration.ID)
	if !ok || currentToken != runtimeToken {
		return
	}

	detail := tunnel.DescribeDisconnectError(disconnectErr)
	_ = s.stopExistingRunner(configuration.ID)
	if !configuration.AutoReconnectEnabled {
		state := RuntimeSession{ConfigurationID: configuration.ID, Status: StatusFailed, StatusDetail: detail, LastStateChangeAt: time.Now().UTC(), LastError: disconnectErr.Error()}
		s.setRuntimeIfTokenCurrent(state, nil, passphrase, runtimeToken)
		return
	}

	go func() {
		for attempt := 1; ; attempt++ {
			currentToken, ok := s.runtimes.Token(configuration.ID)
			if !ok || currentToken != runtimeToken {
				return
			}

			delay := s.reconnectDelay(attempt)
			state := RuntimeSession{ConfigurationID: configuration.ID, Status: StatusReconnecting, StatusDetail: fmt.Sprintf("%s Reconnect attempt %d failed or is pending. Retrying in %s until the tunnel is restored or you stop it.", detail, attempt, delay.Round(time.Second)), LastStateChangeAt: time.Now().UTC(), ReconnectAttemptCount: attempt, LastError: disconnectErr.Error()}
			if !s.setRuntimeIfTokenCurrent(state, nil, passphrase, runtimeToken) {
				return
			}
			time.Sleep(delay)

			currentToken, ok = s.runtimes.Token(configuration.ID)
			if !ok || currentToken != runtimeToken {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), s.reconnectTimeout)
			nextState, restarted, err := s.restartWithToken(ctx, configuration.ID, passphrase, runtimeToken)
			cancel()
			if !restarted {
				return
			}
			if err != nil {
				continue
			}
			if nextState.Status == StatusConnected || nextState.Status == StatusNeedsAttention {
				return
			}
		}
	}()
}

func (s *Service) restartWithToken(ctx context.Context, configurationID string, passphrase string, runtimeToken string) (RuntimeSession, bool, error) {
	if err := s.acquireRuntimeOperation(ctx); err != nil {
		return RuntimeSession{}, true, err
	}
	defer s.operationGate.RUnlock()

	operationLock := s.operationLock(configurationID)
	operationLock.Lock()
	defer operationLock.Unlock()

	currentToken, ok := s.runtimes.Token(configurationID)
	if !ok || currentToken != runtimeToken {
		return RuntimeSession{}, false, nil
	}

	configuration, server, err := s.loadConfigurationAndServer(ctx, configurationID)
	if err != nil {
		return RuntimeSession{}, true, err
	}
	if err := s.stopExistingRunner(configurationID); err != nil {
		return RuntimeSession{}, true, fmt.Errorf("stop existing tunnel: %w", err)
	}

	current, _ := s.runtimes.Get(configurationID)
	startedAt := current.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	runner := s.factory.New(server, configuration, passphrase, func(disconnectErr error) {
		s.handleDisconnect(configuration, passphrase, runtimeToken, disconnectErr)
	})
	startErr := runner.Start()
	if ctxErr := ctx.Err(); ctxErr != nil {
		_ = runner.Stop()
		state := RuntimeSession{ConfigurationID: configurationID, Status: StatusReconnecting, StatusDetail: "Reconnect attempt canceled", StartedAt: startedAt, LastStateChangeAt: time.Now().UTC(), LastError: ctxErr.Error()}
		s.runtimes.SetWithToken(state, nil, passphrase, runtimeToken)
		return state, true, ctxErr
	}
	if startErr != nil {
		if errors.Is(startErr, auth.ErrPassphraseRequired) {
			state := RuntimeSession{ConfigurationID: configurationID, Status: StatusNeedsAttention, StatusDetail: "Unlock the SSH key to continue", StartedAt: startedAt, LastStateChangeAt: time.Now().UTC(), LastError: startErr.Error(), NeedsUserInput: true}
			s.runtimes.SetWithToken(state, nil, passphrase, runtimeToken)
			_ = s.recordHistory(ctx, configurationID, startedAt, OutcomeFailedAuth, state.StatusDetail)
			return state, true, nil
		}

		detail := tunnel.DescribeStartError(startErr, server, configuration)
		state := RuntimeSession{ConfigurationID: configurationID, Status: StatusReconnecting, StatusDetail: detail, StartedAt: startedAt, LastStateChangeAt: time.Now().UTC(), LastError: startErr.Error()}
		s.runtimes.SetWithToken(state, nil, passphrase, runtimeToken)
		return state, true, nil
	}

	connectedPort := runner.BoundPort()
	connected := RuntimeSession{ConfigurationID: configurationID, Status: StatusConnected, BoundPort: connectedPort, StatusDetail: fmt.Sprintf("Listening on localhost:%d", connectedPort), StartedAt: startedAt, LastStateChangeAt: time.Now().UTC()}
	s.runtimes.SetWithToken(connected, runner, passphrase, runtimeToken)
	_ = s.recordHistory(ctx, configurationID, startedAt, OutcomeConnected, connected.StatusDetail)
	return connected, true, nil
}

func (s *Service) loadConfigurationAndServer(ctx context.Context, configurationID string) (configdomain.ConnectionConfiguration, serverdomain.Server, error) {
	configuration, err := s.configs.Get(ctx, configurationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return configdomain.ConnectionConfiguration{}, serverdomain.Server{}, fmt.Errorf("configuration no longer exists")
		}
		return configdomain.ConnectionConfiguration{}, serverdomain.Server{}, fmt.Errorf("load configuration: %w", err)
	}

	server, err := s.servers.Get(ctx, configuration.ServerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return configdomain.ConnectionConfiguration{}, serverdomain.Server{}, fmt.Errorf("server no longer exists")
		}
		return configdomain.ConnectionConfiguration{}, serverdomain.Server{}, fmt.Errorf("load server: %w", err)
	}

	return configuration, server, nil
}

func nextReconnectDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 30 {
		attempt = 30
	}
	return time.Duration(attempt) * time.Second
}

func stoppedState(configurationID string, detail string) RuntimeSession {
	return RuntimeSession{ConfigurationID: configurationID, Status: StatusStopped, StatusDetail: detail, LastStateChangeAt: time.Now().UTC()}
}

func (s *Service) recordHistory(ctx context.Context, configurationID string, startedAt time.Time, outcome HistoryOutcome, message string) error {
	if s.history == nil {
		return nil
	}
	entry := SessionHistoryEntry{ID: historyID(), ConfigurationID: configurationID, StartedAt: startedAt, EndedAt: time.Now().UTC(), Outcome: outcome, Message: message}
	return s.history.Add(ctx, entry)
}

func historyID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
