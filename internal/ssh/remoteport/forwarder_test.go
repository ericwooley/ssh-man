package remoteport

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	serverdomain "ssh-man/internal/domain/server"
)

type echoRemoteClient struct{}

func (echoRemoteClient) Dial(network, address string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		_, _ = io.Copy(server, server)
	}()
	return client, nil
}

func (echoRemoteClient) Close() error {
	return nil
}

func TestForwarderOpensReusableLoopbackForward(t *testing.T) {
	forwarder := NewForwarderWithDialer(
		serverdomain.Server{ID: "server-1"},
		func(context.Context, serverdomain.Server, string) (RemoteClient, error) {
			return echoRemoteClient{}, nil
		},
	)
	t.Cleanup(func() {
		_ = forwarder.Close()
	})

	first, err := forwarder.Open(context.Background(), "", 3000, []string{"0.0.0.0", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := forwarder.Open(context.Background(), "", 3000, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.LocalPort < 1 || first.RemoteHost != "127.0.0.1" {
		t.Fatalf("forwards = %#v and %#v", first, second)
	}
	if !strings.HasPrefix(first.AccessHost, "ssh-man-") || !strings.HasSuffix(first.AccessHost, ".localhost") {
		t.Fatalf("access host = %q", first.AccessHost)
	}
	other, err := forwarder.Open(context.Background(), "", 8080, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if other.AccessHost == first.AccessHost {
		t.Fatalf("ports shared access host %q", first.AccessHost)
	}

	connection, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(first.LocalPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	request := "GET / HTTP/1.1\r\nHost: " + net.JoinHostPort(first.AccessHost, strconv.Itoa(first.LocalPort)) + "\r\n\r\n"
	if _, err := connection.Write([]byte(request)); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, len(request))
	if _, err := io.ReadFull(connection, reply); err != nil {
		t.Fatal(err)
	}
	if string(reply) != request {
		t.Fatalf("forward reply = %q", reply)
	}
}

func TestForwarderRecreatesForwardWhenRemoteHostChanges(t *testing.T) {
	forwarder := NewForwarderWithDialer(
		serverdomain.Server{ID: "server-1"},
		func(context.Context, serverdomain.Server, string) (RemoteClient, error) {
			return echoRemoteClient{}, nil
		},
	)
	t.Cleanup(func() {
		_ = forwarder.Close()
	})

	first, err := forwarder.Open(context.Background(), "", 3000, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := forwarder.Open(context.Background(), "", 3000, []string{"::1"})
	if err != nil {
		t.Fatal(err)
	}
	if second.RemoteHost != "::1" {
		t.Fatalf("remote host = %q, want ::1", second.RemoteHost)
	}
	if second.LocalPort == first.LocalPort || second.AccessHost == first.AccessHost {
		t.Fatalf("stale forward was reused: first = %#v, second = %#v", first, second)
	}
}

type failingRemoteClient struct {
	dialed     chan struct{}
	dialOnce   sync.Once
	closeCount atomic.Int32
}

func newFailingRemoteClient() *failingRemoteClient {
	return &failingRemoteClient{dialed: make(chan struct{})}
}

func (client *failingRemoteClient) Dial(string, string) (net.Conn, error) {
	client.dialOnce.Do(func() {
		close(client.dialed)
	})
	return nil, errors.New("SSH client is closed")
}

func (client *failingRemoteClient) Close() error {
	client.closeCount.Add(1)
	return nil
}

func TestForwarderRecreatesForwardsAfterPersistentClientFailure(t *testing.T) {
	failedClient := newFailingRemoteClient()
	healthyClient := echoRemoteClient{}
	var dialCount atomic.Int32
	forwarder := NewForwarderWithDialer(
		serverdomain.Server{ID: "server-1"},
		func(context.Context, serverdomain.Server, string) (RemoteClient, error) {
			if dialCount.Add(1) == 1 {
				return failedClient, nil
			}
			return healthyClient, nil
		},
	)
	t.Cleanup(func() {
		_ = forwarder.Close()
	})

	first, err := forwarder.Open(context.Background(), "", 3000, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(first.LocalPort)))
	if err != nil {
		t.Fatal(err)
	}
	request := "GET / HTTP/1.1\r\nHost: " + net.JoinHostPort(first.AccessHost, strconv.Itoa(first.LocalPort)) + "\r\n\r\n"
	if _, err := connection.Write([]byte(request)); err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()

	select {
	case <-failedClient.dialed:
	case <-time.After(time.Second):
		t.Fatal("persistent client did not receive the remote dial")
	}
	deadline := time.Now().Add(time.Second)
	for {
		forwarder.mu.Lock()
		cleared := forwarder.client == nil && len(forwarder.forwards) == 0
		forwarder.mu.Unlock()
		if cleared {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed persistent client kept stale forwards")
		}
		time.Sleep(time.Millisecond)
	}

	second, err := forwarder.Open(context.Background(), "", 3000, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatalf("stale forward was reused: %#v", second)
	}
	if dialCount.Load() != 2 {
		t.Fatalf("SSH dial count = %d, want 2", dialCount.Load())
	}
	if failedClient.closeCount.Load() != 1 {
		t.Fatalf("failed client close count = %d, want 1", failedClient.closeCount.Load())
	}
}

type countingRemoteClient struct {
	dialCount  atomic.Int32
	closeCount atomic.Int32
}

func (client *countingRemoteClient) Dial(string, string) (net.Conn, error) {
	client.dialCount.Add(1)
	local, remote := net.Pipe()
	go func() {
		defer remote.Close()
		_, _ = io.Copy(remote, remote)
	}()
	return local, nil
}

func (client *countingRemoteClient) Close() error {
	client.closeCount.Add(1)
	return nil
}

func TestForwarderRejectsRequestsWithoutItsAccessHost(t *testing.T) {
	remote := &countingRemoteClient{}
	forwarder := NewForwarderWithDialer(
		serverdomain.Server{ID: "server-1"},
		func(context.Context, serverdomain.Server, string) (RemoteClient, error) {
			return remote, nil
		},
	)
	t.Cleanup(func() {
		_ = forwarder.Close()
	})
	forward, err := forwarder.Open(context.Background(), "", 3000, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}

	connection, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(forward.LocalPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	if _, err := connection.Write([]byte("GET / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	var reply [1]byte
	if _, err := connection.Read(reply[:]); err == nil {
		t.Fatal("expected the unauthorized connection to close")
	}
	if remote.dialCount.Load() != 0 {
		t.Fatalf("remote dial count = %d", remote.dialCount.Load())
	}
}

func TestForwardAccessAcceptsMatchingTLSServerName(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	t.Cleanup(func() {
		_ = clientConnection.Close()
		_ = serverConnection.Close()
	})
	const accessHost = "ssh-man-test.localhost"
	go func() {
		client := tls.Client(clientConnection, &tls.Config{
			ServerName:         accessHost,
			InsecureSkipVerify: true, // The test reads only the client hello.
		})
		_ = client.Handshake()
	}()

	preamble, err := authorizeForwardConnection(serverConnection, accessHost)
	if err != nil {
		t.Fatal(err)
	}
	if len(preamble) < 6 || preamble[0] != 0x16 {
		t.Fatalf("TLS preamble = %x", preamble)
	}
}

func TestForwardAccessRejectsAnotherTLSServerName(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	t.Cleanup(func() {
		_ = clientConnection.Close()
		_ = serverConnection.Close()
	})
	go func() {
		client := tls.Client(clientConnection, &tls.Config{
			ServerName:         "another-service.localhost",
			InsecureSkipVerify: true, // The test reads only the client hello.
		})
		_ = client.Handshake()
	}()

	if _, err := authorizeForwardConnection(serverConnection, "ssh-man-test.localhost"); err == nil {
		t.Fatal("expected a TLS access-host error")
	}
}

func TestForwardAccessAcceptsFragmentedTLSClientHello(t *testing.T) {
	record := captureTLSClientHello(t, "ssh-man-test.localhost")
	payload := record[5:]
	if len(payload) < 20 {
		t.Fatalf("TLS client hello payload is too short: %d", len(payload))
	}
	first := tlsHandshakeRecord(record[1:3], payload[:20])
	second := tlsHandshakeRecord(record[1:3], payload[20:])
	want := append(append([]byte(nil), first...), second...)

	clientConnection, serverConnection := net.Pipe()
	t.Cleanup(func() {
		_ = clientConnection.Close()
		_ = serverConnection.Close()
	})
	go func() {
		_, _ = clientConnection.Write(want)
	}()

	preamble, err := authorizeForwardConnection(serverConnection, "ssh-man-test.localhost")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(preamble, want) {
		t.Fatalf("TLS preamble length = %d, want %d", len(preamble), len(want))
	}
}

func captureTLSClientHello(t *testing.T, serverName string) []byte {
	t.Helper()
	clientConnection, serverConnection := net.Pipe()
	handshakeDone := make(chan error, 1)
	go func() {
		client := tls.Client(clientConnection, &tls.Config{
			ServerName:         serverName,
			InsecureSkipVerify: true, // The test captures only the client hello.
		})
		handshakeDone <- client.Handshake()
	}()

	_ = serverConnection.SetReadDeadline(time.Now().Add(time.Second))
	header := make([]byte, 5)
	if _, err := io.ReadFull(serverConnection, header); err != nil {
		t.Fatal(err)
	}
	length := int(header[3])<<8 | int(header[4])
	payload := make([]byte, length)
	if _, err := io.ReadFull(serverConnection, payload); err != nil {
		t.Fatal(err)
	}
	_ = serverConnection.Close()
	_ = clientConnection.Close()
	<-handshakeDone
	return append(header, payload...)
}

func tlsHandshakeRecord(version, payload []byte) []byte {
	record := []byte{0x16, version[0], version[1], byte(len(payload) >> 8), byte(len(payload))}
	return append(record, payload...)
}

type blockingRemoteClient struct {
	started   chan struct{}
	closed    chan struct{}
	returned  chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func newBlockingRemoteClient() *blockingRemoteClient {
	return &blockingRemoteClient{
		started:  make(chan struct{}),
		closed:   make(chan struct{}),
		returned: make(chan struct{}),
	}
}

func (client *blockingRemoteClient) Dial(string, string) (net.Conn, error) {
	client.startOnce.Do(func() {
		close(client.started)
	})
	<-client.closed
	close(client.returned)
	return nil, net.ErrClosed
}

func (client *blockingRemoteClient) Close() error {
	client.closeOnce.Do(func() {
		close(client.closed)
	})
	return nil
}

func TestForwarderCanceledDirectDialClosesDedicatedClient(t *testing.T) {
	remote := newBlockingRemoteClient()
	forwarder := NewForwarderWithDialer(
		serverdomain.Server{ID: "server-1"},
		func(context.Context, serverdomain.Server, string) (RemoteClient, error) {
			return remote, nil
		},
	)
	t.Cleanup(func() {
		_ = forwarder.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := forwarder.DialRemote(ctx, "", 3000, []string{"127.0.0.1"})
		result <- err
	}()
	<-remote.started
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("dial error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled direct dial did not return")
	}
	select {
	case <-remote.closed:
	case <-time.After(time.Second):
		t.Fatal("canceled direct dial did not close its SSH client")
	}
	select {
	case <-remote.returned:
	case <-time.After(time.Second):
		t.Fatal("SSH channel dial did not stop after client close")
	}
}

func TestForwarderDialsRemoteWithoutOpeningListener(t *testing.T) {
	remote := &countingRemoteClient{}
	forwarder := NewForwarderWithDialer(
		serverdomain.Server{ID: "server-1"},
		func(context.Context, serverdomain.Server, string) (RemoteClient, error) {
			return remote, nil
		},
	)
	t.Cleanup(func() {
		_ = forwarder.Close()
	})

	connection, err := forwarder.DialRemote(context.Background(), "", 3000, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if remote.dialCount.Load() != 1 {
		t.Fatalf("remote dial count = %d", remote.dialCount.Load())
	}
	if len(forwarder.forwards) != 0 {
		t.Fatalf("persistent forwards = %d", len(forwarder.forwards))
	}
	if forwarder.client != nil {
		t.Fatal("direct dial reused the persistent forward client")
	}
	if remote.closeCount.Load() != 1 {
		t.Fatalf("direct client close count = %d", remote.closeCount.Load())
	}
}

func TestRemoteHostForAddressesPrefersReachableLoopback(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
		want      string
	}{
		{name: "ipv4 wildcard", addresses: []string{"0.0.0.0"}, want: "127.0.0.1"},
		{name: "ipv6 wildcard", addresses: []string{"::"}, want: "::1"},
		{name: "ipv6 loopback", addresses: []string{"::1"}, want: "::1"},
		{name: "host address", addresses: []string{"10.0.0.4"}, want: "10.0.0.4"},
		{name: "star", addresses: []string{"*"}, want: "127.0.0.1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := remoteHostForAddresses(test.addresses); got != test.want {
				t.Fatalf("remoteHostForAddresses(%#v) = %q, want %q", test.addresses, got, test.want)
			}
		})
	}
}
