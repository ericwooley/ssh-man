package bindings

import (
	"fmt"
	"sync"

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
type SettingsWindowBindings struct {
	mu             sync.Mutex
	closeAllowed   bool
	requestCloseFn func()
}

func NewSettingsWindowBindings() *SettingsWindowBindings {
	return &SettingsWindowBindings{}
}

func NewSettingsWindowBindingsWithCloseRequester(request func()) *SettingsWindowBindings {
	return &SettingsWindowBindings{requestCloseFn: request}
}

func (*SettingsWindowBindings) IsSettingsWindow() bool {
	return true
}

func (b *SettingsWindowBindings) AllowClose() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closeAllowed = true
}

func (b *SettingsWindowBindings) RequestCloseConfirmation() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	if b.closeAllowed {
		b.mu.Unlock()
		return false
	}
	request := b.requestCloseFn
	b.mu.Unlock()
	if request != nil {
		request()
	}
	return true
}
