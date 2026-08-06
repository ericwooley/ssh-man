package hostwindow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const ServerArgument = "--ssh-man-host"

var ErrManagerClosed = errors.New("the host process manager is shutting down")

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
	return newManagerWithStart(startHostProcess)
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
	_, err := startHostProcess(serverID)
	return err
}

func (manager *Manager) Launch(serverID string) error {
	if manager == nil {
		return ErrManagerClosed
	}
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.closing {
		return ErrManagerClosed
	}
	process, err := manager.start(serverID)
	if err != nil {
		return err
	}
	manager.nextID++
	processID := manager.nextID
	manager.running[processID] = process
	go manager.removeWhenDone(processID, process.done)
	return nil
}

func (manager *Manager) removeWhenDone(processID uint64, done <-chan struct{}) {
	if done != nil {
		<-done
	}
	manager.mu.Lock()
	delete(manager.running, processID)
	manager.mu.Unlock()
}

func (manager *Manager) Shutdown(ctx context.Context) error {
	if manager == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	manager.mu.Lock()
	manager.closing = true
	processes := make([]managedProcess, 0, len(manager.running))
	for _, process := range manager.running {
		processes = append(processes, process)
	}
	manager.mu.Unlock()

	var shutdownErrors []error
	for _, process := range processes {
		if process.signal == nil {
			continue
		}
		if err := process.signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("signal host window: %w", err))
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
					shutdownErrors = append(shutdownErrors, fmt.Errorf("force host window to stop: %w", err))
				}
			}
		}
		shutdownErrors = append(shutdownErrors, ctx.Err())
		return errors.Join(shutdownErrors...)
	}
}

func startHostProcess(serverID string) (managedProcess, error) {
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
		return managedProcess{}, fmt.Errorf("open host window: %w", err)
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
