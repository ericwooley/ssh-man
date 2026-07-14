package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

type fakeCaller struct {
	requests []control.Request
	handle   func(control.Request) (any, error)
}

func (f *fakeCaller) Call(_ context.Context, request control.Request, output any) error {
	f.requests = append(f.requests, request)
	value, err := f.handle(request)
	if err != nil || output == nil || value == nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, output)
}

func fixtureState() control.State {
	return control.State{
		Servers: []control.ServerRecord{
			{
				Server: serverdomain.Server{ID: "server-prod", Name: "Production", Host: "prod.example.com", Port: 22, Username: "deploy", AuthMode: serverdomain.AuthModeAgent},
				Configurations: []configdomain.ConnectionConfiguration{
					{ID: "tunnel-docs", ServerID: "server-prod", Label: "Docs", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 3000, RemoteHost: "127.0.0.1", RemotePort: 8080, AutoReconnectEnabled: true},
					{ID: "tunnel-socks", ServerID: "server-prod", Label: "Browser", ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 0, AutoReconnectEnabled: true},
				},
			},
			{
				Server: serverdomain.Server{ID: "server-stage", Name: "Staging", Host: "stage.example.com", Port: 2222, Username: "stage", AuthMode: serverdomain.AuthModePrivateKey, KeyReference: "~/.ssh/stage"},
				Configurations: []configdomain.ConnectionConfiguration{
					{ID: "tunnel-stage-docs", ServerID: "server-stage", Label: "Docs", ConnectionType: configdomain.ConnectionTypeLocalForward, LocalPort: 3001, RemoteHost: "127.0.0.1", RemotePort: 8081},
				},
			},
		},
		Preferences:     preferencesdomain.UserPreference{Theme: preferencesdomain.ThemeDark, LastSelectedServerID: "server-prod"},
		Sessions:        []sessiondomain.RuntimeSession{{ConfigurationID: "tunnel-docs", Status: sessiondomain.StatusConnected, BoundPort: 3000, StatusDetail: "Listening"}},
		Diagnostics:     control.Diagnostics{AppDataPath: "/tmp/ssh-man", DatabasePath: "/tmp/ssh-man/ssh-man.db"},
		CurrentUsername: "local-user",
	}
}

func runWithCaller(t *testing.T, caller *fakeCaller, stdin string, args ...string) (int, string, string, int) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	connectCalls := 0
	dependencies := Dependencies{
		Connect: func(context.Context, ConnectOptions) (Caller, error) {
			connectCalls++
			return caller, nil
		},
		Stdin:      strings.NewReader(stdin),
		Stdout:     &stdout,
		Stderr:     &stderr,
		IsTerminal: func() bool { return false },
		Version:    "1.2.3",
	}
	exitCode := Run(context.Background(), args, dependencies)
	return exitCode, stdout.String(), stderr.String(), connectCalls
}

func TestIsCLIInvocationPreservesDesktopLaunches(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: nil, want: false},
		{args: []string{"-psn_0_12345"}, want: false},
		{args: []string{"--help"}, want: true},
		{args: []string{"server", "list"}, want: true},
		{args: []string{"unknown"}, want: true},
	}
	for _, test := range tests {
		if got := IsCLIInvocation(test.args); got != test.want {
			t.Fatalf("IsCLIInvocation(%q) = %v, want %v", test.args, got, test.want)
		}
	}
}

