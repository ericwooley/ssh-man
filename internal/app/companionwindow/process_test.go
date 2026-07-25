package companionwindow

import (
	"context"
	"os"
	"sync"
	"testing"
)

func TestIDFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
		ok   bool
	}{
		{name: "separate", args: []string{"--companion", "server-1"}, want: "server-1", ok: true},
		{name: "inline", args: []string{"--companion=server-2"}, want: "server-2", ok: true},
		{name: "missing", args: []string{"status"}},
		{name: "blank", args: []string{"--companion", "  "}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := IDFromArgs(test.args, "--companion")
			if got != test.want || ok != test.ok {
				t.Fatalf("IDFromArgs() = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestManagerShutdownSignalsAndWaitsForProcesses(t *testing.T) {
	done := make(chan struct{})
	var closeDone sync.Once
	var gotSignal os.Signal
	killCalls := 0
	manager := newManagerWithStart("companion", func(arguments []string) (managedProcess, error) {
		if len(arguments) != 1 || arguments[0] != "server-1" {
			t.Fatalf("arguments = %#v, want server-1", arguments)
		}
		return managedProcess{
			done: done,
			signal: func(signal os.Signal) error {
				gotSignal = signal
				closeDone.Do(func() { close(done) })
				return nil
			},
			kill: func() error {
				killCalls++
				closeDone.Do(func() { close(done) })
				return nil
			},
		}, nil
	})

	if err := manager.Launch("server-1"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotSignal != os.Interrupt || killCalls != 0 {
		t.Fatalf("shutdown signal = %v, kill calls = %d", gotSignal, killCalls)
	}
}

func TestManagerPassesMultipleArgumentsWithoutChangingRemotePaths(t *testing.T) {
	got := []string(nil)
	processDone := make(chan struct{})
	gotSignal := os.Signal(nil)
	manager := newManagerWithStart("preview", func(arguments []string) (managedProcess, error) {
		got = append([]string(nil), arguments...)
		return managedProcess{
			done: processDone,
			signal: func(signal os.Signal) error {
				gotSignal = signal
				return nil
			},
		}, nil
	})

	process, err := manager.LaunchArgsTracked("server-1", "/home/deploy/release notes.md")
	if err != nil {
		t.Fatal(err)
	}
	if process.Done() != (<-chan struct{})(processDone) {
		t.Fatal("tracked launch did not return the child process completion signal")
	}
	if err := process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if gotSignal != os.Interrupt {
		t.Fatalf("tracked signal = %v", gotSignal)
	}
	if len(got) != 2 || got[0] != "server-1" || got[1] != "/home/deploy/release notes.md" {
		t.Fatalf("arguments = %#v", got)
	}
	close(processDone)
}

func TestManagerRejectsBlankArgumentsBeforeStartingProcess(t *testing.T) {
	started := false
	manager := newManagerWithStart("preview", func([]string) (managedProcess, error) {
		started = true
		return managedProcess{}, nil
	})

	if _, err := manager.LaunchArgsTracked("server-1", " "); err == nil {
		t.Fatal("expected a blank remote path to be rejected")
	}
	if started {
		t.Fatal("process started for invalid arguments")
	}
}
