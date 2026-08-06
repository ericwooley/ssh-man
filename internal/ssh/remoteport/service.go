package remoteport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	serverdomain "ssh-man/internal/domain/server"
	sshconnection "ssh-man/internal/ssh/connection"

	"golang.org/x/crypto/ssh"
)

const (
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"

	defaultDiscoveryTimeout = 15 * time.Second
	maxDiscoveryOutputBytes = 1 << 20
)

var errDiscoveryOutputLimit = errors.New("remote port output exceeded the local size limit")

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
	ctx, cancel := context.WithTimeout(ctx, defaultDiscoveryTimeout)
	defer cancel()
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

	if index := strings.LastIndex(value, "."); index >= 0 {
		dotAddress := value[:index]
		dotPort := value[index+1:]
		isBSDAddress := strings.Contains(dotAddress, ":") || strings.Count(value, ".") >= 4 || strings.HasPrefix(value, "*.")
		if port, err := strconv.Atoi(dotPort); isBSDAddress && err == nil && port >= 1 && port <= 65535 && dotAddress != "" {
			return strings.Trim(dotAddress, "[]"), port, true
		}
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

type commandSession interface {
	Start(string) error
	Wait() error
	Close() error
}

type sessionFactory func(io.Writer, io.Writer) (commandSession, error)

type boundedCommandOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	exceeded chan struct{}
	once     sync.Once
}

func newBoundedCommandOutput(limit int) *boundedCommandOutput {
	return &boundedCommandOutput{
		limit:    limit,
		exceeded: make(chan struct{}),
	}
}

func (output *boundedCommandOutput) Write(data []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()

	remaining := output.limit - output.buffer.Len()
	if remaining > 0 {
		writeCount := len(data)
		if writeCount > remaining {
			writeCount = remaining
		}
		_, _ = output.buffer.Write(data[:writeCount])
	}
	if len(data) > remaining {
		output.once.Do(func() {
			close(output.exceeded)
		})
	}
	return len(data), nil
}

func (output *boundedCommandOutput) Bytes() []byte {
	output.mu.Lock()
	defer output.mu.Unlock()
	return append([]byte(nil), output.buffer.Bytes()...)
}

func (output *boundedCommandOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.buffer.String()
}

func (output *boundedCommandOutput) Exceeded() bool {
	select {
	case <-output.exceeded:
		return true
	default:
		return false
	}
}

func runSSHCommand(ctx context.Context, server serverdomain.Server, passphrase, command string) ([]byte, error) {
	client, err := dialSSHClient(ctx, server, passphrase)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return runSSHClientCommand(ctx, client, func(stdout, stderr io.Writer) (commandSession, error) {
		session, err := client.NewSession()
		if err != nil {
			return nil, err
		}
		session.Stdout = stdout
		session.Stderr = stderr
		return session, nil
	}, command)
}

type authMethodFactory func(serverdomain.Server, string) (ssh.AuthMethod, error)

type resolvedSSHClientDialer func(context.Context, serverdomain.Server, ssh.AuthMethod) (*ssh.Client, error)

func dialSSHClient(ctx context.Context, server serverdomain.Server, passphrase string) (*ssh.Client, error) {
	return dialSSHClientWithDependencies(
		ctx,
		server,
		passphrase,
		sshconnection.AuthMethod,
		sshconnection.DialSSH,
	)
}

func dialSSHClientWithDependencies(
	ctx context.Context,
	server serverdomain.Server,
	passphrase string,
	authFactory authMethodFactory,
	dial resolvedSSHClientDialer,
) (*ssh.Client, error) {
	authMethod, err := authFactory(server, passphrase)
	if err != nil {
		return nil, err
	}
	return dial(ctx, server, authMethod)
}

func runSSHClientCommand(ctx context.Context, client io.Closer, newSession sessionFactory, command string) ([]byte, error) {
	operationDone := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			_ = client.Close()
		case <-operationDone:
		}
	}()
	defer func() {
		close(operationDone)
		<-watcherDone
	}()

	output := newBoundedCommandOutput(maxDiscoveryOutputBytes)
	session, err := newSession(output, output)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("start SSH command: %w", err)
	}
	defer session.Close()

	if err := session.Start(command); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("start remote port command: %w", err)
	}
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- session.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitDone:
	case <-output.exceeded:
		_ = session.Close()
		_ = client.Close()
		return nil, errDiscoveryOutputLimit
	}
	if output.Exceeded() {
		return nil, errDiscoveryOutputLimit
	}
	if waitErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("run remote port command: %w: %s", waitErr, strings.TrimSpace(output.String()))
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return output.Bytes(), nil
}
