package previewwindow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"ssh-man/internal/app/companionwindow"
)

const PreviewArgument = "--ssh-man-preview"

var (
	ErrPreviewNotOpen   = errors.New("the preview window is not open")
	ErrFocusUnavailable = errors.New("preview window focus is unavailable")
)

type Request struct {
	ServerID   string
	RemotePath string
}

type State struct {
	RemotePath string `json:"remotePath"`
	Open       bool   `json:"open"`
}

type runningPreview struct {
	process companionwindow.Process
}

type processManager interface {
	LaunchArgsTracked(...string) (companionwindow.Process, error)
	Shutdown(context.Context) error
}

type Manager struct {
	mu       sync.RWMutex
	launchMu sync.Mutex

	processes processManager
	running   map[string]*runningPreview
	listener  func(State)
}

func NewManager() *Manager {
	return newManagerWithProcesses(companionwindow.NewManager(PreviewArgument, "file preview"))
}

func newManagerWithProcesses(processes processManager) *Manager {
	return &Manager{
		processes: processes,
		running:   map[string]*runningPreview{},
	}
}

func normalizeRequest(serverID, remotePath string) (Request, error) {
	request := Request{
		ServerID:   strings.TrimSpace(serverID),
		RemotePath: remotePath,
	}
	if request.ServerID == "" {
		return Request{}, fmt.Errorf("server id is required")
	}
	if strings.TrimSpace(request.RemotePath) == "" {
		return Request{}, fmt.Errorf("remote path is required")
	}
	return request, nil
}

func RequestFromArgs(args []string) (Request, bool) {
	for index, argument := range args {
		if argument != PreviewArgument || index+2 >= len(args) {
			continue
		}
		request, err := normalizeRequest(args[index+1], args[index+2])
		return request, err == nil
	}
	return Request{}, false
}

func InstanceID(request Request) string {
	sum := sha256.Sum256([]byte(request.ServerID + "\x00" + request.RemotePath))
	return hex.EncodeToString(sum[:16])
}

func (m *Manager) Launch(serverID, remotePath string) error {
	if m == nil || m.processes == nil {
		return companionwindow.ErrManagerClosed
	}
	request, err := normalizeRequest(serverID, remotePath)
	if err != nil {
		return err
	}
	m.launchMu.Lock()
	defer m.launchMu.Unlock()

	m.mu.RLock()
	if _, exists := m.running[request.RemotePath]; exists {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	process, err := m.processes.LaunchArgsTracked(request.ServerID, request.RemotePath)
	if err != nil {
		return err
	}
	running := &runningPreview{process: process}
	m.mu.Lock()
	m.running[request.RemotePath] = running
	listener := m.listener
	m.mu.Unlock()
	if listener != nil {
		listener(State{RemotePath: request.RemotePath, Open: true})
	}
	go m.removeWhenDone(request.RemotePath, running)
	return nil
}

func (m *Manager) Focus(remotePath string) error {
	if m == nil || strings.TrimSpace(remotePath) == "" {
		return ErrPreviewNotOpen
	}
	m.mu.RLock()
	running := m.running[remotePath]
	m.mu.RUnlock()
	if running == nil || running.process == nil {
		return ErrPreviewNotOpen
	}
	signal := FocusSignal()
	if signal == nil {
		return ErrFocusUnavailable
	}
	return running.process.Signal(signal)
}

func (m *Manager) SetStateListener(listener func(State)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.listener = listener
	m.mu.Unlock()
}

func (m *Manager) IsOpen(remotePath string) bool {
	if m == nil || strings.TrimSpace(remotePath) == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running[remotePath] != nil
}

func (m *Manager) removeWhenDone(remotePath string, running *runningPreview) {
	if running.process != nil && running.process.Done() != nil {
		<-running.process.Done()
	}
	m.launchMu.Lock()
	defer m.launchMu.Unlock()

	m.mu.Lock()
	if m.running[remotePath] != running {
		m.mu.Unlock()
		return
	}
	delete(m.running, remotePath)
	listener := m.listener
	m.mu.Unlock()
	if listener != nil {
		listener(State{RemotePath: remotePath, Open: false})
	}
}

func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil || m.processes == nil {
		return nil
	}
	return m.processes.Shutdown(ctx)
}
