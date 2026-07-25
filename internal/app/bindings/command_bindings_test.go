package bindings

import (
	"context"
	"errors"
	"testing"
	"time"

	appwindow "ssh-man/internal/app/window"
	commandhistorydomain "ssh-man/internal/domain/commandhistory"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/commandrunner"
)

type fakeCommandRunner struct {
	connectResult commandrunner.ConnectResult
	result        commandrunner.ExecutionResult
	completions   commandrunner.CompletionResult
	command       string
	cancelled     bool
}

func (f *fakeCommandRunner) Connect(context.Context, string) (commandrunner.ConnectResult, error) {
	return f.connectResult, nil
}
func (f *fakeCommandRunner) Run(command string) (commandrunner.ExecutionResult, error) {
	f.command = command
	return f.result, nil
}
func (f *fakeCommandRunner) CompletePath(string) (commandrunner.CompletionResult, error) {
	return f.completions, nil
}
func (f *fakeCommandRunner) Cancel() error { f.cancelled = true; return nil }
func (f *fakeCommandRunner) Close() error  { return nil }

type fakeCommandHistory struct {
	entries       []commandhistorydomain.Entry
	recorded      commandhistorydomain.RecordInput
	deletedServer string
	deletedEntry  string
	err           error
}

func (f *fakeCommandHistory) Record(_ context.Context, input commandhistorydomain.RecordInput) (commandhistorydomain.Entry, error) {
	if f.err != nil {
		return commandhistorydomain.Entry{}, f.err
	}
	f.recorded = input
	entry := commandhistorydomain.Entry{
		ID: "history-1", ServerID: input.ServerID, Command: input.Command,
		Output: input.Output, ExitCode: input.ExitCode, StartedAt: input.StartedAt,
		EndedAt: input.EndedAt, Truncated: input.Truncated, Error: input.Error,
	}
	f.entries = append([]commandhistorydomain.Entry{entry}, f.entries...)
	return entry, nil
}
func (f *fakeCommandHistory) List(context.Context, string) ([]commandhistorydomain.Entry, error) {
	return f.entries, f.err
}
func (f *fakeCommandHistory) Delete(_ context.Context, serverID, entryID string) error {
	f.deletedServer = serverID
	f.deletedEntry = entryID
	return f.err
}

func TestCommandBindingsLoadsServerHistoryAndRecordsExecution(t *testing.T) {
	startedAt := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	runner := &fakeCommandRunner{result: commandrunner.ExecutionResult{
		Output: "ok\n", ExitCode: 0, StartedAt: startedAt, EndedAt: startedAt.Add(time.Second),
	}}
	history := &fakeCommandHistory{entries: []commandhistorydomain.Entry{{ID: "old"}}}
	binding := newCommandBindingsWithDependencies(
		nil,
		serverdomain.Server{ID: "server-1", Name: "Production"},
		runner,
		history,
		appwindow.New(),
	)

	state, err := binding.InitialState()
	if err != nil || state.Server.Name != "Production" || len(state.History) != 1 {
		t.Fatalf("InitialState() = %#v, %v", state, err)
	}
	entry, err := binding.RunCommand("printf ok")
	if err != nil {
		t.Fatal(err)
	}
	if runner.command != "printf ok" || history.recorded.ServerID != "server-1" || entry.Output != "ok\n" {
		t.Fatalf("run state = command %q, recorded %#v, entry %#v", runner.command, history.recorded, entry)
	}
}

func TestCommandBindingsDeletesOnlyFromItsServerAndCancels(t *testing.T) {
	runner := &fakeCommandRunner{}
	history := &fakeCommandHistory{}
	binding := newCommandBindingsWithDependencies(
		nil, serverdomain.Server{ID: "server-1"}, runner, history, appwindow.New(),
	)

	if err := binding.DeleteHistory("history-1"); err != nil {
		t.Fatal(err)
	}
	if history.deletedServer != "server-1" || history.deletedEntry != "history-1" {
		t.Fatalf("deleted = %q / %q", history.deletedServer, history.deletedEntry)
	}
	if err := binding.CancelCommand(); err != nil || !runner.cancelled {
		t.Fatalf("CancelCommand() = %v, cancelled = %v", err, runner.cancelled)
	}
}

func TestCommandBindingsDoesNotLoseStorageFailures(t *testing.T) {
	storeErr := errors.New("database unavailable")
	binding := newCommandBindingsWithDependencies(
		nil,
		serverdomain.Server{ID: "server-1"},
		&fakeCommandRunner{result: commandrunner.ExecutionResult{StartedAt: time.Now(), EndedAt: time.Now()}},
		&fakeCommandHistory{err: storeErr},
		appwindow.New(),
	)

	if _, err := binding.RunCommand("pwd"); !errors.Is(err, storeErr) {
		t.Fatalf("RunCommand() error = %v", err)
	}
}
