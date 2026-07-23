package cli

import (
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"

	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	preferencesdomain "ssh-man/internal/domain/preferences"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
	"ssh-man/internal/platform/browser"
)

func (r *commandRuntime) dispatch(args []string) error {
	switch args[0] {
	case "status":
		return r.tunnelList(args[1:])
	case "diagnostics":
		return r.diagnostics(args[1:])
	case "app":
		return r.app(args[1:])
	case "server":
		return r.server(args[1:])
	case "tunnel":
		return r.tunnel(args[1:])
	case "browser":
		return r.browser(args[1:])
	case "settings":
		return r.settings(args[1:])
	default:
		return usageErrorf("unknown command %q; run ssh-man --help", args[0])
	}
}

func (r *commandRuntime) diagnostics(args []string) error {
	if len(args) != 0 {
		return usageErrorf("usage: ssh-man diagnostics")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, state.Diagnostics, table{
		Header: []string{"APP_DATA", "DATABASE"},
		Rows:   [][]string{{state.Diagnostics.AppDataPath, state.Diagnostics.DatabasePath}},
	})
}

func (r *commandRuntime) app(args []string) error {
	if len(args) == 0 {
		return usageErrorf("usage: ssh-man app status|show|hide|quit")
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return usageErrorf("usage: ssh-man app status")
		}
		return r.appStatus()
	case "show":
		if len(args) != 1 {
			return usageErrorf("usage: ssh-man app show")
		}
		if err := r.call(true, control.Request{Command: "app.show"}, nil); err != nil {
			return err
		}
		return r.writeMessage("shown", "SSH Man is visible.")
	case "hide":
		if len(args) != 1 {
			return usageErrorf("usage: ssh-man app hide")
		}
		if err := r.call(false, control.Request{Command: "app.hide"}, nil); err != nil {
			return err
		}
		return r.writeMessage("hidden", "SSH Man is hidden; tunnels remain active.")
	case "quit":
		parsed, err := parseOptions(args[1:], map[string]optionKind{"--yes": boolOption})
		if err != nil {
			return err
		}
		if err := requirePositionals(parsed, 0, "ssh-man app quit [--yes]"); err != nil {
			return err
		}
		state, stateErr := r.stateWithoutAutostart()
		if stateErr != nil {
			return stateErr
		}
		active := 0
		for _, session := range state.Sessions {
			if isActive(session.Status) {
				active++
			}
		}
		if active > 0 && !parsed.bool("--yes") {
			return usageErrorf("%d tunnel(s) are active; use --yes to stop them and quit", active)
		}
		if err := r.call(false, control.Request{Command: "app.quit"}, nil); err != nil {
			return err
		}
		return r.writeMessage("quitting", "SSH Man is quitting; active tunnels will stop.")
	default:
		return usageErrorf("unknown app command %q", args[0])
	}
}

func (r *commandRuntime) appStatus() error {
	if r.dependencies.Connect == nil {
		return r.writeRunning(false)
	}
	client, err := r.dependencies.Connect(r.ctx, ConnectOptions{
		Autostart:      false,
		ConnectTimeout: r.globals.ConnectTimeout,
		RequestTimeout: r.globals.RequestTimeout,
	})
	if err != nil {
		return r.writeRunning(false)
	}
	r.client = client
	if err := r.call(false, control.Request{Command: "ping"}, nil); err != nil {
		return r.writeRunning(false)
	}
	return r.writeRunning(true)
}

func (r *commandRuntime) writeRunning(running bool) error {
	status := "stopped"
	if running {
		status = "running"
	}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, map[string]any{"running": running}, table{Header: []string{"STATUS"}, Rows: [][]string{{status}}})
}

func (r *commandRuntime) stateWithoutAutostart() (control.State, error) {
	var state control.State
	if err := r.call(false, control.Request{Command: "state"}, &state); err != nil {
		return control.State{}, err
	}
	return state, nil
}

