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

	handshakeDeadline := time.Now().Add(sshDialTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(handshakeDeadline) {
		handshakeDeadline = contextDeadline
	}
	_ = netConn.SetDeadline(handshakeDeadline)
	config := &ssh.ClientConfig{
		User:              server.Username,
		Auth:              []ssh.AuthMethod{authMethod},
		HostKeyCallback:   hostKeyCallback,
		HostKeyAlgorithms: hostKeyAlgorithms,
		Timeout:           sshDialTimeout,
	}
	sshConn, channels, requests, err := ssh.NewClientConn(netConn, endpoint.HostKeyAddress, config)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("connect to ssh server: %w", err)
	}
	_ = netConn.SetDeadline(time.Time{})
	return ssh.NewClient(sshConn, channels, requests), nil
}
