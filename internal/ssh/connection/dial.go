package connection

import (
	"context"
	"fmt"
	"net"
	"time"

	serverdomain "ssh-man/internal/domain/server"

	"golang.org/x/crypto/ssh"
)

const sshDialTimeout = 10 * time.Second

// DialSSH opens a verified SSH connection using the effective OpenSSH Host
// configuration while keeping authentication controlled by the saved server.
func DialSSH(ctx context.Context, server serverdomain.Server, authMethod ssh.AuthMethod) (*ssh.Client, error) {
	endpoint, err := resolveOpenSSHEndpoint(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("connect to ssh server: resolve OpenSSH configuration: %w", err)
	}
	hostKeyCallback, hostKeyAlgorithms, err := knownHostsConfiguration(endpoint.HostKeyAddress)
	if err != nil {
		return nil, fmt.Errorf("configure SSH host key verification: %w", err)
	}

	netConn, err := (&net.Dialer{Timeout: sshDialTimeout}).DialContext(ctx, "tcp", endpoint.DialAddress)
	if err != nil {
		return nil, fmt.Errorf("connect to ssh server: %w", err)
	}

	config := &ssh.ClientConfig{
		User:              server.Username,
		Auth:              []ssh.AuthMethod{authMethod},
		HostKeyCallback:   hostKeyCallback,
		HostKeyAlgorithms: hostKeyAlgorithms,
		Timeout:           sshDialTimeout,
	}
	client, err := handshakeSSHClient(ctx, netConn, endpoint.HostKeyAddress, config)
	if err != nil {
		return nil, fmt.Errorf("connect to ssh server: %w", err)
	}
	return client, nil
}

func handshakeSSHClient(
	ctx context.Context,
	netConn net.Conn,
	hostKeyAddress string,
	config *ssh.ClientConfig,
) (*ssh.Client, error) {
	handshakeDeadline := time.Now().Add(sshDialTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(handshakeDeadline) {
		handshakeDeadline = contextDeadline
	}
	if err := netConn.SetDeadline(handshakeDeadline); err != nil {
		_ = netConn.Close()
		return nil, err
	}

	handshakeDone := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			_ = netConn.SetDeadline(time.Now())
		case <-handshakeDone:
		}
	}()

	sshConn, channels, requests, err := ssh.NewClientConn(netConn, hostKeyAddress, config)
	close(handshakeDone)
	<-watcherDone
	if err != nil {
		_ = netConn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	if ctx.Err() != nil {
		_ = sshConn.Close()
		return nil, ctx.Err()
	}
	if err := netConn.SetDeadline(time.Time{}); err != nil {
		_ = sshConn.Close()
		return nil, err
	}
	return ssh.NewClient(sshConn, channels, requests), nil
}