func (r *commandRuntime) writeMessage(status string, message string) error {
	return writeOutput(r.dependencies.Stdout, r.globals.Output, map[string]string{"status": status, "message": message}, table{Header: []string{"STATUS", "MESSAGE"}, Rows: [][]string{{status, message}}})
}

func (r *commandRuntime) server(args []string) error {
	if len(args) == 0 {
		return usageErrorf("usage: ssh-man server list|get|add|update|delete|start|stop")
	}
	switch args[0] {
	case "list":
		return r.serverList(args[1:])
	case "get":
		return r.serverGet(args[1:])
	case "add":
		return r.serverAdd(args[1:])
	case "update":
		return r.serverUpdate(args[1:])
	case "delete", "remove", "rm":
		return r.serverDelete(args[1:])
	case "start":
		return r.serverSession(args[1:], "session.start_server")
	case "stop":
		return r.serverSession(args[1:], "session.stop_server")
	default:
		return usageErrorf("unknown server command %q", args[0])
	}
}

func (r *commandRuntime) serverList(args []string) error {
	if len(args) != 0 {
		return usageErrorf("usage: ssh-man server list")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	records := sortedServerRecords(state)
	return writeOutput(r.dependencies.Stdout, r.globals.Output, serverJSON(records), serverTable(state, records))
}

func (r *commandRuntime) serverGet(args []string) error {
	if len(args) != 1 {
		return usageErrorf("usage: ssh-man server get SERVER")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveServer(state, args[0])
	if err != nil {
		return err
	}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, record, serverTable(state, []control.ServerRecord{record}))
}

var serverAddOptionSpecs = map[string]optionKind{
	"--host": stringOption, "--port": stringOption, "--user": stringOption,
	"--auth": stringOption, "--key": stringOption, "--socks-port": stringOption,
}

var serverUpdateOptionSpecs = map[string]optionKind{
	"--host": stringOption, "--port": stringOption, "--user": stringOption,
	"--auth": stringOption, "--key": stringOption, "--name": stringOption,
	"--socks-port": stringOption,
}

func (r *commandRuntime) serverAdd(args []string) error {
	parsed, err := parseOptions(args, serverAddOptionSpecs)
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man server add NAME --host HOST"); err != nil {
		return err
	}
	host, err := requireOption(parsed, "--host")
	if err != nil {
		return err
	}
	port := 22
	if parsed.has("--port") {
		port, err = parsed.int("--port")
		if err != nil {
			return err
		}
	}
	authMode, key, err := parseAuth(parsed.string("--auth"), parsed.string("--key"), serverdomain.AuthModeAgent, "")
	if err != nil {
		return err
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	username := parsed.string("--user")
	if strings.TrimSpace(username) == "" {
		username = state.CurrentUsername
	}
	socksPort, err := parseServerSOCKSPort(parsed.string("--socks-port"))
	if err != nil {
		return err
	}
	input := serverdomain.Server{Name: parsed.positionals[0], Host: host, Port: port, SocksPort: socksPort, Username: username, AuthMode: authMode, KeyReference: key}
	if err := input.Validate(); err != nil {
		return usageErrorf("%v", err)
	}
	var saved serverdomain.Server
	if err := r.call(true, control.Request{Command: "server.save", Server: &input}, &saved); err != nil {
		return err
	}
	record := control.ServerRecord{Server: saved, Configurations: []configdomain.ConnectionConfiguration{}}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, record, serverTable(control.State{}, []control.ServerRecord{record}))
}

