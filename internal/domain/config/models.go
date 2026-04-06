package config

import (
	"fmt"
	"strings"
	"time"
)

type ConnectionType string

const (
	ConnectionTypeLocalForward ConnectionType = "local_forward"
	ConnectionTypeSOCKSProxy   ConnectionType = "socks_proxy"
)

type ConnectionConfiguration struct {
	ID                   string         `json:"id"`
	ServerID             string         `json:"serverId"`
	Label                string         `json:"label"`
	ConnectionType       ConnectionType `json:"connectionType"`
	LocalPort            int            `json:"localPort,omitempty"`
	RemoteHost           string         `json:"remoteHost,omitempty"`
	RemotePort           int            `json:"remotePort,omitempty"`
	SocksPort            int            `json:"socksPort,omitempty"`
	AutoReconnectEnabled bool           `json:"autoReconnectEnabled"`
	StartOnLaunch        bool           `json:"startOnLaunch"`
	Notes                string         `json:"notes,omitempty"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}

func (c ConnectionConfiguration) Validate() error {
	if strings.TrimSpace(c.ServerID) == "" {
		return fmt.Errorf("server id is required")
	}
	if strings.TrimSpace(c.Label) == "" {
		return fmt.Errorf("configuration label is required")
	}
	switch c.ConnectionType {
	case ConnectionTypeLocalForward:
		if c.LocalPort < 1 || c.LocalPort > 65535 {
			return fmt.Errorf("local port must be between 1 and 65535")
		}
		if strings.TrimSpace(c.RemoteHost) == "" {
			return fmt.Errorf("remote host is required for local forward")
		}
		if c.RemotePort < 1 || c.RemotePort > 65535 {
			return fmt.Errorf("remote port must be between 1 and 65535")
		}
	case ConnectionTypeSOCKSProxy:
		if c.SocksPort < 1 || c.SocksPort > 65535 {
			return fmt.Errorf("socks port must be between 1 and 65535")
		}
	default:
		return fmt.Errorf("configuration type must be local_forward or socks_proxy")
	}

	return nil
}

func (c ConnectionConfiguration) BoundPort() int {
	if c.ConnectionType == ConnectionTypeSOCKSProxy {
		return c.SocksPort
	}
	return c.LocalPort
}

type Summary struct {
	ID                   string `json:"id"`
	ServerID             string `json:"serverId"`
	Label                string `json:"label"`
	ConnectionType       string `json:"connectionType"`
	LocalPort            int    `json:"localPort,omitempty"`
	RemoteHost           string `json:"remoteHost,omitempty"`
	RemotePort           int    `json:"remotePort,omitempty"`
	SocksPort            int    `json:"socksPort,omitempty"`
	AutoReconnectEnabled bool   `json:"autoReconnectEnabled"`
	RuntimeStatus        string `json:"runtimeStatus,omitempty"`
	RuntimeStatusDetail  string `json:"runtimeStatusDetail,omitempty"`
	ReconnectAttempts    int    `json:"reconnectAttemptCount,omitempty"`
}
