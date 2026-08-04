package appupdate

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
)

type Channel string

const (
	ChannelStable       Channel = "stable"
	ChannelExperimental Channel = "experimental"
)

type State string

const (
	StateIdle        State = "idle"
	StateChecking    State = "checking"
	StateAvailable   State = "available"
	StateDownloading State = "downloading"
	StateReady       State = "ready"
	StateError       State = "error"
)

type Status struct {
	State   State   `json:"state"`
	Version string  `json:"version,omitempty"`
	Channel Channel `json:"channel"`
	Message string  `json:"message,omitempty"`
}

type stagedUpdate struct {
	Version        string
	AppPath        string
	RootPath       string
	CurrentAppPath string
}

type platformInstaller interface {
	supported() bool
	stage(context.Context, *Client, *updatePlan, string) (*stagedUpdate, error)
	prepare(*stagedUpdate, int) error
	cleanup(*stagedUpdate) error
}

type Manager struct {
	currentVersion string
	configDir      string
	client         *Client
	installer      platformInstaller

	mu             sync.Mutex
	configured     bool
	enabled        bool
	experimental   bool
	cancel         context.CancelFunc
	runID          uint64
	staged         *stagedUpdate
	status         Status
	statusObserver func(Status)
	wait           sync.WaitGroup
}

func NewManager(currentVersion, configDir string) *Manager {
	return &Manager{
		currentVersion: currentVersion,
		configDir:      configDir,
		client:         newClient(),
		installer:      newPlatformInstaller(),
		status:         Status{State: StateIdle, Channel: ChannelStable},
	}
}

func (m *Manager) Start(enabled bool) {
	m.SetEnabled(enabled)
}

func (m *Manager) SetEnabled(enabled bool) {
	if m == nil {
		return
	}

	m.mu.Lock()
	experimental := m.experimental
	m.mu.Unlock()
	m.Configure(enabled, experimental)
}

func (m *Manager) Configure(enabled, experimental bool) {
	if m == nil {
		return
	}

	channel := ChannelStable
	if experimental {
		channel = ChannelExperimental
	}

	m.mu.Lock()
	if m.configured && m.enabled == enabled && m.experimental == experimental {
		m.mu.Unlock()
		return
	}
	m.configured = true
	m.enabled = enabled
	m.experimental = experimental
	previousCancel := m.cancel
	previousStaged := m.staged
	m.cancel = nil
	m.staged = nil
	m.runID++
	runID := m.runID
	observer := m.statusObserver

	if !enabled || !m.installer.supported() {
		status := Status{State: StateIdle, Channel: channel}
		m.status = status
		m.mu.Unlock()
		cancelAndCleanup(previousCancel, m.installer, previousStaged)
		notifyStatus(observer, status)
		return
	}
	if _, ok := parseVersion(m.currentVersion); !ok {
		status := Status{State: StateIdle, Channel: channel}
		m.status = status
		m.mu.Unlock()
		cancelAndCleanup(previousCancel, m.installer, previousStaged)
		notifyStatus(observer, status)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.wait.Add(1)
	status := Status{State: StateChecking, Channel: channel}
	m.status = status
	m.mu.Unlock()
	cancelAndCleanup(previousCancel, m.installer, previousStaged)
	notifyStatus(observer, status)

	go func() {
		defer m.wait.Done()
		defer func() {
			m.mu.Lock()
			if m.runID == runID {
				m.cancel = nil
			}
			m.mu.Unlock()
		}()
		plan, err := m.client.check(ctx, m.currentVersion, experimental)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("automatic update check: %v", err)
				m.setStatus(runID, Status{State: StateError, Channel: channel, Message: err.Error()})
			}
			return
		}
		if plan == nil {
			m.setStatus(runID, Status{State: StateIdle, Channel: channel})
			return
		}
		if !m.setStatus(runID, Status{State: StateAvailable, Version: plan.Version, Channel: channel}) {
			return
		}
		if !m.setStatus(runID, Status{State: StateDownloading, Version: plan.Version, Channel: channel}) {
			return
		}
		staged, err := m.installer.stage(ctx, m.client, plan, m.configDir)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("stage automatic update %s: %v", plan.Version, err)
				m.setStatus(runID, Status{State: StateError, Version: plan.Version, Channel: channel, Message: err.Error()})
			}
			return
		}
		if staged == nil {
			m.setStatus(runID, Status{State: StateError, Version: plan.Version, Channel: channel, Message: "the update could not be prepared"})
			return
		}

		m.mu.Lock()
		if !m.enabled || m.runID != runID {
			m.mu.Unlock()
			if cleanupErr := m.installer.cleanup(staged); cleanupErr != nil {
				log.Printf("remove cancelled or superseded automatic update: %v", cleanupErr)
			}
			return
		}
		m.staged = staged
		status := Status{State: StateReady, Version: plan.Version, Channel: channel}
		m.status = status
		observer := m.statusObserver
		m.mu.Unlock()
		notifyStatus(observer, status)
		log.Printf("automatic update %s is verified and will install after SSH Man quits", plan.Version)
	}()
}

func (m *Manager) SetStatusObserver(observer func(Status)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.statusObserver = observer
	status := normalizedStatus(m.status, m.experimental)
	m.mu.Unlock()
	notifyStatus(observer, status)
}

func (m *Manager) Status() Status {
	if m == nil {
		return Status{State: StateIdle, Channel: ChannelStable}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return normalizedStatus(m.status, m.experimental)
}

func (m *Manager) Install() error {
	if m == nil {
		return errors.New("application updates are unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled {
		return errors.New("automatic updates are disabled")
	}
	if m.status.State != StateReady || m.staged == nil {
		return errors.New("the update is not ready")
	}
	return nil
}

func (m *Manager) setStatus(runID uint64, status Status) bool {
	m.mu.Lock()
	if m.runID != runID || !m.enabled {
		m.mu.Unlock()
		return false
	}
	m.status = status
	observer := m.statusObserver
	m.mu.Unlock()
	notifyStatus(observer, status)
	return true
}

func normalizedStatus(status Status, experimental bool) Status {
	if status.State == "" {
		status.State = StateIdle
	}
	if status.Channel == "" {
		status.Channel = ChannelStable
		if experimental {
			status.Channel = ChannelExperimental
		}
	}
	return status
}

func notifyStatus(observer func(Status), status Status) {
	if observer != nil {
		observer(status)
	}
}

func cancelAndCleanup(cancel context.CancelFunc, installer platformInstaller, staged *stagedUpdate) {
	if cancel != nil {
		cancel()
	}
	if staged != nil {
		if err := installer.cleanup(staged); err != nil {
			log.Printf("remove superseded automatic update: %v", err)
		}
	}
}

// Supported reports whether this platform can verify and install application updates.
func Supported() bool {
	return newPlatformInstaller().supported()
}

func (m *Manager) StopAndPrepare(enabled bool, parentPID int) error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	m.enabled = enabled
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wait.Wait()

	m.mu.Lock()
	staged := m.staged
	m.staged = nil
	m.mu.Unlock()
	if staged == nil {
		return nil
	}
	if !enabled {
		if err := m.installer.cleanup(staged); err != nil {
			return fmt.Errorf("remove disabled automatic update: %w", err)
		}
		return nil
	}
	if err := m.installer.prepare(staged, parentPID); err != nil {
		return fmt.Errorf("prepare automatic update %s: %w", staged.Version, err)
	}
	return nil
}