func (r *commandRuntime) serverUpdate(args []string) error {
	parsed, err := parseOptions(args, serverUpdateOptionSpecs)
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man server update SERVER [options]"); err != nil {
		return err
	}
	if len(parsed.present) == 0 {
		return usageErrorf("server update requires at least one option")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveServer(state, parsed.positionals[0])
	if err != nil {
		return err
	}
	input := record.Server
	if parsed.has("--name") {
		input.Name = parsed.string("--name")
	}
	if parsed.has("--host") {
		input.Host = parsed.string("--host")
	}
	if parsed.has("--port") {
		input.Port, err = parsed.int("--port")
		if err != nil {
			return err
		}
	}
	if parsed.has("--user") {
		input.Username = parsed.string("--user")
	}
	if parsed.has("--socks-port") {
		input.SocksPort, err = parseServerSOCKSPort(parsed.string("--socks-port"))
		if err != nil {
			return err
		}
	}
	input.AuthMode, input.KeyReference, err = parseAuth(parsed.string("--auth"), parsed.string("--key"), input.AuthMode, input.KeyReference)
	if err != nil {
		return err
	}
	if err := input.Validate(); err != nil {
		return usageErrorf("%v", err)
	}
	var saved serverdomain.Server
	if err := r.call(true, control.Request{Command: "server.save", Server: &input}, &saved); err != nil {
		return err
	}
	record.Server = saved
	return writeOutput(r.dependencies.Stdout, r.globals.Output, record, serverTable(state, []control.ServerRecord{record}))
}

func parseServerSOCKSPort(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" || strings.EqualFold(value, "auto") {
		return 0, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, usageErrorf("--socks-port must be auto or a port between 1 and 65535")
	}
	return port, nil
}

func parseAuth(auth string, key string, fallback serverdomain.AuthMode, fallbackKey string) (serverdomain.AuthMode, string, error) {
	mode := fallback
	if key != "" && strings.EqualFold(auth, "agent") {
		return "", "", usageErrorf("--key cannot be used with --auth agent")
	}
	if auth == "" && key != "" {
		mode = serverdomain.AuthModePrivateKey
	}
	if auth != "" {
		switch strings.ToLower(auth) {
		case "agent":
			mode = serverdomain.AuthModeAgent
		case "key", "private_key", "private-key":
			mode = serverdomain.AuthModePrivateKey
		default:
			return "", "", usageErrorf("--auth must be agent or key")
		}
	}
	keyReference := fallbackKey
	if key != "" {
		keyReference = key
	}
	if mode == serverdomain.AuthModeAgent {
		keyReference = ""
	}
	if mode == serverdomain.AuthModePrivateKey && strings.TrimSpace(keyReference) == "" {
		return "", "", usageErrorf("--key is required when --auth is key")
	}
	return mode, keyReference, nil
}

func (r *commandRuntime) serverDelete(args []string) error {
	parsed, err := parseOptions(args, map[string]optionKind{"--yes": boolOption, "--stop-active": boolOption})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man server delete SERVER --yes [--stop-active]"); err != nil {
		return err
	}
	if !parsed.bool("--yes") {
		return usageErrorf("server delete requires --yes")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveServer(state, parsed.positionals[0])
	if err != nil {
		return err
	}
	if parsed.bool("--stop-active") {
		var result control.BulkResult
		if err := r.call(true, control.Request{Command: "session.stop_server", ServerID: record.Server.ID}, &result); err != nil {
			return err
		}
		if len(result.Failures) > 0 {
			return r.bulkFailure(result)
		}
	}
	if err := r.call(true, control.Request{Command: "server.delete", ServerID: record.Server.ID}, nil); err != nil {
		return err
	}
	return r.writeMessage("deleted", fmt.Sprintf("Server %s was deleted.", record.Server.Name))
}

