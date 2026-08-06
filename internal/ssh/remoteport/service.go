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

const discoveryCommand = `if [ -r /proc/meminfo ]; then
  awk '/^MemTotal:/ { total=$2*1024 } /^MemAvailable:/ { available=$2*1024 } END { printf "SSH_MAN_METRIC\tmemoryTotalBytes\t%.0f\nSSH_MAN_METRIC\tmemoryAvailableBytes\t%.0f\n", total, available }' /proc/meminfo
  awk '{ printf "SSH_MAN_METRIC\tuptimeSeconds\t%.0f\n", $1 }' /proc/uptime
  awk '{ printf "SSH_MAN_METRIC\tloadOne\t%s\n", $1 }' /proc/loadavg
  cpu_count=$(getconf _NPROCESSORS_ONLN 2>/dev/null || printf '0')
  printf 'SSH_MAN_METRIC\tcpuCount\t%s\n' "$cpu_count"
elif command -v sysctl >/dev/null 2>&1; then
  total=$(sysctl -n hw.memsize 2>/dev/null || printf '0')
  printf 'SSH_MAN_METRIC\tmemoryTotalBytes\t%s\n' "$total"
  if command -v vm_stat >/dev/null 2>&1; then
    vm_stat | awk '
      /page size of/ { page=$8; gsub(/\./, "", page) }
      /^Pages free:/ || /^Pages inactive:/ || /^Pages speculative:/ || /^Pages purgeable:/ {
        value=$NF; gsub(/\./, "", value); available+=value
      }
      END { printf "SSH_MAN_METRIC\tmemoryAvailableBytes\t%.0f\n", available*page }
    '
  fi
  boot=$(sysctl -n kern.boottime 2>/dev/null | sed -E 's/.*sec = ([0-9]+).*/\1/')
  now=$(date +%s)
  if [ -n "$boot" ]; then printf 'SSH_MAN_METRIC\tuptimeSeconds\t%s\n' "$((now-boot))"; fi
  sysctl -n vm.loadavg 2>/dev/null | awk '{ gsub(/[{}]/, ""); printf "SSH_MAN_METRIC\tloadOne\t%s\n", $1 }'
  cpu_count=$(sysctl -n hw.logicalcpu 2>/dev/null || printf '0')
  printf 'SSH_MAN_METRIC\tcpuCount\t%s\n' "$cpu_count"
fi

if command -v ss >/dev/null 2>&1; then
  ss -H -lntp 2>/dev/null | head -n 4096 | while IFS= read -r line; do
    address=$(printf '%s\n' "$line" | awk '{print $4}')
    process=$(printf '%s\n' "$line" | sed -n 's/.*users:(("\([^"]*\)".*/\1/p')
    pid=$(printf '%s\n' "$line" | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p')
    printf 'SSH_MAN_PORT\t%s\t%s\t%s\n' "$address" "$process" "$pid"
  done
elif command -v lsof >/dev/null 2>&1; then
  lsof -nP -iTCP -sTCP:LISTEN -Fpcn 2>/dev/null | awk '
    /^p/ { pid=substr($0, 2) }
    /^c/ { name=substr($0, 2) }
    /^n/ { printf "SSH_MAN_PORT\t%s\t%s\t%s\n", substr($0, 2), name, pid }
  ' | head -n 4096
elif command -v netstat >/dev/null 2>&1; then
  netstat -an 2>/dev/null | awk '$1 ~ /^tcp/ && $NF == "LISTEN" { printf "SSH_MAN_PORT\t%s\t\t\n", $4 }' | head -n 4096
else
  printf 'Neither ss nor netstat is available.\n' >&2
  exit 127
fi`

type ListeningPort struct {
	Port            int      `json:"port"`
	Addresses       []string `json:"addresses"`
	SuggestedScheme string   `json:"suggestedScheme"`
}

type HostMetrics struct {
	MemoryTotalBytes     uint64  `json:"memoryTotalBytes"`
	MemoryAvailableBytes uint64  `json:"memoryAvailableBytes"`
	UptimeSeconds        uint64  `json:"uptimeSeconds"`
	LoadOne              float64 `json:"loadOne"`
	CPUCount             int     `json:"cpuCount"`
}

