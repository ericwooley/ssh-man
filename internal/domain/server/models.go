package server

import (
	"fmt"
	"strings"
	"time"
)

type AuthMode string

const (
	AuthModePrivateKey AuthMode = "private_key"
	AuthModeAgent      AuthMode = "agent"
)

type Server struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Username     string    `json:"username"`
	AuthMode     AuthMode  `json:"authMode"`
	KeyReference string    `json:"keyReference,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (s Server) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("server name is required")
	}
	if strings.TrimSpace(s.Host) == "" {
		return fmt.Errorf("server host is required")
	}
	if strings.TrimSpace(s.Username) == "" {
		return fmt.Errorf("server username is required")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	if s.AuthMode != AuthModePrivateKey && s.AuthMode != AuthModeAgent {
		return fmt.Errorf("server auth mode must be private_key or agent")
	}
	if s.AuthMode == AuthModePrivateKey && strings.TrimSpace(s.KeyReference) == "" {
		return fmt.Errorf("private key path is required when auth mode is private_key")
	}

	return nil
}

type Summary struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Username           string `json:"username"`
	ConfigurationCount int    `json:"configurationCount"`
}