func (r *commandRuntime) serverSession(args []string, command string) error {
	if len(args) != 1 {
		return usageErrorf("usage: ssh-man server start|stop SERVER")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveServer(state, args[0])
	if err != nil {
		return err
	}
	var result control.BulkResult
	if err := r.call(true, control.Request{Command: command, ServerID: record.Server.ID}, &result); err != nil {
		return err
	}
	if err := writeOutput(r.dependencies.Stdout, r.globals.Output, result, sessionTable(result.Sessions)); err != nil {
		return err
	}
	if len(result.Failures) > 0 {
		return r.bulkFailure(result)
	}
	for _, state := range result.Sessions {
		if err := sessionStatusError(state); err != nil {
			return err
		}
	}
	return nil
}

func (r *commandRuntime) bulkFailure(result control.BulkResult) error {
	messages := make([]string, 0, len(result.Failures))
	for _, failure := range result.Failures {
		messages = append(messages, failure.Message)
	}
	return &exitError{code: ExitOperation, message: strings.Join(messages, "; ")}
}

func (r *commandRuntime) tunnel(args []string) error {
	if len(args) == 0 {
		return usageErrorf("usage: ssh-man tunnel list|get|add|update|delete|start|stop|restart|unlock|history")
	}
	switch args[0] {
	case "list":
		return r.tunnelList(args[1:])
	case "get":
		return r.tunnelGet(args[1:])
	case "add":
		return r.tunnelAdd(args[1:])
	case "update":
		return r.tunnelUpdate(args[1:])
	case "delete", "remove", "rm":
		return r.tunnelDelete(args[1:])
	case "start":
		return r.tunnelSession(args[1:], "session.start")
	case "stop":
		return r.tunnelSession(args[1:], "session.stop")
	case "restart", "retry":
		return r.tunnelSession(args[1:], "session.retry")
	case "unlock":
		return r.tunnelUnlock(args[1:])
	case "history":
		return r.tunnelHistory(args[1:])
	default:
		return usageErrorf("unknown tunnel command %q", args[0])
	}
}

func (r *commandRuntime) tunnelList(args []string) error {
	parsed, err := parseOptions(args, map[string]optionKind{
		"--server": stringOption, "--type": stringOption, "--status": stringOption, "--active": boolOption,
	})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 0, "ssh-man tunnel list [options]"); err != nil {
		return err
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	records := tunnelRecords(state)
	serverID := ""
	if parsed.has("--server") {
		server, err := resolveServer(state, parsed.string("--server"))
		if err != nil {
			return err
		}
		serverID = server.Server.ID
	}
	typeFilter := normalizeConnectionType(parsed.string("--type"))
	if parsed.has("--type") && typeFilter == "" {
		return usageErrorf("--type must be local or socks")
	}
	statuses := map[string]bool{}
	for _, status := range strings.Split(parsed.string("--status"), ",") {
		if strings.TrimSpace(status) != "" {
			statuses[strings.TrimSpace(status)] = true
		}
	}
	filtered := make([]tunnelRecord, 0, len(records))
	for _, record := range records {
		if serverID != "" && record.Server.ID != serverID {
			continue
		}
		if typeFilter != "" && record.Configuration.ConnectionType != typeFilter {
			continue
		}
		if len(statuses) > 0 && !statuses[string(record.Session.Status)] {
			continue
		}
		if parsed.bool("--active") && !isActive(record.Session.Status) {
			continue
		}
		filtered = append(filtered, record)
	}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, tunnelJSON(filtered), tunnelTable(filtered))
}

func (r *commandRuntime) tunnelGet(args []string) error {
	parsed, err := parseOptions(args, map[string]optionKind{"--server": stringOption})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man tunnel get TUNNEL [--server SERVER]"); err != nil {
		return err
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveTunnel(state, parsed.positionals[0], parsed.string("--server"))
	if err != nil {
		return err
	}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, singleTunnelJSON(record), tunnelTable([]tunnelRecord{record}))
}

var tunnelAddOptionSpecs = map[string]optionKind{
	"--server": stringOption, "--listen": stringOption, "--remote": stringOption,
	"--reconnect": boolOption, "--start-on-launch": boolOption, "--notes": stringOption,
}

var tunnelUpdateOptionSpecs = map[string]optionKind{
	"--server": stringOption, "--listen": stringOption, "--remote": stringOption,
	"--reconnect": boolOption, "--notes": stringOption, "--label": stringOption,
	"--type": stringOption, "--clear-notes": boolOption, "--start-on-launch": boolOption,
}

