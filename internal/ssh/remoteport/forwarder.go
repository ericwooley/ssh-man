package remoteport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	serverdomain "ssh-man/internal/domain/server"

	"golang.org/x/crypto/ssh"
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
	AccessHost string `json:"accessHost"`
}

type runningForward struct {
	result   Forward
	listener net.Listener
}

type clientOwnedConnection struct {
	net.Conn
	client    RemoteClient
	closeOnce sync.Once
	closeErr  error
}

func (connection *clientOwnedConnection) Close() error {
	connection.closeOnce.Do(func() {
		connection.closeErr = errors.Join(connection.Conn.Close(), connection.client.Close())
	})
	return connection.closeErr
}

type Forwarder struct {
	mu               sync.Mutex
	server           serverdomain.Server
	dial             clientDialer
	client           RemoteClient
	clientGeneration uint64
	forwards         map[int]runningForward
}

const (
	maxForwardPreambleBytes = 64 << 10
	forwardAccessTimeout    = 5 * time.Second
)

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
	remoteHost := remoteHostForAddresses(addresses)
	if existing, ok := forwarder.forwards[remotePort]; ok {
		if existing.result.RemoteHost == remoteHost {
			return existing.result, nil
		}
		if err := existing.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return Forward{}, fmt.Errorf("replace local port access: %w", err)
		}
		delete(forwarder.forwards, remotePort)
	}
	if forwarder.client == nil {
		client, err := forwarder.dial(ctx, forwarder.server, passphrase)
		if err != nil {
			return Forward{}, fmt.Errorf("connect before opening port: %w", err)
		}
		forwarder.client = client
		forwarder.clientGeneration++
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
		RemoteHost: remoteHost,
	}
	result.AccessHost, err = newForwardAccessHost()
	if err != nil {
		_ = listener.Close()
		return Forward{}, err
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
	preamble, err := authorizeForwardConnection(localConnection, forward.AccessHost)
	if err != nil {
		_ = localConnection.Close()
		return
	}

	forwarder.mu.Lock()
	client := forwarder.client
	clientGeneration := forwarder.clientGeneration
	forwarder.mu.Unlock()
	if client == nil {
		_ = localConnection.Close()
		return
	}
	remoteConnection, err := client.Dial("tcp", net.JoinHostPort(forward.RemoteHost, strconv.Itoa(forward.RemotePort)))
	if err != nil {
		_ = localConnection.Close()
		forwarder.handlePersistentDialFailure(clientGeneration, forward, err)
		return
	}
	if _, err := remoteConnection.Write(preamble); err != nil {
		_ = localConnection.Close()
		_ = remoteConnection.Close()
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

func (forwarder *Forwarder) handlePersistentDialFailure(clientGeneration uint64, forward Forward, err error) {
	var channelError *ssh.OpenChannelError
	if errors.As(err, &channelError) {
		forwarder.removePersistentForward(clientGeneration, forward)
		return
	}
	forwarder.invalidatePersistentClient(clientGeneration)
}

func (forwarder *Forwarder) removePersistentForward(clientGeneration uint64, forward Forward) {
	forwarder.mu.Lock()
	if forwarder.client == nil || forwarder.clientGeneration != clientGeneration {
		forwarder.mu.Unlock()
		return
	}
	running, ok := forwarder.forwards[forward.RemotePort]
	if !ok || running.result != forward {
		forwarder.mu.Unlock()
		return
	}
	delete(forwarder.forwards, forward.RemotePort)
	forwarder.mu.Unlock()

	_ = running.listener.Close()
}

func (forwarder *Forwarder) invalidatePersistentClient(clientGeneration uint64) {
	forwarder.mu.Lock()
	if forwarder.client == nil || forwarder.clientGeneration != clientGeneration {
		forwarder.mu.Unlock()
		return
	}
	client := forwarder.client
	forwarder.client = nil
	forwards := forwarder.forwards
	forwarder.forwards = map[int]runningForward{}
	forwarder.mu.Unlock()

	for _, running := range forwards {
		_ = running.listener.Close()
	}
	_ = client.Close()
}

func (forwarder *Forwarder) DialRemote(
	ctx context.Context,
	passphrase string,
	remotePort int,
	addresses []string,
) (net.Conn, error) {
	if remotePort < 1 || remotePort > 65535 {
		return nil, fmt.Errorf("remote port must be between 1 and 65535")
	}

	client, err := forwarder.dial(ctx, forwarder.server, passphrase)
	if err != nil {
		return nil, fmt.Errorf("connect before accessing port: %w", err)
	}

	type dialResult struct {
		connection net.Conn
		err        error
	}
	result := make(chan dialResult, 1)
	go func() {
		connection, err := client.Dial("tcp", net.JoinHostPort(
			remoteHostForAddresses(addresses),
			strconv.Itoa(remotePort),
		))
		result <- dialResult{connection: connection, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = client.Close()
		completed := <-result
		if completed.connection != nil {
			_ = completed.connection.Close()
		}
		return nil, ctx.Err()
	case completed := <-result:
		if ctx.Err() != nil {
			if completed.connection != nil {
				_ = completed.connection.Close()
			}
			_ = client.Close()
			return nil, ctx.Err()
		}
		if completed.err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("access remote port %d: %w", remotePort, completed.err)
		}
		return &clientOwnedConnection{Conn: completed.connection, client: client}, nil
	}
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

func newForwardAccessHost() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("create local forward access host: %w", err)
	}
	return "ssh-man-" + hex.EncodeToString(token[:]) + ".localhost", nil
}

func authorizeForwardConnection(connection net.Conn, accessHost string) ([]byte, error) {
	if err := connection.SetReadDeadline(time.Now().Add(forwardAccessTimeout)); err != nil {
		return nil, err
	}
	defer connection.SetReadDeadline(time.Time{})

	prefix := make([]byte, 5)
	if _, err := io.ReadFull(connection, prefix); err != nil {
		return nil, err
	}
	if prefix[0] == 0x16 {
		preamble, clientHello, err := readTLSClientHello(connection, prefix)
		if err != nil {
			return nil, err
		}
		serverName, err := tlsClientHelloServerName(clientHello)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(serverName, accessHost) {
			return nil, fmt.Errorf("TLS client hello did not match the local access host")
		}
		return preamble, nil
	}

	preamble := append([]byte(nil), prefix...)
	for !bytes.Contains(preamble, []byte("\r\n\r\n")) {
		if len(preamble) >= maxForwardPreambleBytes {
			return nil, fmt.Errorf("HTTP request exceeded the access preamble limit")
		}
		next := make([]byte, 1024)
		count, err := connection.Read(next)
		if count > 0 {
			preamble = append(preamble, next[:count]...)
		}
		if err != nil {
			return nil, err
		}
	}
	request, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(preamble)))
	if err != nil {
		return nil, fmt.Errorf("read local HTTP request: %w", err)
	}
	if !strings.EqualFold(requestHostName(request.Host), accessHost) {
		return nil, fmt.Errorf("HTTP request did not match the local access host")
	}
	return preamble, nil
}