type ListeningApplication struct {
	Name  string `json:"name"`
	PID   int    `json:"pid"`
	Ports []int  `json:"ports"`
}

type DashboardSnapshot struct {
	Metrics      HostMetrics            `json:"metrics"`
	Ports        []ListeningPort        `json:"ports"`
	Applications []ListeningApplication `json:"applications"`
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

func (service *Service) Discover(ctx context.Context, passphrase string) (DashboardSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultDiscoveryTimeout)
	defer cancel()
	output, err := service.run(ctx, service.server, passphrase, discoveryCommand)
	if err != nil {
		return DashboardSnapshot{}, fmt.Errorf("load host dashboard: %w", err)
	}
	return parseDashboardSnapshot(output), nil
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

func parseDashboardSnapshot(output []byte) DashboardSnapshot {
	snapshot := DashboardSnapshot{
		Ports:        []ListeningPort{},
		Applications: []ListeningApplication{},
	}
	var portOutput strings.Builder
	applicationPorts := map[string]map[int]struct{}{}

	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		switch fields[0] {
		case "SSH_MAN_METRIC":
			parseMetric(&snapshot.Metrics, fields[1], fields[2])
		case "SSH_MAN_PORT":
			address, port, ok := splitAddressPort(strings.TrimSpace(fields[1]))
			if !ok {
				continue
			}
			fmt.Fprintf(&portOutput, "%s:%d\n", formatAddress(address), port)
			if len(fields) < 4 {
				continue
			}
			name := strings.TrimSpace(fields[2])
			pid, err := strconv.Atoi(strings.TrimSpace(fields[3]))
			if name == "" || err != nil || pid < 1 {
				continue
			}
			key := fmt.Sprintf("%s\t%d", name, pid)
			if applicationPorts[key] == nil {
				applicationPorts[key] = map[int]struct{}{}
			}
			applicationPorts[key][port] = struct{}{}
		}
	}

	snapshot.Ports = parseListeningPorts([]byte(portOutput.String()))
	for key, portSet := range applicationPorts {
		fields := strings.Split(key, "\t")
		pid, _ := strconv.Atoi(fields[1])
		ports := make([]int, 0, len(portSet))
		for port := range portSet {
			ports = append(ports, port)
		}
		sort.Ints(ports)
		snapshot.Applications = append(snapshot.Applications, ListeningApplication{
			Name:  fields[0],
			PID:   pid,
			Ports: ports,
		})
	}
	sort.Slice(snapshot.Applications, func(first, second int) bool {
		if snapshot.Applications[first].Name != snapshot.Applications[second].Name {
			return snapshot.Applications[first].Name < snapshot.Applications[second].Name
		}
		return snapshot.Applications[first].PID < snapshot.Applications[second].PID
	})
	return snapshot
}

func parseMetric(metrics *HostMetrics, name, value string) {
	switch name {
	case "memoryTotalBytes":
		metrics.MemoryTotalBytes, _ = strconv.ParseUint(value, 10, 64)
	case "memoryAvailableBytes":
		metrics.MemoryAvailableBytes, _ = strconv.ParseUint(value, 10, 64)
	case "uptimeSeconds":
		metrics.UptimeSeconds, _ = strconv.ParseUint(value, 10, 64)
	case "loadOne":
		metrics.LoadOne, _ = strconv.ParseFloat(value, 64)
	case "cpuCount":
		metrics.CPUCount, _ = strconv.Atoi(value)
	}
}

func formatAddress(address string) string {
	if strings.Contains(address, ":") {
		return "[" + address + "]"
	}
	return address
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

type sshClientDependencies struct {
	authFactory authMethodFactory
	dial        resolvedSSHClientDialer
}

var defaultSSHClientDependencies = sshClientDependencies{
	authFactory: sshconnection.AuthMethod,
	dial:        sshconnection.DialSSH,
}

func dialSSHClient(ctx context.Context, server serverdomain.Server, passphrase string) (*ssh.Client, error) {
	return dialSSHClientWithDependencies(
		ctx,
		server,
		passphrase,
		defaultSSHClientDependencies.authFactory,
		defaultSSHClientDependencies.dial,
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