func (r *commandRuntime) tunnelAdd(args []string) error {
	if len(args) == 0 {
		return usageErrorf("usage: ssh-man tunnel add local|socks LABEL [options]")
	}
	tunnelType := normalizeConnectionType(args[0])
	if tunnelType == "" {
		return usageErrorf("tunnel type must be local or socks")
	}
	parsed, err := parseOptions(args[1:], tunnelAddOptionSpecs)
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man tunnel add local|socks LABEL [options]"); err != nil {
		return err
	}
	serverSelector, err := requireOption(parsed, "--server")
	if err != nil {
		return err
	}
	input := configdomain.ConnectionConfiguration{
		Label: parsed.positionals[0], ConnectionType: tunnelType,
		AutoReconnectEnabled: true, Notes: parsed.string("--notes"),
	}
	if parsed.has("--reconnect") {
		input.AutoReconnectEnabled = parsed.bool("--reconnect")
	}
	if parsed.has("--start-on-launch") {
		input.StartOnLaunch = parsed.bool("--start-on-launch")
	}
	if err := applyTunnelRouting(&input, parsed, true); err != nil {
		return err
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	server, err := resolveServer(state, serverSelector)
	if err != nil {
		return err
	}
	input.ServerID = server.Server.ID
	return r.saveTunnel(input, server.Server)
}

func (r *commandRuntime) tunnelUpdate(args []string) error {
	parsed, err := parseOptions(args, tunnelUpdateOptionSpecs)
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man tunnel update TUNNEL [options]"); err != nil {
		return err
	}
	if len(parsed.present) == 0 || (len(parsed.present) == 1 && parsed.has("--server")) {
		return usageErrorf("tunnel update requires at least one change option")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveTunnel(state, parsed.positionals[0], parsed.string("--server"))
	if err != nil {
		return err
	}
	input := record.Configuration
	if parsed.has("--label") {
		input.Label = parsed.string("--label")
	}
	if parsed.has("--type") {
		input.ConnectionType = normalizeConnectionType(parsed.string("--type"))
		if input.ConnectionType == "" {
			return usageErrorf("--type must be local or socks")
		}
	}
	if parsed.has("--reconnect") {
		input.AutoReconnectEnabled = parsed.bool("--reconnect")
	}
	if parsed.has("--start-on-launch") {
		input.StartOnLaunch = parsed.bool("--start-on-launch")
	}
	if parsed.has("--notes") {
		input.Notes = parsed.string("--notes")
	}
	if parsed.bool("--clear-notes") {
		input.Notes = ""
	}
	if err := applyTunnelRouting(&input, parsed, false); err != nil {
		return err
	}
	return r.saveTunnel(input, record.Server)
}

func normalizeConnectionType(value string) configdomain.ConnectionType {
	switch strings.ToLower(value) {
	case "local", "local_forward", "local-forward":
		return configdomain.ConnectionTypeLocalForward
	case "socks", "socks_proxy", "socks-proxy":
		return configdomain.ConnectionTypeSOCKSProxy
	default:
		return ""
	}
}

func applyTunnelRouting(input *configdomain.ConnectionConfiguration, parsed parsedOptions, creating bool) error {
	listen := parsed.string("--listen")
	switch input.ConnectionType {
	case configdomain.ConnectionTypeLocalForward:
		if creating || parsed.has("--listen") {
			if listen == "" || strings.EqualFold(listen, "auto") {
				return usageErrorf("local tunnels require --listen PORT")
			}
			port, err := strconv.Atoi(listen)
			if err != nil {
				return usageErrorf("--listen must be a port")
			}
			input.LocalPort = port
		}
		if creating || parsed.has("--remote") {
			remote, err := parseRemote(parsed.string("--remote"))
			if err != nil {
				return err
			}
			input.RemoteHost = remote.host
			input.RemotePort = remote.port
		}
		input.SocksPort = 0
	case configdomain.ConnectionTypeSOCKSProxy:
		if parsed.has("--remote") {
			return usageErrorf("--remote is only valid for local tunnels")
		}
		if creating && listen == "" {
			listen = "auto"
		}
		if creating || parsed.has("--listen") {
			if strings.EqualFold(listen, "auto") || listen == "0" {
				input.SocksPort = 0
			} else {
				port, err := strconv.Atoi(listen)
				if err != nil {
					return usageErrorf("--listen must be auto or a port")
				}
				input.SocksPort = port
			}
		}
		input.LocalPort = 0
		input.RemoteHost = ""
		input.RemotePort = 0
	}
	return nil
}

