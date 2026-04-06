package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
	"ssh-man/internal/ssh/tunnel"
)

type ConfigStore interface {
	Get(ctx context.Context, id string) (configdomain.ConnectionConfiguration, error)
}

type ServerStore interface {
	Get(ctx context.Context, id string) (serverdomain.Server, error)
}

type HistoryStore interface {
	Add(ctx context.Context, entry SessionHistoryEntry) error
}

type Factory interface {
	New(server serverdomain.Server, config configdomain.ConnectionConfiguration, passphrase string, onDisconnect func(error)) Runner
}

type tunnelFactory struct{}

func (t tunnelFactory) New(server serverdomain.Server, config configdomain.ConnectionConfiguration, passphrase string, onDisconnect func(error)) Runner {
	return tunnel.NewSession(server, config, passphrase, onDisconnect)
}

type Service struct {
	configs  ConfigStore
	servers  ServerStore
	history  HistoryStore
	runtimes *RuntimeStore
	factory  Factory
}

func NewService(configs ConfigStore, servers ServerStore, history HistoryStore, runtimes *RuntimeStore) *Service {
	return &Service{configs: configs, servers: servers, history: history, runtimes: runtimes, factory: tunnelFactory{}}
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

func (s *Service) Start(ctx context.Context, configurationID string) (RuntimeSession, error) {
	return s.start(ctx, configurationID, "")
}

func (s *Service) Retry(ctx context.Context, configurationID string) (RuntimeSession, error) {
	_, passphrase, _ := s.runtimes.Runner(configurationID)
	return s.start(ctx, configurationID, passphrase)
}

func (s *Service) SubmitKeyUnlock(ctx context.Context, configurationID string, passphrase string) (RuntimeSession, error) {
	return s.start(ctx, configurationID, passphrase)
}

func (s *Service) Stop(ctx context.Context, configurationID string) (RuntimeSession, error) {
	current, exists := s.runtimes.Get(configurationID)
	if !exists {
		state := stoppedState(configurationID, "Configuration is already stopped")
		s.runtimes.Set(state, nil, "")
		return state, nil
	}

	runner, _, _ := s.runtimes.Runner(configurationID)
	if runner != nil {
		if err := runner.Stop(); err != nil {
			return current, fmt.Errorf("stop session: %w", err)
		}
	}
	state := stoppedState(configurationID, "Tunnel stopped")
	s.runtimes.Set(state, nil, "")
	_ = s.recordHistory(ctx, configurationID, current.StartedAt, OutcomeStopped, state.StatusDetail)
	return state, nil
}

func (s *Service) start(ctx context.Context, configurationID string, passphrase string) (RuntimeSession, error) {
	configuration, err := s.configs.Get(ctx, configurationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RuntimeSession{}, fmt.Errorf("configuration no longer exists")
		}
		return RuntimeSession{}, fmt.Errorf("load configuration: %w", err)
	}
	server, err := s.servers.Get(ctx, configuration.ServerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RuntimeSession{}, fmt.Errorf("server no longer exists")
		}
		return RuntimeSession{}, fmt.Errorf("load server: %w", err)
	}

	starting := RuntimeSession{ConfigurationID: configurationID, Status: StatusStarting, StatusDetail: "Starting tunnel", StartedAt: time.Now().UTC(), LastStateChangeAt: time.Now().UTC()}
	s.runtimes.Set(starting, nil, passphrase)

	runner := s.factory.New(server, configuration, passphrase, func(disconnectErr error) {
		s.handleDisconnect(configuration, passphrase, disconnectErr)
	})
	if err := runner.Start(); err != nil {
		if errors.Is(err, auth.ErrPassphraseRequired) {
			state := RuntimeSession{ConfigurationID: configurationID, Status: StatusNeedsAttention, StatusDetail: "Unlock the SSH key to continue", StartedAt: starting.StartedAt, LastStateChangeAt: time.Now().UTC(), LastError: err.Error(), NeedsUserInput: true}
			s.runtimes.Set(state, nil, passphrase)
			_ = s.recordHistory(ctx, configurationID, starting.StartedAt, OutcomeFailedAuth, state.StatusDetail)
			return state, nil
		}
		detail := tunnel.DescribeStartError(err, server, configuration)
		state := RuntimeSession{ConfigurationID: configurationID, Status: StatusFailed, StatusDetail: detail, StartedAt: starting.StartedAt, LastStateChangeAt: time.Now().UTC(), LastError: err.Error()}
		s.runtimes.Set(state, nil, passphrase)
		_ = s.recordHistory(ctx, configurationID, starting.StartedAt, OutcomeFailedRuntime, state.StatusDetail)
		return state, nil
	}

	connected := RuntimeSession{ConfigurationID: configurationID, Status: StatusConnected, StatusDetail: fmt.Sprintf("Listening on localhost:%d", configuration.BoundPort()), StartedAt: starting.StartedAt, LastStateChangeAt: time.Now().UTC()}
	s.runtimes.Set(connected, runner, passphrase)
	_ = s.recordHistory(ctx, configurationID, starting.StartedAt, OutcomeConnected, connected.StatusDetail)
	return connected, nil
}

func (s *Service) handleDisconnect(configuration configdomain.ConnectionConfiguration, passphrase string, disconnectErr error) {
	detail := tunnel.DescribeDisconnectError(disconnectErr)
	if !configuration.AutoReconnectEnabled {
		state := RuntimeSession{ConfigurationID: configuration.ID, Status: StatusFailed, StatusDetail: detail, LastStateChangeAt: time.Now().UTC(), LastError: disconnectErr.Error()}
		s.runtimes.Set(state, nil, passphrase)
		return
	}

	go func() {
		for attempt := 1; attempt <= 3; attempt++ {
			state := RuntimeSession{ConfigurationID: configuration.ID, Status: StatusReconnecting, StatusDetail: fmt.Sprintf("%s Reconnect attempt %d of 3.", detail, attempt), LastStateChangeAt: time.Now().UTC(), ReconnectAttemptCount: attempt, LastError: disconnectErr.Error()}
			s.runtimes.Set(state, nil, passphrase)
			time.Sleep(time.Duration(attempt) * time.Second)

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			nextState, err := s.start(ctx, configuration.ID, passphrase)
			cancel()
			if err != nil {
				continue
			}
			if nextState.Status == StatusConnected || nextState.Status == StatusNeedsAttention {
				return
			}
		}

		failed := RuntimeSession{ConfigurationID: configuration.ID, Status: StatusFailed, StatusDetail: fmt.Sprintf("%s Reconnect attempts exhausted.", detail), LastStateChangeAt: time.Now().UTC(), LastError: disconnectErr.Error()}
		s.runtimes.Set(failed, nil, passphrase)
		_ = s.recordHistory(context.Background(), configuration.ID, time.Now().UTC(), OutcomeReconnectExhausted, failed.StatusDetail)
	}()
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