func TestHelpAndVersionNeverConnect(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "root help", args: []string{"--help"}, want: "Usage:"},
		{name: "nested help", args: []string{"server", "--help"}, want: "ssh-man server add"},
		{name: "app help", args: []string{"app", "--help"}, want: "ssh-man app status"},
		{name: "browser help", args: []string{"browser", "--help"}, want: "ssh-man browser preview"},
		{name: "settings help", args: []string{"settings", "--help"}, want: "ssh-man settings set"},
		{name: "version", args: []string{"version"}, want: "ssh-man 1.2.3"},
		{name: "json version", args: []string{"version", "--output", "json"}, want: `"version": "1.2.3"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &fakeCaller{handle: func(control.Request) (any, error) { return nil, errors.New("must not be called") }}
			code, stdout, stderr, connects := runWithCaller(t, caller, "", test.args...)
			if code != ExitOK || stderr != "" || connects != 0 || !strings.Contains(stdout, test.want) {
				t.Fatalf("code=%d connects=%d stdout=%q stderr=%q", code, connects, stdout, stderr)
			}
		})
	}
}

func TestInvalidSyntaxIsRejectedBeforeConnecting(t *testing.T) {
	caller := &fakeCaller{handle: func(control.Request) (any, error) { return nil, errors.New("must not be called") }}
	for _, args := range [][]string{
		{"unknown"},
		{"settings", "set", "--theme", "sepia"},
		{"server", "delete", "Production"},
		{"tunnel", "add", "socks", "Proxy", "--server", "Production", "--remote", "host:80"},
	} {
		code, _, _, connects := runWithCaller(t, caller, "", args...)
		if code != ExitUsage || connects != 0 {
			t.Fatalf("Run(%q) code=%d connects=%d", args, code, connects)
		}
	}
}

func TestSelectorsUseIDThenCaseInsensitiveExactName(t *testing.T) {
	state := fixtureState()
	server, err := resolveServer(state, "production")
	if err != nil || server.Server.ID != "server-prod" {
		t.Fatalf("resolveServer() = %+v, %v", server, err)
	}
	tunnel, err := resolveTunnel(state, "browser", "PRODUCTION")
	if err != nil || tunnel.Configuration.ID != "tunnel-socks" {
		t.Fatalf("resolveTunnel() = %+v, %v", tunnel, err)
	}
	if _, err := resolveTunnel(state, "Docs", ""); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous tunnel error = %v", err)
	}
	byID, err := resolveTunnel(state, "tunnel-stage-docs", "")
	if err != nil || byID.Server.ID != "server-stage" {
		t.Fatalf("ID lookup = %+v, %v", byID, err)
	}
}

func TestResolveServerRejectsCaseInsensitiveNameAmbiguity(t *testing.T) {
	state := fixtureState()
	state.Servers = append(state.Servers, control.ServerRecord{Server: serverdomain.Server{ID: "server-prod-two", Name: "PRODUCTION"}})
	_, err := resolveServer(state, "production")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "server-prod") || !strings.Contains(err.Error(), "server-prod-two") {
		t.Fatalf("resolveServer ambiguity error = %v", err)
	}
	byID, err := resolveServer(state, "server-prod-two")
	if err != nil || byID.Server.ID != "server-prod-two" {
		t.Fatalf("resolveServer ID = %+v, %v", byID, err)
	}
}

func TestTunnelRecordsSynthesizeStoppedAndSortStably(t *testing.T) {
	records := tunnelRecords(fixtureState())
	if len(records) != 3 {
		t.Fatalf("len(records) = %d", len(records))
	}
	if records[0].Configuration.ID != "tunnel-socks" || records[1].Configuration.ID != "tunnel-docs" || records[2].Configuration.ID != "tunnel-stage-docs" {
		t.Fatalf("unexpected order: %+v", records)
	}
	if records[0].Session.Status != sessiondomain.StatusStopped {
		t.Fatalf("missing runtime status = %q", records[0].Session.Status)
	}
}

func TestTunnelAddLocalBuildsValidatedControlRequest(t *testing.T) {
	var saved configdomain.ConnectionConfiguration
	caller := &fakeCaller{handle: func(request control.Request) (any, error) {
		switch request.Command {
		case "state":
			return fixtureState(), nil
		case "configuration.save":
			saved = *request.Configuration
			saved.ID = "new-tunnel"
			return saved, nil
		default:
			return nil, fmt.Errorf("unexpected command %s", request.Command)
		}
	}}
	code, stdout, stderr, _ := runWithCaller(t, caller, "", "--output", "json", "tunnel", "add", "local", "Admin", "--server", "production", "--listen", "9000", "--remote", "internal.example:443", "--reconnect=false", "--notes", "admin endpoint")
	if code != ExitOK || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if saved.ServerID != "server-prod" || saved.LocalPort != 9000 || saved.RemoteHost != "internal.example" || saved.RemotePort != 443 || saved.AutoReconnectEnabled {
		t.Fatalf("saved configuration = %+v", saved)
	}
	if !strings.Contains(stdout, `"id": "new-tunnel"`) {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestServerAddInfersPrivateKeyAuthFromKeyFlag(t *testing.T) {
	var saved serverdomain.Server
	caller := &fakeCaller{handle: func(request control.Request) (any, error) {
		switch request.Command {
		case "state":
			return fixtureState(), nil
		case "server.save":
			saved = *request.Server
			saved.ID = "new-server"
			return saved, nil
		default:
			return nil, fmt.Errorf("unexpected command %s", request.Command)
		}
	}}
	code, _, stderr, _ := runWithCaller(t, caller, "", "server", "add", "Key host", "--host", "key.example.com", "--key", "~/.ssh/id_ed25519")
	if code != ExitOK || stderr != "" || saved.AuthMode != serverdomain.AuthModePrivateKey || saved.KeyReference != "~/.ssh/id_ed25519" {
		t.Fatalf("code=%d saved=%+v stderr=%q", code, saved, stderr)
	}
}

func TestTunnelStartMapsNeedsAttentionToExitCode(t *testing.T) {
	caller := &fakeCaller{handle: func(request control.Request) (any, error) {
		switch request.Command {
		case "state":
			return fixtureState(), nil
		case "session.start":
			if request.ConfigurationID != "tunnel-socks" {
				t.Fatalf("configuration ID = %q", request.ConfigurationID)
			}
			return sessiondomain.RuntimeSession{ConfigurationID: request.ConfigurationID, Status: sessiondomain.StatusNeedsAttention, StatusDetail: "Unlock the SSH key"}, nil
		default:
			return nil, fmt.Errorf("unexpected command %s", request.Command)
		}
	}}
	code, stdout, stderr, _ := runWithCaller(t, caller, "", "tunnel", "start", "Browser", "--server", "Production")
	if code != ExitAttention || !strings.Contains(stdout, "needs_attention") || !strings.Contains(stderr, "Unlock the SSH key") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestUnlockReadsPassphraseOnlyFromStdin(t *testing.T) {
	secret := ""
	caller := &fakeCaller{handle: func(request control.Request) (any, error) {
		switch request.Command {
		case "state":
			return fixtureState(), nil
		case "session.unlock":
			secret = request.Secret
			return sessiondomain.RuntimeSession{ConfigurationID: request.ConfigurationID, Status: sessiondomain.StatusConnected, BoundPort: 3000}, nil
		default:
			return nil, fmt.Errorf("unexpected command %s", request.Command)
		}
	}}
	code, _, stderr, _ := runWithCaller(t, caller, "correct horse battery staple\n", "tunnel", "unlock", "Docs", "--server", "Production", "--passphrase-stdin")
	if code != ExitOK || stderr != "" || secret != "correct horse battery staple" {
		t.Fatalf("code=%d secret=%q stderr=%q", code, secret, stderr)
	}

	code, _, stderr, _ = runWithCaller(t, caller, "secret", "tunnel", "unlock", "Docs", "--server", "Production")
	if code != ExitUsage || !strings.Contains(stderr, "interactive terminal") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestUnlockRejectsOversizedStdinBeforeSendingSecret(t *testing.T) {
	unlockCalls := 0
	caller := &fakeCaller{handle: func(request control.Request) (any, error) {
		if request.Command == "state" {
			return fixtureState(), nil
		}
		if request.Command == "session.unlock" {
			unlockCalls++
		}
		return nil, nil
	}}
	code, _, stderr, _ := runWithCaller(t, caller, strings.Repeat("x", 64*1024+1), "tunnel", "unlock", "Browser", "--server", "Production", "--passphrase-stdin")
	if code != ExitUsage || unlockCalls != 0 || !strings.Contains(stderr, "exceeds 64 KiB") {
		t.Fatalf("code=%d unlockCalls=%d stderr=%q", code, unlockCalls, stderr)
	}
}

func TestHistoryLimitIsAppliedAfterRetrieval(t *testing.T) {
	entries := []sessiondomain.SessionHistoryEntry{{ID: "one"}, {ID: "two"}, {ID: "three"}}
	caller := &fakeCaller{handle: func(request control.Request) (any, error) {
		if request.Command == "state" {
			return fixtureState(), nil
		}
		if request.Command == "session.history" {
			return entries, nil
		}
		return nil, fmt.Errorf("unexpected command %s", request.Command)
	}}
	code, stdout, stderr, _ := runWithCaller(t, caller, "", "--output", "json", "tunnel", "history", "Browser", "--server", "Production", "--limit", "2")
	if code != ExitOK || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var decoded []sessiondomain.SessionHistoryEntry
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil || len(decoded) != 2 || decoded[1].ID != "two" {
		t.Fatalf("decoded=%+v err=%v stdout=%q", decoded, err, stdout)
	}
}

func TestTunnelDeleteStopActiveOrdersOperations(t *testing.T) {
	caller := &fakeCaller{handle: func(request control.Request) (any, error) {
		switch request.Command {
		case "state":
			return fixtureState(), nil
		case "session.stop":
			return sessiondomain.RuntimeSession{ConfigurationID: request.ConfigurationID, Status: sessiondomain.StatusStopped}, nil
		case "configuration.delete":
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected command %s", request.Command)
		}
	}}
	code, _, stderr, _ := runWithCaller(t, caller, "", "tunnel", "delete", "Docs", "--server", "Production", "--stop-active", "--yes")
	if code != ExitOK || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	commands := make([]string, 0, len(caller.requests))
	for _, request := range caller.requests {
		commands = append(commands, request.Command)
	}
	if strings.Join(commands, ",") != "state,session.stop,configuration.delete" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestAppStatusReportsStoppedWithoutAutostartOrFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	autostart := true
	code := Run(context.Background(), []string{"--output", "json", "app", "status"}, Dependencies{
		Connect: func(_ context.Context, options ConnectOptions) (Caller, error) {
			autostart = options.Autostart
			return nil, errors.New("not running")
		},
		Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader(""), Version: "dev",
	})
	if code != ExitOK || autostart || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"running": false`) {
		t.Fatalf("code=%d autostart=%v stdout=%q stderr=%q", code, autostart, stdout.String(), stderr.String())
	}
}