type remoteAddress struct {
	host string
	port int
}

func parseRemote(value string) (remoteAddress, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return remoteAddress{}, usageErrorf("--remote HOST:PORT is required")
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		index := strings.LastIndex(value, ":")
		if index <= 0 || strings.Contains(value[:index], ":") {
			return remoteAddress{}, usageErrorf("--remote must be HOST:PORT; bracket IPv6 addresses")
		}
		host, portText = value[:index], value[index+1:]
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || strings.TrimSpace(host) == "" {
		return remoteAddress{}, usageErrorf("--remote must be HOST:PORT with a port from 1 to 65535")
	}
	return remoteAddress{host: host, port: port}, nil
}

func (r *commandRuntime) saveTunnel(input configdomain.ConnectionConfiguration, server serverdomain.Server) error {
	if err := input.Validate(); err != nil {
		return usageErrorf("%v", err)
	}
	var saved configdomain.ConnectionConfiguration
	if err := r.call(true, control.Request{Command: "configuration.save", Configuration: &input}, &saved); err != nil {
		return err
	}
	record := tunnelRecord{Server: server, Configuration: saved, Session: sessiondomain.RuntimeSession{ConfigurationID: saved.ID, Status: sessiondomain.StatusStopped}}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, singleTunnelJSON(record), tunnelTable([]tunnelRecord{record}))
}

func (r *commandRuntime) tunnelDelete(args []string) error {
	parsed, err := parseOptions(args, map[string]optionKind{"--server": stringOption, "--yes": boolOption, "--stop-active": boolOption})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man tunnel delete TUNNEL --yes [--server SERVER] [--stop-active]"); err != nil {
		return err
	}
	if !parsed.bool("--yes") {
		return usageErrorf("tunnel delete requires --yes")
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveTunnel(state, parsed.positionals[0], parsed.string("--server"))
	if err != nil {
		return err
	}
	if parsed.bool("--stop-active") && isActive(record.Session.Status) {
		var stopped sessiondomain.RuntimeSession
		if err := r.call(true, control.Request{Command: "session.stop", ConfigurationID: record.Configuration.ID}, &stopped); err != nil {
			return err
		}
	}
	if err := r.call(true, control.Request{Command: "configuration.delete", ConfigurationID: record.Configuration.ID}, nil); err != nil {
		return err
	}
	return r.writeMessage("deleted", fmt.Sprintf("Tunnel %s was deleted.", record.Configuration.Label))
}

func (r *commandRuntime) tunnelSession(args []string, command string) error {
	parsed, err := parseOptions(args, map[string]optionKind{"--server": stringOption})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man tunnel start|stop|restart TUNNEL [--server SERVER]"); err != nil {
		return err
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveTunnel(state, parsed.positionals[0], parsed.string("--server"))
	if err != nil {
		return err
	}
	var runtime sessiondomain.RuntimeSession
	if err := r.call(true, control.Request{Command: command, ConfigurationID: record.Configuration.ID}, &runtime); err != nil {
		return err
	}
	if err := writeOutput(r.dependencies.Stdout, r.globals.Output, runtime, sessionTable([]sessiondomain.RuntimeSession{runtime})); err != nil {
		return err
	}
	return sessionStatusError(runtime)
}

