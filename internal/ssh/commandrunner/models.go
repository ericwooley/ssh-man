package commandrunner

import "time"

const MaxOutputBytes = 2 * 1024 * 1024

type ConnectResult struct {
	Connected       bool   `json:"connected"`
	NeedsPassphrase bool   `json:"needsPassphrase,omitempty"`
	HomePath        string `json:"homePath,omitempty"`
}

type ExecutionResult struct {
	Output    string    `json:"output"`
	ExitCode  int       `json:"exitCode"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Truncated bool      `json:"truncated,omitempty"`
	Error     string    `json:"error,omitempty"`
	Cancelled bool      `json:"cancelled,omitempty"`
}

type Completion struct {
	Value string `json:"value"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
}

type CompletionResult struct {
	Items []Completion `json:"items"`
}
