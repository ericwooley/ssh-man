package bindings

import (
	"fmt"
	"strings"

	"ssh-man/internal/app/previewwindow"
)

// PreviewLauncherBindings keeps preview process creation at an explicit
// side-effect boundary owned by the explorer window.
type PreviewLauncherBindings struct {
	serverID string
	launch   func(string, string) error
	focus    func(string) error
	isOpen   func(string) bool
}

func NewPreviewLauncherBindingsWithDependencies(
	serverID string,
	launch func(string, string) error,
	focus func(string) error,
	isOpen func(string) bool,
) *PreviewLauncherBindings {
	return &PreviewLauncherBindings{
		serverID: strings.TrimSpace(serverID),
		launch:   launch,
		focus:    focus,
		isOpen:   isOpen,
	}
}

func (b *PreviewLauncherBindings) Open(remotePath string) (previewwindow.State, error) {
	if b.serverID == "" {
		return previewwindow.State{}, fmt.Errorf("server id is required")
	}
	if strings.TrimSpace(remotePath) == "" {
		return previewwindow.State{}, fmt.Errorf("remote path is required")
	}
	if b.launch == nil {
		return previewwindow.State{}, fmt.Errorf("preview window launcher is unavailable")
	}
	if err := b.launch(b.serverID, remotePath); err != nil {
		return previewwindow.State{}, err
	}
	return previewwindow.State{RemotePath: remotePath, Open: true}, nil
}

func (b *PreviewLauncherBindings) Focus(remotePath string) (previewwindow.State, error) {
	if strings.TrimSpace(remotePath) == "" {
		return previewwindow.State{}, fmt.Errorf("remote path is required")
	}
	if b.focus == nil {
		return previewwindow.State{}, fmt.Errorf("preview window focus is unavailable")
	}
	if err := b.focus(remotePath); err != nil {
		return previewwindow.State{}, err
	}
	return previewwindow.State{RemotePath: remotePath, Open: true}, nil
}

func (b *PreviewLauncherBindings) State(remotePath string) previewwindow.State {
	state := previewwindow.State{RemotePath: remotePath}
	if b != nil && b.isOpen != nil && strings.TrimSpace(remotePath) != "" {
		state.Open = b.isOpen(remotePath)
	}
	return state
}