func sessionStatusError(runtime sessiondomain.RuntimeSession) error {
	switch runtime.Status {
	case sessiondomain.StatusFailed:
		message := runtime.StatusDetail
		if message == "" {
			message = "tunnel failed"
		}
		return &exitError{code: ExitTunnelFailed, message: message}
	case sessiondomain.StatusNeedsAttention:
		message := runtime.StatusDetail
		if message == "" {
			message = "tunnel needs attention"
		}
		return &exitError{code: ExitAttention, message: message}
	default:
		return nil
	}
}

func (r *commandRuntime) tunnelUnlock(args []string) error {
	parsed, err := parseOptions(args, map[string]optionKind{"--server": stringOption, "--passphrase-stdin": boolOption})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man tunnel unlock TUNNEL [--server SERVER] [--passphrase-stdin]"); err != nil {
		return err
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveTunnel(state, parsed.positionals[0], parsed.string("--server"))
	if err != nil {
		return err
	}
	secret, err := r.readPassphrase(parsed.bool("--passphrase-stdin"))
	if err != nil {
		return err
	}
	var runtime sessiondomain.RuntimeSession
	if err := r.call(true, control.Request{Command: "session.unlock", ConfigurationID: record.Configuration.ID, Secret: secret}, &runtime); err != nil {
		return err
	}
	if err := writeOutput(r.dependencies.Stdout, r.globals.Output, runtime, sessionTable([]sessiondomain.RuntimeSession{runtime})); err != nil {
		return err
	}
	return sessionStatusError(runtime)
}

func (r *commandRuntime) readPassphrase(fromStdin bool) (string, error) {
	if fromStdin {
		const maxPassphraseBytes = 64 * 1024
		value, err := io.ReadAll(io.LimitReader(r.dependencies.Stdin, maxPassphraseBytes+1))
		if err != nil {
			return "", fmt.Errorf("read passphrase: %w", err)
		}
		if len(value) > maxPassphraseBytes {
			return "", usageErrorf("passphrase from stdin exceeds 64 KiB")
		}
		secret := strings.TrimRight(string(value), "\r\n")
		if secret == "" {
			return "", usageErrorf("passphrase from stdin is empty")
		}
		return secret, nil
	}
	if !r.dependencies.IsTerminal() || r.dependencies.ReadPassword == nil {
		return "", usageErrorf("unlock requires an interactive terminal or --passphrase-stdin")
	}
	_, _ = fmt.Fprint(r.dependencies.Stderr, "SSH key passphrase: ")
	value, err := r.dependencies.ReadPassword()
	_, _ = fmt.Fprintln(r.dependencies.Stderr)
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	if len(value) == 0 {
		return "", usageErrorf("passphrase is empty")
	}
	return string(value), nil
}

func (r *commandRuntime) tunnelHistory(args []string) error {
	parsed, err := parseOptions(args, map[string]optionKind{"--server": stringOption, "--limit": stringOption})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 1, "ssh-man tunnel history TUNNEL [--server SERVER] [--limit N]"); err != nil {
		return err
	}
	limit := -1
	if parsed.has("--limit") {
		limit, err = parsed.int("--limit")
		if err != nil || limit < 0 {
			return usageErrorf("--limit must be zero or greater")
		}
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveTunnel(state, parsed.positionals[0], parsed.string("--server"))
	if err != nil {
		return err
	}
	var entries []sessiondomain.SessionHistoryEntry
	if err := r.call(true, control.Request{Command: "session.history", ConfigurationID: record.Configuration.ID}, &entries); err != nil {
		return err
	}
	if entries == nil {
		entries = []sessiondomain.SessionHistoryEntry{}
	}
	if limit >= 0 {
		if limit < len(entries) {
			entries = entries[:limit]
		}
	}
	return writeOutput(r.dependencies.Stdout, r.globals.Output, entries, historyTable(entries))
}

