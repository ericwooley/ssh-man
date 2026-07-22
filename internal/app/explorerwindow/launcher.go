package explorerwindow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const ServerArgument = "--ssh-man-explorer"

var ErrManagerClosed = errors.New("the explorer process manager is shutting down")

type managedProcess struct {
	done   <-chan struct{}
	signal func(os.Signal) error
	kill   func() error
}

type processStarter func(string) (managedProcess, error)

type Manager struct {
	mu      sync.Mutex
	start   processStarter
	nextID  uint64
	closing bool
	running map[uint64]managedProcess
}

func NewManager() *Manager {
	return newManagerWithStart(startExplorerProcess)
}

func newManagerWithStart(start processStarter) *Manager {
	return &Manager{start: start, running: map[uint64]managedProcess{}}
}

func ServerIDFromArgs(args []string) (string, bool) {
	for index, argument := range args {
		if argument == ServerArgument && index+1 < len(args) {
			serverID := strings.TrimSpace(args[index+1])
			return serverID, serverID != ""
		}
		if strings.HasPrefix(argument, ServerArgument+"=") {
			serverID := strings.TrimSpace(strings.TrimPrefix(argument, ServerArgument+"="))
			return serverID, serverID != ""
		}
	}
	return "", false
}

func Launch(serverID string) error {
	_, err := startExplorerProcess(serverID)
	return err
}

func (m *Manager) Launch(serverID string) error {
	if m == nil {
		return ErrManagerClosed
	}
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return ErrManagerClosed
	}
	process, err := m.start(serverID)
	if err != nil {
		return err
	}
	m.nextID++
	processID := m.nextID
	m.running[processID] = process
	go m.removeWhenDone(processID, process.done)
	return nil
}

func (m *Manager) removeWhenDone(processID uint64, done <-chan struct{}) {
	if done != nil {
		<-done
	}
	m.mu.Lock()
	delete(m.running, processID)
	m.mu.Unlock()
}

func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	m.closing = true
	processes := make([]managedProcess, 0, len(m.running))
	for _, process := range m.running {
		processes = append(processes, process)
	}
	m.mu.Unlock()

	var shutdownErrors []error
	for _, process := range processes {
		if process.signal == nil {
			continue
		}
		if err := process.signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("signal explorer process: %w", err))
		}
	}
	allDone := make(chan struct{})
	go func() {
		for _, process := range processes {
			if process.done != nil {
				<-process.done
			}
		}
		close(allDone)
	}()

	select {
	case <-allDone:
		return errors.Join(shutdownErrors...)
	case <-ctx.Done():
		for _, process := range processes {
			if process.done != nil {
				select {
				case <-process.done:
					continue
				default:
				}
			}
			if process.kill != nil {
				if err := process.kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					shutdownErrors = append(shutdownErrors, fmt.Errorf("force explorer process to stop: %w", err))
				}
			}
		}
		shutdownErrors = append(shutdownErrors, ctx.Err())
		return errors.Join(shutdownErrors...)
	}
}

func startExplorerProcess(serverID string) (managedProcess, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return managedProcess{}, fmt.Errorf("server id is required")
	}
	executable, err := os.Executable()
	if err != nil {
		return managedProcess{}, fmt.Errorf("locate SSH Man executable: %w", err)
	}
	command := exec.Command(executable, ServerArgument, serverID)
	if err := command.Start(); err != nil {
		return managedProcess{}, fmt.Errorf("open server explorer: %w", err)
	}
	done := make(chan struct{})
	go func() {
		_ = command.Wait()
		close(done)
	}()
	return managedProcess{
		done:   done,
		signal: command.Process.Signal,
		kill:   command.Process.Kill,
	}, nil
}
