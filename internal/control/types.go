package control

import (
	"context"
	"encoding/json"

	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
	"ssh-man/internal/platform/defaultbrowser"
)

const ProtocolVersion = 1

type ServerRecord struct {
	Server         serverdomain.Server                    `json:"server"`
	Configurations []configdomain.ConnectionConfiguration `json:"configurations"`
}

type Diagnostics struct {
	AppDataPath  string `json:"appDataPath"`
	DatabasePath string `json:"databasePath"`
}

type State struct {
	Servers         []ServerRecord                   `json:"servers"`
	Preferences     preferencesdomain.UserPreference `json:"preferences"`
	Sessions        []sessiondomain.RuntimeSession   `json:"sessions"`
	Diagnostics     Diagnostics                      `json:"diagnostics"`
	CurrentUsername string                           `json:"currentUsername,omitempty"`
	Message         string                           `json:"message,omitempty"`
	Recoverable     bool                             `json:"recoverable,omitempty"`
}

type Failure struct {
	ID      string `json:"id,omitempty"`
	Label   string `json:"label,omitempty"`
	Message string `json:"message"`
}

type BulkResult struct {
	Sessions []sessiondomain.RuntimeSession `json:"sessions"`
	Failures []Failure                      `json:"failures"`
}

type Request struct {
	ProtocolVersion int                                   `json:"protocolVersion"`
	Command         string                                `json:"command"`
	ServerID        string                                `json:"serverId,omitempty"`
	ConfigurationID string                                `json:"configurationId,omitempty"`
	BrowserID       string                                `json:"browserId,omitempty"`
	Secret          string                                `json:"secret,omitempty"`
	Server          *serverdomain.Server                  `json:"server,omitempty"`
	Configuration   *configdomain.ConnectionConfiguration `json:"configuration,omitempty"`
	Preferences     *preferencesdomain.UserPreference     `json:"preferences,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Response struct {
	ProtocolVersion int             `json:"protocolVersion"`
	Data            json.RawMessage `json:"data,omitempty"`
	Error           *Error          `json:"error,omitempty"`
}

type Backend struct {
	State               func(context.Context) (State, error)
	SaveServer          func(context.Context, serverdomain.Server) (serverdomain.Server, error)
	DeleteServer        func(context.Context, string) error
	SaveConfiguration   func(context.Context, configdomain.ConnectionConfiguration) (configdomain.ConnectionConfiguration, error)
	DeleteConfiguration func(context.Context, string) error
	Start               func(context.Context, string) (sessiondomain.RuntimeSession, error)
	StartServer         func(context.Context, string) BulkResult
	Stop                func(context.Context, string) (sessiondomain.RuntimeSession, error)
	StopServer          func(context.Context, string) BulkResult
	Retry               func(context.Context, string) (sessiondomain.RuntimeSession, error)
	Unlock              func(context.Context, string, string) (sessiondomain.RuntimeSession, error)
	History             func(context.Context, string) ([]sessiondomain.SessionHistoryEntry, error)
	DiscoverBrowsers    func(context.Context) ([]browser.BrowserOption, error)
	PreviewBrowser      func(context.Context, string, string) (browser.LaunchPreview, error)
	LaunchBrowser       func(context.Context, string, string) error
	SavePreferences     func(context.Context, preferencesdomain.UserPreference) (preferencesdomain.UserPreference, error)
	SetDefaultBrowser   func(context.Context) (defaultbrowser.Status, error)
	Show                func() error
	Hide                func() error
	Quit                func() error
}
