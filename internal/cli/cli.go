// Package cli implements the scripting interface for the SSH Man menu-bar agent.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"ssh-man/internal/buildinfo"
	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	sessiondomain "ssh-man/internal/domain/session"
)

const (
	ExitOK           = 0
	ExitOperation    = 1
	ExitUsage        = 2
	ExitSelector     = 3
	ExitUnavailable  = 4
	ExitTunnelFailed = 5
	ExitAttention    = 6
	ExitTimeout      = 7
)

type Caller interface {
	Call(context.Context, control.Request, any) error
}

type ConnectOptions struct {
	Autostart      bool
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
}

type Connector func(context.Context, ConnectOptions) (Caller, error)

type Dependencies struct {
	Connect      Connector
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	IsTerminal   func() bool
	ReadPassword func() ([]byte, error)
	Version      string
}

func DefaultDependencies() Dependencies {
	return Dependencies{
		Connect:      defaultConnector(defaultLauncher),
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		IsTerminal:   stdinIsTerminal,
		ReadPassword: readTerminalPassword,
		Version:      buildinfo.Version,
	}
}

// IsCLIInvocation keeps the no-argument desktop behavior and ignores the
// legacy process serial argument supplied by some macOS LaunchServices paths.
func IsCLIInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return !(len(args) == 1 && strings.HasPrefix(args[0], "-psn_"))
}

func Run(ctx context.Context, args []string, dependencies Dependencies) int {
	dependencies = normalizeDependencies(dependencies)
	options, remaining, err := parseGlobalOptions(args)
	if err != nil {
		return reportError(dependencies.Stderr, err)
	}

	if options.Version || (len(remaining) == 1 && remaining[0] == "version") {
		return printVersion(dependencies, options.Output)
	}
	if options.Help || len(remaining) == 0 || remaining[0] == "help" {
		_, _ = io.WriteString(dependencies.Stdout, usageFor(remaining))
		return ExitOK
	}

	runtime := commandRuntime{ctx: ctx, dependencies: dependencies, globals: options}
	if err := runtime.dispatch(remaining); err != nil {
		return reportError(dependencies.Stderr, err)
	}
	return ExitOK
}

func normalizeDependencies(dependencies Dependencies) Dependencies {
	if dependencies.Stdin == nil {
		dependencies.Stdin = strings.NewReader("")
	}
	if dependencies.Stdout == nil {
		dependencies.Stdout = io.Discard
	}
	if dependencies.Stderr == nil {
		dependencies.Stderr = io.Discard
	}
	if dependencies.Version == "" {
		dependencies.Version = "dev"
	}
	if dependencies.IsTerminal == nil {
		dependencies.IsTerminal = func() bool { return false }
	}
	return dependencies
}

func printVersion(dependencies Dependencies, mode string) int {
	if mode == "json" || mode == "jsonl" {
		if err := writeOutput(dependencies.Stdout, mode, map[string]string{"version": dependencies.Version}, table{}); err != nil {
			return reportError(dependencies.Stderr, err)
		}
		return ExitOK
	}
	_, _ = fmt.Fprintf(dependencies.Stdout, "ssh-man %s\n", dependencies.Version)
	return ExitOK
}

type exitError struct {
	code    int
	message string
}

func (e *exitError) Error() string { return e.message }

func reportError(writer io.Writer, err error) int {
	if err == nil {
		return ExitOK
	}
	code := ExitOperation
	var usage *usageError
	var selector *selectorError
	var exited *exitError
	switch {
	case errors.As(err, &usage):
		code = ExitUsage
	case errors.As(err, &selector):
		code = ExitSelector
	case errors.As(err, &exited):
		code = exited.code
	}
	_, _ = fmt.Fprintf(writer, "ssh-man: %s\n", err)
	return code
}

type commandRuntime struct {
	ctx          context.Context
	dependencies Dependencies
	globals      globalOptions
	client       Caller
}

func (r *commandRuntime) connect(autostart bool) (Caller, error) {
	if r.client != nil {
		return r.client, nil
	}
	if r.dependencies.Connect == nil {
		return nil, &exitError{code: ExitUnavailable, message: "SSH Man control client is unavailable"}
	}
	client, err := r.dependencies.Connect(r.ctx, ConnectOptions{
		Autostart:      autostart && !r.globals.NoAutostart,
		ConnectTimeout: r.globals.ConnectTimeout,
		RequestTimeout: r.globals.RequestTimeout,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &exitError{code: ExitTimeout, message: err.Error()}
		}
		return nil, &exitError{code: ExitUnavailable, message: err.Error()}
	}
	r.client = client
	return client, nil
}

func (r *commandRuntime) call(autostart bool, request control.Request, output any) error {
	client, err := r.connect(autostart)
	if err != nil {
		return err
	}
	requestContext, cancel := context.WithTimeout(r.ctx, r.globals.RequestTimeout)
	defer cancel()
	if err := client.Call(requestContext, request, output); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return &exitError{code: ExitTimeout, message: err.Error()}
		}
		return err
	}
	return nil
}

func (r *commandRuntime) state() (control.State, error) {
	var state control.State
	if err := r.call(true, control.Request{Command: "state"}, &state); err != nil {
		return control.State{}, err
	}
	if state.Servers == nil {
		state.Servers = []control.ServerRecord{}
	}
	for index := range state.Servers {
		if state.Servers[index].Configurations == nil {
			state.Servers[index].Configurations = []configdomain.ConnectionConfiguration{}
		}
	}
	if state.Sessions == nil {
		state.Sessions = []sessiondomain.RuntimeSession{}
	}
	return state, nil
}
