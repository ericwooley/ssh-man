package bindings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	commandhistorydomain "ssh-man/internal/domain/commandhistory"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/commandrunner"
)

const commandConnectTimeout = 15 * time.Second

type commandRunner interface {
	Connect(context.Context, string) (commandrunner.ConnectResult, error)
	Run(string) (commandrunner.ExecutionResult, error)
	CompletePath(string) (commandrunner.CompletionResult, error)
	Cancel() error
	Close() error
}

type commandHistory interface {
	Record(context.Context, commandhistorydomain.RecordInput) (commandhistorydomain.Entry, error)
	List(context.Context, string) ([]commandhistorydomain.Entry, error)
	Delete(context.Context, string, string) error
}

type CommandInitialState struct {
	Server  serverdomain.Server          `json:"server"`
	History []commandhistorydomain.Entry `json:"history"`
}

type CommandBindings struct {
	app     *bootstrap.Application
	server  serverdomain.Server
	runner  commandRunner
	history commandHistory
	window  *appwindow.Controller
}

func NewCommandBindings(app *bootstrap.Application, server serverdomain.Server, window *appwindow.Controller) *CommandBindings {
	if window == nil {
		window = appwindow.New()
	}
	return newCommandBindingsWithDependencies(
		app,
		server,
		commandrunner.NewService(server),
		app.CommandHistoryService,
		window,
	)
}

func newCommandBindingsWithDependencies(
	app *bootstrap.Application,
	server serverdomain.Server,
	runner commandRunner,
	history commandHistory,
	window *appwindow.Controller,
) *CommandBindings {
	if window == nil {
		window = appwindow.New()
	}
	return &CommandBindings{
		app: app, server: server, runner: runner, history: history, window: window,
	}
}

func (b *CommandBindings) SetContext(ctx context.Context) {
	b.window.SetContext(ctx)
}

func (b *CommandBindings) InitialState() (CommandInitialState, error) {
	entries, err := b.history.List(context.Background(), b.server.ID)
	if err != nil {
		return CommandInitialState{}, err
	}
	return CommandInitialState{Server: b.server, History: entries}, nil
}

func (b *CommandBindings) Connect(passphrase string) (commandrunner.ConnectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandConnectTimeout)
	defer cancel()
	return b.runner.Connect(ctx, passphrase)
}

func (b *CommandBindings) CompletePath(pathPrefix string) (commandrunner.CompletionResult, error) {
	return b.runner.CompletePath(pathPrefix)
}

func (b *CommandBindings) RunCommand(command string) (commandhistorydomain.Entry, error) {
	if strings.TrimSpace(command) == "" {
		return commandhistorydomain.Entry{}, fmt.Errorf("command is required")
	}
	if len(command) > 32*1024 {
		return commandhistorydomain.Entry{}, fmt.Errorf("command is too long")
	}
	result, err := b.runner.Run(command)
	if err != nil {
		return commandhistorydomain.Entry{}, err
	}
	entry, err := b.history.Record(context.Background(), commandhistorydomain.RecordInput{
		ServerID:  b.server.ID,
		Command:   command,
		Output:    result.Output,
		ExitCode:  result.ExitCode,
		StartedAt: result.StartedAt,
		EndedAt:   result.EndedAt,
		Truncated: result.Truncated,
		Error:     result.Error,
	})
	if err != nil {
		return commandhistorydomain.Entry{}, err
	}
	return entry, nil
}

func (b *CommandBindings) CancelCommand() error {
	return b.runner.Cancel()
}

func (b *CommandBindings) DeleteHistory(entryID string) error {
	return b.history.Delete(context.Background(), b.server.ID, entryID)
}

func (b *CommandBindings) Close() error {
	return b.window.Quit()
}

func (b *CommandBindings) Shutdown(ctx context.Context) error {
	var appErr error
	if b.app != nil {
		appErr = b.app.Shutdown(ctx)
	}
	return errors.Join(b.runner.Close(), appErr)
}
