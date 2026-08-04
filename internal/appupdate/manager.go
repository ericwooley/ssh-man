package appupdate

import (
	"context"
	"fmt"
	"log"
	"sync"
)

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

	mu      sync.Mutex
	enabled bool
	cancel  context.CancelFunc
	runID   uint64
	staged  *stagedUpdate
	wait    sync.WaitGroup
}

func NewManager(currentVersion, configDir string) *Manager {
	return &Manager{
		currentVersion: currentVersion,
		configDir:      configDir,
		client:         newClient(),
		installer:      newPlatformInstaller(),
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
	m.enabled = enabled
	if !enabled {
		cancel := m.cancel
		m.cancel = nil
		m.runID++
		staged := m.staged
		m.staged = nil
		m.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if staged != nil {
			if err := m.installer.cleanup(staged); err != nil {
				log.Printf("remove disabled automatic update: %v", err)
			}
		}
		return
	}
	if !m.installer.supported() {
		m.mu.Unlock()
		return
	}
	if _, ok := parseVersion(m.currentVersion); !ok {
		m.mu.Unlock()
		return
	}
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.runID++
	runID := m.runID
	m.cancel = cancel
	m.wait.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wait.Done()
		defer func() {
			m.mu.Lock()
			if m.runID == runID {
				m.cancel = nil
			}
			m.mu.Unlock()
		}()
		plan, err := m.client.check(ctx, m.currentVersion)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("automatic update check: %v", err)
			}
			return
		}
		if plan == nil {
			return
		}
		staged, err := m.installer.stage(ctx, m.client, plan, m.configDir)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("stage automatic update %s: %v", plan.Version, err)
			}
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
		m.mu.Unlock()
		log.Printf("automatic update %s is verified and will install after SSH Man quits", plan.Version)
	}()
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