func readTLSClientHello(connection net.Conn, firstHeader []byte) ([]byte, []byte, error) {
	header := append([]byte(nil), firstHeader...)
	var preamble []byte
	var handshake []byte

	for {
		if len(header) != 5 || header[0] != 0x16 {
			return nil, nil, fmt.Errorf("connection did not contain a TLS handshake record")
		}
		recordLength := int(header[3])<<8 | int(header[4])
		if recordLength < 1 || len(preamble)+len(header)+recordLength > maxForwardPreambleBytes {
			return nil, nil, fmt.Errorf("TLS client hello exceeded the access preamble limit")
		}
		payload := make([]byte, recordLength)
		if _, err := io.ReadFull(connection, payload); err != nil {
			return nil, nil, err
		}
		preamble = append(preamble, header...)
		preamble = append(preamble, payload...)
		handshake = append(handshake, payload...)

		if len(handshake) >= 4 {
			if handshake[0] != 0x01 {
				return nil, nil, fmt.Errorf("connection did not contain a TLS client hello")
			}
			helloLength := int(handshake[1])<<16 | int(handshake[2])<<8 | int(handshake[3])
			if helloLength+4 > maxForwardPreambleBytes {
				return nil, nil, fmt.Errorf("TLS client hello exceeded the access preamble limit")
			}
			if len(handshake) >= helloLength+4 {
				return preamble, handshake[:helloLength+4], nil
			}
		}

		header = make([]byte, 5)
		if _, err := io.ReadFull(connection, header); err != nil {
			return nil, nil, err
		}
	}
}

func requestHostName(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return strings.Trim(value, "[]")
}

func tlsClientHelloServerName(clientHello []byte) (string, error) {
	if len(clientHello) < 4 || clientHello[0] != 0x01 {
		return "", fmt.Errorf("connection did not contain a TLS client hello")
	}
	helloLength := int(clientHello[1])<<16 | int(clientHello[2])<<8 | int(clientHello[3])
	if helloLength+4 > len(clientHello) {
		return "", fmt.Errorf("TLS client hello message was incomplete")
	}

	index := 4 + 2 + 32
	if index >= len(clientHello) {
		return "", fmt.Errorf("TLS client hello omitted the session identifier")
	}
	index += 1 + int(clientHello[index])
	if index+2 > len(clientHello) {
		return "", fmt.Errorf("TLS client hello omitted cipher suites")
	}
	cipherLength := int(clientHello[index])<<8 | int(clientHello[index+1])
	index += 2 + cipherLength
	if index >= len(clientHello) {
		return "", fmt.Errorf("TLS client hello omitted compression methods")
	}
	index += 1 + int(clientHello[index])
	if index+2 > len(clientHello) {
		return "", fmt.Errorf("TLS client hello omitted extensions")
	}
	extensionsLength := int(clientHello[index])<<8 | int(clientHello[index+1])
	index += 2
	extensionsEnd := index + extensionsLength
	if extensionsEnd > len(clientHello) {
		return "", fmt.Errorf("TLS client hello extensions were incomplete")
	}

	for index+4 <= extensionsEnd {
		extensionType := int(clientHello[index])<<8 | int(clientHello[index+1])
		extensionLength := int(clientHello[index+2])<<8 | int(clientHello[index+3])
		index += 4
		if index+extensionLength > extensionsEnd {
			return "", fmt.Errorf("TLS client hello extension was incomplete")
		}
		if extensionType == 0 {
			return serverNameFromExtension(clientHello[index : index+extensionLength])
		}
		index += extensionLength
	}
	return "", fmt.Errorf("TLS client hello omitted a server name")
}

func serverNameFromExtension(extension []byte) (string, error) {
	if len(extension) < 2 {
		return "", fmt.Errorf("TLS server name extension was incomplete")
	}
	listLength := int(extension[0])<<8 | int(extension[1])
	if listLength+2 > len(extension) {
		return "", fmt.Errorf("TLS server name list was incomplete")
	}
	index := 2
	end := 2 + listLength
	for index+3 <= end {
		nameType := extension[index]
		nameLength := int(extension[index+1])<<8 | int(extension[index+2])
		index += 3
		if index+nameLength > end {
			return "", fmt.Errorf("TLS server name was incomplete")
		}
		if nameType == 0 {
			return string(extension[index : index+nameLength]), nil
		}
		index += nameLength
	}
	return "", fmt.Errorf("TLS server name extension omitted a host name")
}
