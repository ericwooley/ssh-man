package remoteport

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	serverdomain "ssh-man/internal/domain/server"
	sshconnection "ssh-man/internal/ssh/connection"

	"golang.org/x/crypto/ssh"
)

const (
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"
)

const discoveryCommand = `if command -v ss >/dev/null 2>&1; then ss -H -lnt | awk '{print $4}' | head -n 4096; elif command -v netstat >/dev/null 2>&1; then netstat -an 2>/dev/null | awk '$1 ~ /^tcp/ && $NF == "LISTEN" {print $4}' | head -n 4096; else printf 'Neither ss nor netstat is available.\n' >&2; exit 127; fi`

type ListeningPort struct {
	Port            int      `json:"port"`
	Addresses       []string `json:"addresses"`
	SuggestedScheme string   `json:"suggestedScheme"`
}

type commandRunner func(context.Context, serverdomain.Server, string, string) ([]byte, error)

type Service struct {
	server serverdomain.Server
	run    commandRunner
}

func NewService(server serverdomain.Server) *Service {
	return NewServiceWithRunner(server, runSSHCommand)
}

func NewServiceWithRunner(server serverdomain.Server, runner commandRunner) *Service {
	return &Service{server: server, run: runner}
}

func (service *Service) Discover(ctx context.Context, passphrase string) ([]ListeningPort, error) {
	output, err := service.run(ctx, service.server, passphrase, discoveryCommand)
	if err != nil {
		return nil, fmt.Errorf("find listening ports: %w", err)
	}
	return parseListeningPorts(output), nil
}

func parseListeningPorts(output []byte) []ListeningPort {
	addressesByPort := map[int]map[string]struct{}{}
	for _, line := range strings.Split(string(output), "\n") {
		address, port, ok := splitAddressPort(strings.TrimSpace(line))
		if !ok {
			continue
		}
		if addressesByPort[port] == nil {
			addressesByPort[port] = map[string]struct{}{}
		}
		addressesByPort[port][address] = struct{}{}
	}

	ports := make([]int, 0, len(addressesByPort))
	for port := range addressesByPort {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	items := make([]ListeningPort, 0, len(ports))
	for _, port := range ports {
		addresses := make([]string, 0, len(addressesByPort[port]))
		for address := range addressesByPort[port] {
			addresses = append(addresses, address)
		}
		sort.Strings(addresses)
		scheme := SchemeHTTP
		switch port {
		case 443, 8443, 9443:
			scheme = SchemeHTTPS
		}
		items = append(items, ListeningPort{
			Port:            port,
			Addresses:       addresses,
			SuggestedScheme: scheme,
		})
	}
	return items
}

func splitAddressPort(value string) (string, int, bool) {
	if value == "" {
		return "", 0, false
	}

	address := ""
	portText := ""
	if strings.HasPrefix(value, "[") {
		if host, port, err := net.SplitHostPort(value); err == nil {
			address = host
			portText = port
		}
	}
	if portText == "" {
		if index := strings.LastIndex(value, ":"); index >= 0 {
			address = strings.Trim(value[:index], "[]")
			portText = value[index+1:]
		} else if index := strings.LastIndex(value, "."); index >= 0 {
			address = value[:index]
			portText = value[index+1:]
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || address == "" {
		return "", 0, false
	}
	return address, port, true
}

func runSSHCommand(ctx context.Context, server serverdomain.Server, passphrase, command string) ([]byte, error) {
	authMethod, err := sshconnection.AuthMethod(server, passphrase)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := sshconnection.KnownHostsCallback()
	if err != nil {
		return nil, fmt.Errorf("configure SSH host key verification: %w", err)
	}

	rawConnection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(server.Host, strconv.Itoa(server.Port)))
	if err != nil {
		return nil, fmt.Errorf("connect to SSH server: %w", err)
	}

	connection, channels, requests, err := ssh.NewClientConn(rawConnection, net.JoinHostPort(server.Host, strconv.Itoa(server.Port)), &ssh.ClientConfig{
		User:            server.Username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
	})
	if err != nil {
		_ = rawConnection.Close()
		return nil, fmt.Errorf("start SSH connection: %w", err)
	}
	client := ssh.NewClient(connection, channels, requests)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("start SSH command: %w", err)
	}
	defer session.Close()

	var output bytes.Buffer
	session.Stdout = &output
	session.Stderr = &output
	if err := session.Start(command); err != nil {
		return nil, fmt.Errorf("start remote port command: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()
	select {
	case <-ctx.Done():
		_ = session.Close()
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("run remote port command: %w: %s", err, strings.TrimSpace(output.String()))
		}
		return output.Bytes(), nil
	}
}
