package settingswindow

import (
	"context"
	"os"
	"sync"
	"testing"
)

func TestFromArgsRecognizesSettingsWindow(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "present", args: []string{Argument}, want: true},
		{name: "among other arguments", args: []string{"--debug", Argument}, want: true},
		{name: "missing", args: []string{"status"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FromArgs(test.args); got != test.want {
				t.Fatalf("FromArgs() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestManagerShutdownSignalsAndWaitsForSettingsProcesses(t *testing.T) {
	done := make(chan struct{})
	var closeDone sync.Once
	var gotSignal os.Signal
	killCalls := 0
	manager := newManagerWithStart(func() (managedProcess, error) {
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

	if err := manager.Launch(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	if gotSignal != os.Interrupt {
		t.Fatalf("shutdown signal = %v, want %v", gotSignal, os.Interrupt)
	}
	if killCalls != 0 {
		t.Fatalf("kill calls = %d, want graceful shutdown", killCalls)
	}
}
