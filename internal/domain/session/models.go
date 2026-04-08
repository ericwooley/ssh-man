package session

import (
	"time"
)

type Status string

const (
	StatusStopped        Status = "stopped"
	StatusStarting       Status = "starting"
	StatusConnected      Status = "connected"
	StatusReconnecting   Status = "reconnecting"
	StatusNeedsAttention Status = "needs_attention"
	StatusFailed         Status = "failed"
)

type RuntimeSession struct {
	ConfigurationID       string    `json:"configurationId"`
	Status                Status    `json:"status"`
	BoundPort             int       `json:"boundPort,omitempty"`
	StatusDetail          string    `json:"statusDetail,omitempty"`
	StartedAt             time.Time `json:"startedAt,omitempty"`
	LastStateChangeAt     time.Time `json:"lastStateChangeAt"`
	ReconnectAttemptCount int       `json:"reconnectAttemptCount,omitempty"`
	LastError             string    `json:"lastError,omitempty"`
	NeedsUserInput        bool      `json:"needsUserInput,omitempty"`
}

type HistoryOutcome string

const (
	OutcomeConnected          HistoryOutcome = "connected"
	OutcomeStopped            HistoryOutcome = "stopped"
	OutcomeReconnectExhausted HistoryOutcome = "reconnect_exhausted"
	OutcomeFailedValidation   HistoryOutcome = "failed_validation"
	OutcomeFailedAuth         HistoryOutcome = "failed_auth"
	OutcomeFailedRuntime      HistoryOutcome = "failed_runtime"
)

type SessionHistoryEntry struct {
	ID              string         `json:"id"`
	ConfigurationID string         `json:"configurationId"`
	StartedAt       time.Time      `json:"startedAt"`
	EndedAt         time.Time      `json:"endedAt"`
	Outcome         HistoryOutcome `json:"outcome"`
	Message         string         `json:"message"`
}
