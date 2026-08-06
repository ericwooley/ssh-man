package portlink

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

type Scheme string

const (
	SchemeHTTP  Scheme = "http"
	SchemeHTTPS Scheme = "https"

	MaxFaviconDataURLBytes = 256 * 1024
	maxNameBytes           = 120
)

var ErrNotFound = errors.New("saved port link not found")

type Link struct {
	ID             string    `json:"id"`
	ServerID       string    `json:"serverId"`
	Port           int       `json:"port"`
	Name           string    `json:"name"`
	Scheme         Scheme    `json:"scheme"`
	FaviconDataURL string    `json:"faviconDataUrl,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (link Link) Validate() error {
	if strings.TrimSpace(link.ServerID) == "" {
		return fmt.Errorf("server id is required")
	}
	if link.Port < 1 || link.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if strings.TrimSpace(link.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(link.Name) > maxNameBytes {
		return fmt.Errorf("name must be at most %d bytes", maxNameBytes)
	}
	for _, value := range link.Name {
		if unicode.IsControl(value) {
			return fmt.Errorf("name must not contain control characters")
		}
	}
	if link.Scheme != SchemeHTTP && link.Scheme != SchemeHTTPS {
		return fmt.Errorf("scheme must be http or https")
	}
	if link.FaviconDataURL == "" {
		return nil
	}
	if len(link.FaviconDataURL) > MaxFaviconDataURLBytes {
		return fmt.Errorf("favicon data is too large")
	}
	allowedPrefix := false
	for _, prefix := range []string{
		"data:image/png;base64,",
		"data:image/jpeg;base64,",
		"data:image/gif;base64,",
		"data:image/webp;base64,",
		"data:image/x-icon;base64,",
		"data:image/vnd.microsoft.icon;base64,",
	} {
		if strings.HasPrefix(link.FaviconDataURL, prefix) {
			allowedPrefix = true
			break
		}
	}
	if !allowedPrefix {
		return fmt.Errorf("favicon must be a base64 image data URL")
	}
	return nil
}
