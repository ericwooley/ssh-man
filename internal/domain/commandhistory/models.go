package commandhistory

import "time"

type Entry struct {
	ID        string    `json:"id"`
	ServerID  string    `json:"serverId"`
	Command   string    `json:"command"`
	Output    string    `json:"output"`
	ExitCode  int       `json:"exitCode"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Truncated bool      `json:"truncated,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type RecordInput struct {
	ServerID  string
	Command   string
	Output    string
	ExitCode  int
	StartedAt time.Time
	EndedAt   time.Time
	Truncated bool
	Error     string
}
