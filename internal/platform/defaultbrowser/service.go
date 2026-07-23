package defaultbrowser

import "fmt"

const BundleID = "tech.moonpixels.ssh-man"

type Status struct {
	Supported bool `json:"supported"`
	IsDefault bool `json:"isDefault"`
}

type Manager struct {
	supported  func() bool
	isDefault  func() (bool, error)
	setDefault func() error
}

func NewManager() *Manager {
	return &Manager{
		supported:  defaultBrowserSupported,
		isDefault:  isDefaultBrowser,
		setDefault: setDefaultBrowser,
	}
}

func (m *Manager) Status() (Status, error) {
	if !m.supported() {
		return Status{Supported: false}, nil
	}
	current, err := m.isDefault()
	if err != nil {
		return Status{}, fmt.Errorf("check the default browser: %w", err)
	}
	return Status{Supported: true, IsDefault: current}, nil
}

func (m *Manager) SetAsDefault() (Status, error) {
	if !m.supported() {
		return Status{}, fmt.Errorf("setting the default browser is only supported on macOS")
	}
	if err := m.setDefault(); err != nil {
		return Status{}, fmt.Errorf("set SSH Man as the default browser: %w", err)
	}
	status, err := m.Status()
	if err != nil {
		return Status{}, err
	}
	if !status.IsDefault {
		return status, fmt.Errorf("macOS did not retain SSH Man as the HTTP and HTTPS handler")
	}
	return status, nil
}
