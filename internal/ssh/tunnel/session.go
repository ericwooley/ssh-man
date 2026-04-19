package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"

	"golang.org/x/crypto/ssh"
)

type Session struct {
	server         serverdomain.Server
	config         configdomain.ConnectionConfiguration
	passphrase     string
	onDisconnect   func(error)
	client         *ssh.Client
	listener       net.Listener
	stopCh         chan struct{}
	stopped        atomic.Bool
	stopOnce       sync.Once
	disconnectOnce sync.Once
}

const (
	keepaliveInterval = 15 * time.Second
	keepaliveTimeout  = 5 * time.Second
	probeTimeout      = 5 * time.Second
	probeReadWindow   = 750 * time.Millisecond
)

func NewSession(server serverdomain.Server, config configdomain.ConnectionConfiguration, passphrase string, onDisconnect func(error)) *Session {
	return &Session{server: server, config: config, passphrase: passphrase, onDisconnect: onDisconnect, stopCh: make(chan struct{})}
}

func (s *Session) Start() error {
	authMethod, err := s.authMethod()
	if err != nil {
		return err
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", s.server.Host, s.server.Port), &ssh.ClientConfig{
		User:            s.server.Username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("connect to ssh server: %w", err)
	}
	s.client = client

	bindAddr := fmt.Sprintf("127.0.0.1:%d", s.config.BoundPort())
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("bind local port %s: %w", bindAddr, err)
	}
	s.listener = listener

	go s.watchConnection()
	go s.serve()
	return nil
}

func (s *Session) Stop() error {
	var stopErr error
	s.stopOnce.Do(func() {
		s.stopped.Store(true)
		close(s.stopCh)
		if s.listener != nil {
			_ = s.listener.Close()
		}
		if s.client != nil {
			stopErr = s.client.Close()
		}
	})
	return stopErr
}

func (s *Session) BoundPort() int {
	if s.listener == nil {
		return s.config.BoundPort()
	}

	addr, ok := s.listener.Addr().(*net.TCPAddr)
	if !ok || addr == nil {
		return s.config.BoundPort()
	}

	return addr.Port
}

func (s *Session) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				s.reportDisconnect(err)
				return
			}
		}

		go s.handleConnection(conn)
	}
}

func (s *Session) handleConnection(conn net.Conn) {
	defer conn.Close()
	if s.config.ConnectionType == configdomain.ConnectionTypeSOCKSProxy {
		handleSOCKSProxy(conn, s.client)
		return
	}

	remote, err := s.client.Dial("tcp", fmt.Sprintf("%s:%d", s.config.RemoteHost, s.config.RemotePort))
	if err != nil {
		return
	}
	defer remote.Close()
	pipeConnections(conn, remote)
}

func (s *Session) watchConnection() {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.sendKeepalive(); err != nil {
				s.reportDisconnect(err)
				return
			}
			if err := s.probeTunnel(); err != nil {
				s.reportDisconnect(err)
				return
			}
		}
	}
}

func (s *Session) sendKeepalive() error {
	resultCh := make(chan error, 1)

	go func() {
		_, _, err := s.client.SendRequest("keepalive@ssh-man", true, nil)
		if err != nil {
			resultCh <- fmt.Errorf("ssh keepalive failed: %w", err)
			return
		}
		resultCh <- nil
	}()

	select {
	case <-s.stopCh:
		return nil
	case err := <-resultCh:
		return err
	case <-time.After(keepaliveTimeout):
		_ = s.client.Close()
		return fmt.Errorf("ssh keepalive timed out after %s: %w", keepaliveTimeout, context.DeadlineExceeded)
	}
}

func (s *Session) probeTunnel() error {
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.BoundPort()), probeTimeout)
	if err != nil {
		return fmt.Errorf("tunnel health check failed: connect local listener: %w", err)
	}
	defer localConn.Close()

	switch s.config.ConnectionType {
	case configdomain.ConnectionTypeSOCKSProxy:
		if err := s.probeSOCKSProxy(localConn); err != nil {
			return fmt.Errorf("socks proxy health check failed: %w", err)
		}
		return nil
	case configdomain.ConnectionTypeLocalForward:
		if err := verifyConnectionStaysOpen(localConn); err != nil {
			return fmt.Errorf("local forward health check failed: %w", err)
		}
		return nil
	default:
		return nil
	}
}

func (s *Session) probeSOCKSProxy(conn net.Conn) error {
	if err := conn.SetDeadline(time.Now().Add(probeTimeout)); err != nil {
		return err
	}

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("write socks handshake: %w", err)
	}

	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		return fmt.Errorf("read socks handshake response: %w", err)
	}
	if response[0] != 0x05 || response[1] != 0x00 {
		return fmt.Errorf("unexpected socks handshake response %v", response)
	}

	targetHost := "127.0.0.1"

	request, err := buildSOCKSConnectRequest(targetHost, s.server.Port)
	if err != nil {
		return err
	}
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("write socks connect request: %w", err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read socks connect response: %w", err)
	}
	if header[0] != 0x05 {
		return fmt.Errorf("unexpected socks response version %d", header[0])
	}
	if header[1] != 0x00 {
		return fmt.Errorf("socks connect request failed with code 0x%02x", header[1])
	}

	if err := discardSOCKSAddress(conn, header[3]); err != nil {
		return fmt.Errorf("read socks bind address: %w", err)
	}

	return nil
}

