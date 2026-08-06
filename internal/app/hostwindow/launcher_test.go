package hostwindow

import (
	"context"
	"os"
	"sync"
	"testing"
)

func TestServerIDFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
		ok   bool
	}{
		{name: "separate", args: []string{ServerArgument, "server-1"}, want: "server-1", ok: true},
		{name: "inline", args: []string{ServerArgument + "=server-2"}, want: "server-2", ok: true},
		{name: "missing", args: []string{"status"}},
		{name: "blank", args: []string{ServerArgument, "  "}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ServerIDFromArgs(test.args)
			if got != test.want || ok != test.ok {
				t.Fatalf("ServerIDFromArgs() = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestManagerShutdownSignalsAndWaitsForHostProcesses(t *testing.T) {
	done := make(chan struct{})
	var closeDone sync.Once
	var gotSignal os.Signal
	manager := newManagerWithStart(func(serverID string) (managedProcess, error) {
		return managedProcess{
			done: done,
			signal: func(signal os.Signal) error {
				gotSignal = signal
				closeDone.Do(func() { close(done) })
				return nil
			},
			kill: func() error {
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
	if gotSignal != os.Interrupt {
		t.Fatalf("shutdown signal = %v, want %v", gotSignal, os.Interrupt)
	}
}
