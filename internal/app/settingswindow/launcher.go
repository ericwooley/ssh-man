package settingswindow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

const Argument = "--ssh-man-settings"

var ErrManagerClosed = errors.New("the settings process manager is shutting down")

type managedProcess struct {
	done   <-chan struct{}
	signal func(os.Signal) error
	kill   func() error
}

type processStarter func() (managedProcess, error)

type Manager struct {
	mu      sync.Mutex
	start   processStarter
	nextID  uint64
	closing bool
	running map[uint64]managedProcess
}

func NewManager() *Manager {
	return newManagerWithStart(startSettingsProcess)
}

func newManagerWithStart(start processStarter) *Manager {
	return &Manager{start: start, running: map[uint64]managedProcess{}}
}

func FromArgs(args []string) bool {
	for _, argument := range args {
		if argument == Argument {
			return true
		}
	}
	return false
}

func Launch() error {
	_, err := startSettingsProcess()
	return err
}

func (m *Manager) Launch() error {
	if m == nil {
		return ErrManagerClosed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return ErrManagerClosed
	}
	process, err := m.start()
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
			shutdownErrors = append(shutdownErrors, fmt.Errorf("signal settings process: %w", err))
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
					shutdownErrors = append(shutdownErrors, fmt.Errorf("force settings process to stop: %w", err))
				}
			}
		}
		shutdownErrors = append(shutdownErrors, ctx.Err())
		return errors.Join(shutdownErrors...)
	}
}

func startSettingsProcess() (managedProcess, error) {
	executable, err := os.Executable()
	if err != nil {
		return managedProcess{}, fmt.Errorf("locate SSH Man executable: %w", err)
	}
	command := exec.Command(executable, Argument)
	if err := command.Start(); err != nil {
		return managedProcess{}, fmt.Errorf("open settings: %w", err)
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
