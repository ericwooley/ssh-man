package bindings

import (
	"fmt"

	"ssh-man/internal/app/settingswindow"
)

// SettingsLauncherBindings keeps the native companion-process launch at a
// small side-effect boundary that the compact menu-bar UI can call.
type SettingsLauncherBindings struct {
	launch func() error
}

func NewSettingsLauncherBindings() *SettingsLauncherBindings {
	return NewSettingsLauncherBindingsWithDependency(settingswindow.Launch)
}

func NewSettingsLauncherBindingsWithDependency(launch func() error) *SettingsLauncherBindings {
	return &SettingsLauncherBindings{launch: launch}
}

func (b *SettingsLauncherBindings) Open() error {
	if b == nil || b.launch == nil {
		return fmt.Errorf("settings window launcher is unavailable")
	}
	return b.launch()
}

// SettingsWindowBindings marks a Wails process as the dedicated settings UI.
// The generated frontend bindings use its runtime presence to select the
// full-size settings layout instead of the compact controller.
type SettingsWindowBindings struct{}

func NewSettingsWindowBindings() *SettingsWindowBindings {
	return &SettingsWindowBindings{}
}

func (*SettingsWindowBindings) IsSettingsWindow() bool {
	return true
}
