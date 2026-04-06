package preferences

import (
	"fmt"
	"time"
)

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

type UserPreference struct {
	Theme                Theme     `json:"theme"`
	LastSelectedServerID string    `json:"lastSelectedServerId,omitempty"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func Default() UserPreference {
	return UserPreference{Theme: ThemeDark, UpdatedAt: time.Now().UTC()}
}

func (p UserPreference) Validate() error {
	if p.Theme != ThemeLight && p.Theme != ThemeDark {
		return fmt.Errorf("theme must be light or dark")
	}
	return nil
}
