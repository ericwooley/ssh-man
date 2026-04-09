package tunnel

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"

	"golang.org/x/crypto/ssh"
)

type Session struct {
	server       serverdomain.Server
	config       configdomain.ConnectionConfiguration
	passphrase   string
	onDisconnect func(error)
	client       *ssh.Client
	listener     net.Listener
	stopCh       chan struct{}
	stopped      bool
	stopOnce     sync.Once
}

const (
	keepaliveInterval = 15 * time.Second
	keepaliveTimeout  = 5 * time.Second
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
		s.stopped = true
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
				if !s.stopped && s.onDisconnect != nil {
					s.onDisconnect(err)
				}
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
			if err := s.sendKeepalive(); err != nil && !s.stopped {
				if s.onDisconnect != nil {
					s.onDisconnect(err)
				}
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

func (s *Session) authMethod() (ssh.AuthMethod, error) {
	switch s.server.AuthMode {
	case serverdomain.AuthModeAgent:
		return nil, fmt.Errorf("ssh agent auth is not yet wired in this MVP")
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
	case strings.Contains(message, "ssh agent auth"):
		return "SSH agent authentication is not available in this MVP yet. Switch this server to private-key authentication and retry."
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
	case strings.Contains(message, "use of closed network connection"):
		return "The local tunnel listener closed unexpectedly."
	case strings.Contains(message, "connection reset"), strings.Contains(message, "broken pipe"), strings.Contains(message, "eof"):
		return "The SSH connection was interrupted."
	default:
		return fmt.Sprintf("The tunnel stopped unexpectedly. %s", err.Error())
	}
}