func (r *commandRuntime) browser(args []string) error {
	if len(args) == 0 {
		return usageErrorf("usage: ssh-man browser list|preview|launch")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usageErrorf("usage: ssh-man browser list")
		}
		var items []browser.BrowserOption
		if err := r.call(true, control.Request{Command: "browser.list"}, &items); err != nil {
			return err
		}
		if items == nil {
			items = []browser.BrowserOption{}
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].DisplayName != items[j].DisplayName {
				return items[i].DisplayName < items[j].DisplayName
			}
			return items[i].ID < items[j].ID
		})
		return writeOutput(r.dependencies.Stdout, r.globals.Output, items, browserTable(items))
	case "preview", "launch":
		return r.browserAction(args[0], args[1:])
	default:
		return usageErrorf("unknown browser command %q", args[0])
	}
}

func (r *commandRuntime) browserAction(action string, args []string) error {
	parsed, err := parseOptions(args, map[string]optionKind{"--tunnel": stringOption, "--server": stringOption, "--browser": stringOption})
	if err != nil {
		return err
	}
	if err := requirePositionals(parsed, 0, "ssh-man browser preview|launch --tunnel TUNNEL --browser BROWSER"); err != nil {
		return err
	}
	tunnelSelector, err := requireOption(parsed, "--tunnel")
	if err != nil {
		return err
	}
	browserID, err := requireOption(parsed, "--browser")
	if err != nil {
		return err
	}
	state, err := r.state()
	if err != nil {
		return err
	}
	record, err := resolveTunnel(state, tunnelSelector, parsed.string("--server"))
	if err != nil {
		return err
	}
	if action == "preview" {
		var preview browser.LaunchPreview
		if err := r.call(true, control.Request{Command: "browser.preview", ConfigurationID: record.Configuration.ID, BrowserID: browserID}, &preview); err != nil {
			return err
		}
		return writeOutput(r.dependencies.Stdout, r.globals.Output, preview, table{Header: []string{"BROWSER", "SUPPORTED", "COMMAND"}, Rows: [][]string{{preview.BrowserName, strconv.FormatBool(preview.Supported), preview.Command}}})
	}
	if err := r.call(true, control.Request{Command: "browser.launch", ConfigurationID: record.Configuration.ID, BrowserID: browserID}, nil); err != nil {
		return err
	}
	return r.writeMessage("launched", fmt.Sprintf("Browser %s launched through %s.", browserID, record.Configuration.Label))
}

func (r *commandRuntime) settings(args []string) error {
	if len(args) == 0 {
		return usageErrorf("usage: ssh-man settings get|set")
	}
	switch args[0] {
	case "get":
		if len(args) != 1 {
			return usageErrorf("usage: ssh-man settings get")
		}
		state, err := r.state()
		if err != nil {
			return err
		}
		return writeOutput(r.dependencies.Stdout, r.globals.Output, state.Preferences, preferencesTable(state.Preferences))
	case "set":
		parsed, err := parseOptions(args[1:], map[string]optionKind{"--theme": stringOption})
		if err != nil {
			return err
		}
		if err := requirePositionals(parsed, 0, "ssh-man settings set --theme dark|light"); err != nil {
			return err
		}
		theme, err := requireOption(parsed, "--theme")
		if err != nil {
			return err
		}
		if theme != "dark" && theme != "light" {
			return usageErrorf("--theme must be dark or light")
		}
		state, err := r.state()
		if err != nil {
			return err
		}
		preferences := state.Preferences
		preferences.Theme = preferencesdomain.Theme(theme)
		var saved preferencesdomain.UserPreference
		if err := r.call(true, control.Request{Command: "preferences.save", Preferences: &preferences}, &saved); err != nil {
			return err
		}
		return writeOutput(r.dependencies.Stdout, r.globals.Output, saved, preferencesTable(saved))
	default:
		return usageErrorf("unknown settings command %q", args[0])
	}
}

func preferencesTable(preferences preferencesdomain.UserPreference) table {
	return table{Header: []string{"THEME", "LAST_SERVER"}, Rows: [][]string{{string(preferences.Theme), preferences.LastSelectedServerID}}}
}
