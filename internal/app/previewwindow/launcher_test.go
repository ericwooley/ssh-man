package previewwindow

import (
	"context"
	"os"
	"sync"
	"testing"

	"ssh-man/internal/app/companionwindow"
)

type fakeProcess struct {
	done    chan struct{}
	signals chan os.Signal
}

func (p *fakeProcess) Done() <-chan struct{} {
	return p.done
}

func (p *fakeProcess) Signal(signal os.Signal) error {
	p.signals <- signal
	return nil
}

type fakeProcessManager struct {
	mu       sync.Mutex
	launches [][]string
	process  []*fakeProcess
}

func (f *fakeProcessManager) LaunchArgsTracked(arguments ...string) (companionwindow.Process, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	process := &fakeProcess{
		done:    make(chan struct{}),
		signals: make(chan os.Signal, 1),
	}
	f.launches = append(f.launches, append([]string(nil), arguments...))
	f.process = append(f.process, process)
	return process, nil
}

func (f *fakeProcessManager) Shutdown(context.Context) error {
	return nil
}

func (f *fakeProcessManager) closeLaunch(index int) {
	f.mu.Lock()
	done := f.process[index].done
	f.mu.Unlock()
	close(done)
}

func TestRequestFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want Request
		ok   bool
	}{
		{
			name: "separate",
			args: []string{PreviewArgument, "server-1", "/var/www/index.html"},
			want: Request{ServerID: "server-1", RemotePath: "/var/www/index.html"},
			ok:   true,
		},
		{
			name: "path with spaces",
			args: []string{PreviewArgument, "server-2", "/home/deploy/release notes.md"},
			want: Request{ServerID: "server-2", RemotePath: "/home/deploy/release notes.md"},
			ok:   true,
		},
		{name: "missing path", args: []string{PreviewArgument, "server-1"}},
		{name: "blank server", args: []string{PreviewArgument, " ", "/tmp/file.txt"}},
		{name: "blank path", args: []string{PreviewArgument, "server-1", " "}},
		{name: "unrelated", args: []string{"status"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := RequestFromArgs(test.args)
			if got != test.want || ok != test.ok {
				t.Fatalf("RequestFromArgs() = %#v, %v; want %#v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestInstanceIDIsStablePerServerAndPath(t *testing.T) {
	first := InstanceID(Request{ServerID: "server-1", RemotePath: "/var/www/index.html"})
	same := InstanceID(Request{ServerID: "server-1", RemotePath: "/var/www/index.html"})
	differentPath := InstanceID(Request{ServerID: "server-1", RemotePath: "/var/www/app.js"})
	differentServer := InstanceID(Request{ServerID: "server-2", RemotePath: "/var/www/index.html"})

	if first == "" || first != same {
		t.Fatalf("stable IDs = %q and %q", first, same)
	}
	if first == differentPath || first == differentServer {
		t.Fatalf("instance ID %q should be unique per server and path", first)
	}
}

func TestManagerRejectsInvalidPreviewRequestBeforeLaunching(t *testing.T) {
	manager := NewManager()
	if err := manager.Launch("", "/tmp/report.pdf"); err == nil {
		t.Fatal("expected blank server ID to fail")
	}
	if err := manager.Launch("server-1", " "); err == nil {
		t.Fatal("expected blank remote path to fail")
	}
}

func TestManagerReportsPreviewOpenAndClosed(t *testing.T) {
	processes := &fakeProcessManager{}
	manager := newManagerWithProcesses(processes)
	states := make(chan State, 2)
	manager.SetStateListener(func(state State) {
		states <- state
	})

	if err := manager.Launch("server-1", "/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}
	if !manager.IsOpen("/tmp/report.pdf") {
		t.Fatal("preview should be open after launch")
	}
	if state := <-states; state != (State{RemotePath: "/tmp/report.pdf", Open: true}) {
		t.Fatalf("open state = %#v", state)
	}

	processes.closeLaunch(0)

	if state := <-states; state != (State{RemotePath: "/tmp/report.pdf", Open: false}) {
		t.Fatalf("closed state = %#v", state)
	}
	if manager.IsOpen("/tmp/report.pdf") {
		t.Fatal("preview should return inline after its process exits")
	}
}

func TestManagerFocusesExistingPreviewWithoutLaunchingAnotherProcess(t *testing.T) {
	processes := &fakeProcessManager{}
	manager := newManagerWithProcesses(processes)

	if err := manager.Launch("server-1", "/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Focus("/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}

	if len(processes.launches) != 1 {
		t.Fatalf("focus launched %d preview processes, want 1 total", len(processes.launches))
	}
	if signal := <-processes.process[0].signals; signal != FocusSignal() {
		t.Fatalf("focus signal = %v, want %v", signal, FocusSignal())
	}
}

func TestManagerDoesNotLaunchDuplicatePreviewForOpenPath(t *testing.T) {
	processes := &fakeProcessManager{}
	manager := newManagerWithProcesses(processes)

	if err := manager.Launch("server-1", "/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Launch("server-1", "/tmp/report.pdf"); err != nil {
		t.Fatal(err)
	}

	if len(processes.launches) != 1 {
		t.Fatalf("duplicate open launched %d processes, want 1", len(processes.launches))
	}
}
