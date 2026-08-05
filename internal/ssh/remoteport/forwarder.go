package remoteport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"

	serverdomain "ssh-man/internal/domain/server"
)

type RemoteClient interface {
	Dial(network, address string) (net.Conn, error)
	Close() error
}

type clientDialer func(context.Context, serverdomain.Server, string) (RemoteClient, error)

type Forward struct {
	RemotePort int    `json:"remotePort"`
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
}

type runningForward struct {
	result   Forward
	listener net.Listener
}

type Forwarder struct {
	mu       sync.Mutex
	server   serverdomain.Server
	dial     clientDialer
	client   RemoteClient
	forwards map[int]runningForward
}

func NewForwarder(server serverdomain.Server) *Forwarder {
	return NewForwarderWithDialer(server, func(ctx context.Context, server serverdomain.Server, passphrase string) (RemoteClient, error) {
		return dialSSHClient(ctx, server, passphrase)
	})
}

func NewForwarderWithDialer(server serverdomain.Server, dial clientDialer) *Forwarder {
	return &Forwarder{
		server:   server,
		dial:     dial,
		forwards: map[int]runningForward{},
	}
}

func (forwarder *Forwarder) Open(ctx context.Context, passphrase string, remotePort int, addresses []string) (Forward, error) {
	if remotePort < 1 || remotePort > 65535 {
		return Forward{}, fmt.Errorf("remote port must be between 1 and 65535")
	}

	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	if existing, ok := forwarder.forwards[remotePort]; ok {
		return existing.result, nil
	}
	if forwarder.client == nil {
		client, err := forwarder.dial(ctx, forwarder.server, passphrase)
		if err != nil {
			return Forward{}, fmt.Errorf("connect before opening port: %w", err)
		}
		forwarder.client = client
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Forward{}, fmt.Errorf("listen for local port access: %w", err)
	}
	tcpAddress, ok := listener.Addr().(*net.TCPAddr)
	if !ok || tcpAddress.Port < 1 {
		_ = listener.Close()
		return Forward{}, fmt.Errorf("local port address is unavailable")
	}
	result := Forward{
		RemotePort: remotePort,
		LocalPort:  tcpAddress.Port,
		RemoteHost: remoteHostForAddresses(addresses),
	}
	forwarder.forwards[remotePort] = runningForward{result: result, listener: listener}
	go forwarder.accept(listener, result)
	return result, nil
}

func (forwarder *Forwarder) accept(listener net.Listener, forward Forward) {
	for {
		localConnection, err := listener.Accept()
		if err != nil {
			return
		}
		go forwarder.proxy(localConnection, forward)
	}
}

func (forwarder *Forwarder) proxy(localConnection net.Conn, forward Forward) {
	forwarder.mu.Lock()
	client := forwarder.client
	forwarder.mu.Unlock()
	if client == nil {
		_ = localConnection.Close()
		return
	}
	remoteConnection, err := client.Dial("tcp", net.JoinHostPort(forward.RemoteHost, strconv.Itoa(forward.RemotePort)))
	if err != nil {
		_ = localConnection.Close()
		return
	}

	done := make(chan struct{}, 2)
	copyConnection := func(destination io.Writer, source io.Reader) {
		_, _ = io.Copy(destination, source)
		done <- struct{}{}
	}
	go copyConnection(remoteConnection, localConnection)
	go copyConnection(localConnection, remoteConnection)
	<-done
	_ = localConnection.Close()
	_ = remoteConnection.Close()
	<-done
}

func (forwarder *Forwarder) Close() error {
	if forwarder == nil {
		return nil
	}
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()

	var closeErrors []error
	ports := make([]int, 0, len(forwarder.forwards))
	for port := range forwarder.forwards {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	for _, port := range ports {
		if err := forwarder.forwards[port].listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			closeErrors = append(closeErrors, fmt.Errorf("close local port %d: %w", port, err))
		}
		delete(forwarder.forwards, port)
	}
	if forwarder.client != nil {
		closeErrors = append(closeErrors, forwarder.client.Close())
		forwarder.client = nil
	}
	return errors.Join(closeErrors...)
}

func remoteHostForAddresses(addresses []string) string {
	for _, address := range addresses {
		address = strings.TrimSpace(strings.Trim(address, "[]"))
		switch address {
		case "*", "0.0.0.0":
			return "127.0.0.1"
		case "127.0.0.1":
			return address
		}
	}
	for _, address := range addresses {
		address = strings.TrimSpace(strings.Trim(address, "[]"))
		if zoneIndex := strings.LastIndex(address, "%"); zoneIndex >= 0 {
			address = address[:zoneIndex]
		}
		switch address {
		case "::", "::1":
			return "::1"
		case "":
			continue
		default:
			if net.ParseIP(address) != nil {
				return address
			}
		}
	}
	return "127.0.0.1"
}
