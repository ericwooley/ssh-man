package companionwindow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var ErrManagerClosed = errors.New("the companion process manager is shutting down")

type Process interface {
	Done() <-chan struct{}
	Signal(os.Signal) error
}

type managedProcess struct {
	done   <-chan struct{}
	signal func(os.Signal) error
	kill   func() error
}

type trackedProcess struct {
	process managedProcess
}

func (p trackedProcess) Done() <-chan struct{} {
	return p.process.done
}

func (p trackedProcess) Signal(signal os.Signal) error {
	if p.process.signal == nil {
		return fmt.Errorf("companion process signal is unavailable")
	}
	return p.process.signal(signal)
}

type processStarter func([]string) (managedProcess, error)

type Manager struct {
	mu      sync.Mutex
	start   processStarter
	action  string
	nextID  uint64
	closing bool
	running map[uint64]managedProcess
}

func NewManager(argument, action string) *Manager {
	return newManagerWithStart(action, func(arguments []string) (managedProcess, error) {
		return startProcess(argument, action, arguments)
	})
}

func newManagerWithStart(action string, start processStarter) *Manager {
	return &Manager{start: start, action: action, running: map[uint64]managedProcess{}}
}

func IDFromArgs(args []string, argument string) (string, bool) {
	for index, value := range args {
		if value == argument && index+1 < len(args) {
			serverID := strings.TrimSpace(args[index+1])
			return serverID, serverID != ""
		}
		if strings.HasPrefix(value, argument+"=") {
			serverID := strings.TrimSpace(strings.TrimPrefix(value, argument+"="))
			return serverID, serverID != ""
		}
	}
	return "", false
}

func Launch(argument, action, serverID string) error {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	_, err := startProcess(argument, action, []string{serverID})
	return err
}

func (m *Manager) Launch(serverID string) error {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	_, err := m.launch([]string{serverID})
	return err
}

// LaunchArgsTracked starts a managed child process and returns a signal that
// closes when that exact process exits.
func (m *Manager) LaunchArgsTracked(arguments ...string) (Process, error) {
	arguments, err := validateArguments(arguments)
	if err != nil {
		return nil, err
	}
	return m.launch(arguments)
}

func validateArguments(arguments []string) ([]string, error) {
	if len(arguments) == 0 {
		return nil, fmt.Errorf("companion process arguments are required")
	}
	normalized := append([]string(nil), arguments...)
	for _, argument := range normalized {
		if strings.TrimSpace(argument) == "" {
			return nil, fmt.Errorf("companion process arguments cannot be blank")
		}
	}
	return normalized, nil
}

func (m *Manager) launch(arguments []string) (Process, error) {
	if m == nil {
		return nil, ErrManagerClosed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return nil, ErrManagerClosed
	}
	process, err := m.start(arguments)
	if err != nil {
		return nil, err
	}
	m.nextID++
	processID := m.nextID
	m.running[processID] = process
	go m.removeWhenDone(processID, process.done)
	return trackedProcess{process: process}, nil
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
			shutdownErrors = append(shutdownErrors, fmt.Errorf("signal %s process: %w", m.action, err))
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
					shutdownErrors = append(shutdownErrors, fmt.Errorf("force %s process to stop: %w", m.action, err))
				}
			}
		}
		shutdownErrors = append(shutdownErrors, ctx.Err())
		return errors.Join(shutdownErrors...)
	}
}

func startProcess(argument, action string, arguments []string) (managedProcess, error) {
	arguments, err := validateArguments(arguments)
	if err != nil {
		return managedProcess{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return managedProcess{}, fmt.Errorf("locate SSH Man executable: %w", err)
	}
	commandArguments := append([]string{argument}, arguments...)
	command := exec.Command(executable, commandArguments...)
	if err := command.Start(); err != nil {
		return managedProcess{}, fmt.Errorf("open %s: %w", action, err)
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