func TestConnectorDeadlineUsesTimeoutExitCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"server", "list"}, Dependencies{
		Connect: func(context.Context, ConnectOptions) (Caller, error) {
			return nil, fmt.Errorf("wait for agent: %w", context.DeadlineExceeded)
		},
		Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader(""), Version: "dev",
	})
	if code != ExitTimeout || !strings.Contains(stderr.String(), "deadline exceeded") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestGlobalTimeoutAndOutputOptionsCanFollowCommand(t *testing.T) {
	options, remaining, err := parseGlobalOptions([]string{"server", "list", "--request-timeout=2s", "--output", "jsonl"})
	if err != nil {
		t.Fatal(err)
	}
	if options.RequestTimeout != 2*time.Second || options.Output != "jsonl" || strings.Join(remaining, " ") != "server list" {
		t.Fatalf("options=%+v remaining=%v", options, remaining)
	}
}

func TestCommandSurfaceRoutesThroughExpectedControlOperations(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantCommands string
	}{
		{name: "diagnostics", args: []string{"diagnostics"}, wantCommands: "state"},
		{name: "app show", args: []string{"app", "show"}, wantCommands: "app.show"},
		{name: "app hide", args: []string{"app", "hide"}, wantCommands: "app.hide"},
		{name: "app quit", args: []string{"app", "quit", "--yes"}, wantCommands: "state,app.quit"},
		{name: "server list", args: []string{"server", "list"}, wantCommands: "state"},
		{name: "server get", args: []string{"server", "get", "production"}, wantCommands: "state"},
		{name: "server update", args: []string{"server", "update", "Staging", "--port", "2200"}, wantCommands: "state,server.save"},
		{name: "server delete", args: []string{"server", "delete", "Staging", "--yes"}, wantCommands: "state,server.delete"},
		{name: "server start", args: []string{"server", "start", "Production"}, wantCommands: "state,session.start_server"},
		{name: "server stop", args: []string{"server", "stop", "Production"}, wantCommands: "state,session.stop_server"},
		{name: "tunnel list", args: []string{"tunnel", "list", "--server", "Production"}, wantCommands: "state"},
		{name: "tunnel get", args: []string{"tunnel", "get", "Browser", "--server", "Production"}, wantCommands: "state"},
		{name: "tunnel update", args: []string{"tunnel", "update", "Browser", "--server", "Production", "--notes", "proxy"}, wantCommands: "state,configuration.save"},
		{name: "tunnel stop", args: []string{"tunnel", "stop", "Docs", "--server", "Production"}, wantCommands: "state,session.stop"},
		{name: "tunnel restart", args: []string{"tunnel", "restart", "Docs", "--server", "Production"}, wantCommands: "state,session.retry"},
		{name: "browser list", args: []string{"browser", "list"}, wantCommands: "browser.list"},
		{name: "browser preview", args: []string{"browser", "preview", "--tunnel", "Browser", "--server", "Production", "--browser", "chrome"}, wantCommands: "state,browser.preview"},
		{name: "browser launch", args: []string{"browser", "launch", "--tunnel", "Browser", "--server", "Production", "--browser", "chrome"}, wantCommands: "state,browser.launch"},
		{name: "settings get", args: []string{"settings", "get"}, wantCommands: "state"},
		{name: "settings set", args: []string{"settings", "set", "--theme", "light"}, wantCommands: "state,preferences.save"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &fakeCaller{handle: func(request control.Request) (any, error) {
				switch request.Command {
				case "state":
					return fixtureState(), nil
				case "app.show", "app.hide", "app.quit", "server.delete", "configuration.delete", "browser.launch":
					return nil, nil
				case "server.save":
					return *request.Server, nil
				case "configuration.save":
					return *request.Configuration, nil
				case "session.start_server", "session.stop_server":
					return control.BulkResult{Sessions: []sessiondomain.RuntimeSession{}, Failures: []control.Failure{}}, nil
				case "session.start", "session.retry":
					return sessiondomain.RuntimeSession{ConfigurationID: request.ConfigurationID, Status: sessiondomain.StatusConnected, BoundPort: 3000}, nil
				case "session.stop":
					return sessiondomain.RuntimeSession{ConfigurationID: request.ConfigurationID, Status: sessiondomain.StatusStopped}, nil
				case "browser.list":
					return []any{}, nil
				case "browser.preview":
					return map[string]any{"browserId": "chrome", "browserName": "Chrome", "command": "chrome --proxy", "supported": true, "configurationId": request.ConfigurationID}, nil
				case "preferences.save":
					return *request.Preferences, nil
				default:
					return nil, fmt.Errorf("unexpected command %s", request.Command)
				}
			}}
			code, _, stderr, _ := runWithCaller(t, caller, "", test.args...)
			if code != ExitOK || stderr != "" {
				t.Fatalf("code=%d stderr=%q requests=%+v", code, stderr, caller.requests)
			}
			commands := make([]string, 0, len(caller.requests))
			for _, request := range caller.requests {
				commands = append(commands, request.Command)
			}
			if got := strings.Join(commands, ","); got != test.wantCommands {
				t.Fatalf("commands=%q, want %q", got, test.wantCommands)
			}
		})
	}
}

func TestParseRemoteSupportsHostAndBracketedIPv6(t *testing.T) {
	for _, test := range []struct {
		input string
		host  string
		port  int
	}{
		{input: "localhost:3000", host: "localhost", port: 3000},
		{input: "[::1]:443", host: "::1", port: 443},
	} {
		got, err := parseRemote(test.input)
		if err != nil || got.host != test.host || got.port != test.port {
			t.Fatalf("parseRemote(%q) = %+v, %v", test.input, got, err)
		}
	}
}