func verifyConnectionStaysOpen(conn net.Conn) error {
	if err := conn.SetReadDeadline(time.Now().Add(probeReadWindow)); err != nil {
		return err
	}

	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err == nil {
		return nil
	}

	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return nil
	}

	if err == io.EOF {
		return fmt.Errorf("connection closed immediately after accept")
	}

	return err
}

func buildSOCKSConnectRequest(host string, port int) ([]byte, error) {
	request := []byte{0x05, 0x01, 0x00}
	ip := net.ParseIP(host)
	switch {
	case ip != nil && ip.To4() != nil:
		request = append(request, 0x01)
		request = append(request, ip.To4()...)
	case ip != nil && ip.To16() != nil:
		request = append(request, 0x04)
		request = append(request, ip.To16()...)
	default:
		if len(host) > 255 {
			return nil, fmt.Errorf("socks health check host is too long")
		}
		request = append(request, 0x03, byte(len(host)))
		request = append(request, []byte(host)...)
	}

	request = append(request, byte(port>>8), byte(port))
	return request, nil
}

func discardSOCKSAddress(conn net.Conn, addressType byte) error {
	var discard []byte
	switch addressType {
	case 0x01:
		discard = make([]byte, 4+2)
	case 0x04:
		discard = make([]byte, 16+2)
	case 0x03:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return err
		}
		discard = make([]byte, int(length[0])+2)
	default:
		return fmt.Errorf("unsupported socks address type %d", addressType)
	}

	_, err := io.ReadFull(conn, discard)
	return err
}

func (s *Session) reportDisconnect(err error) {
	if s.stopped.Load() || s.onDisconnect == nil {
		return
	}

	s.disconnectOnce.Do(func() {
		if s.stopped.Load() {
			return
		}
		s.onDisconnect(err)
	})
}

func (s *Session) authMethod() (ssh.AuthMethod, error) {
	switch s.server.AuthMode {
	case serverdomain.AuthModeAgent:
		return auth.LoadAgentAuthMethod()
	case serverdomain.AuthModePrivateKey:
		signer, err := auth.LoadSigner(s.server.KeyReference, s.passphrase)
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("unsupported auth mode %q", s.server.AuthMode)
	}
}

func DescribeStartError(err error, server serverdomain.Server, config configdomain.ConnectionConfiguration) string {
	if err == nil {
		return "Tunnel start failed."
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "ssh_auth_sock"):
		return "SSH agent authentication is selected, but this app cannot reach your local SSH agent. Start your SSH agent, ensure SSH_AUTH_SOCK is available to the app, then retry."
	case strings.Contains(message, "bind local port"):
		return fmt.Sprintf("Couldn't listen on localhost:%d. Another app may already be using that port. Stop the conflicting app or choose another port, then retry.", config.BoundPort())
	case strings.Contains(message, "connect to ssh server"):
		return fmt.Sprintf("Couldn't reach %s:%d over SSH. Check the host, port, network connection, and SSH access, then retry.", server.Host, server.Port)
	case strings.Contains(message, "read private key"):
		return fmt.Sprintf("Couldn't read the SSH key at %q. Confirm the file exists and that the app can read it.", server.KeyReference)
	case strings.Contains(message, "parse encrypted private key"):
		return "The SSH key could not be unlocked. Verify the passphrase and key format, then retry."
	case strings.Contains(message, "parse private key"):
		return "The SSH key could not be parsed. Verify that the key file is valid and supported, then retry."
	default:
		return fmt.Sprintf("Tunnel start failed. %s", err.Error())
	}
}

func DescribeDisconnectError(err error) string {
	if err == nil {
		return "The tunnel stopped unexpectedly."
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "keepalive timed out"), strings.Contains(message, "deadline exceeded"):
		return "The SSH connection health check timed out, likely because the computer slept or the network path stalled."
	case strings.Contains(message, "keepalive"):
		return "The SSH connection stopped responding to keepalive checks."
	case strings.Contains(message, "socks proxy health check"):
		return "The SOCKS5 proxy stopped accepting or completing health-check requests."
	case strings.Contains(message, "local forward health check"):
		return "The forwarded port stopped accepting or holding health-check connections."
	case strings.Contains(message, "tunnel health check"):
		return "The local tunnel listener stopped responding to health checks."
	case strings.Contains(message, "use of closed network connection"):
		return "The local tunnel listener closed unexpectedly."
	case strings.Contains(message, "connection reset"), strings.Contains(message, "broken pipe"), strings.Contains(message, "eof"):
		return "The SSH connection was interrupted."
	default:
		return fmt.Sprintf("The tunnel stopped unexpectedly. %s", err.Error())
	}
}
