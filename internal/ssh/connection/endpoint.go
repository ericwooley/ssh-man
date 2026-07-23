package connection

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	serverdomain "ssh-man/internal/domain/server"
)

// OpenSSHSettings contains the effective connection fields SSH Man needs from
// `ssh -G`. OpenSSH remains responsible for Host/Include/Match/token semantics.
type OpenSSHSettings struct {
	Hostname     string
	Port         int
	HostKeyAlias string
}

// Endpoint separates the address used for the network connection from the
// address supplied to known_hosts verification.
type Endpoint struct {
	DialAddress    string
	HostKeyAddress string
}

func resolveOpenSSHEndpoint(ctx context.Context, server serverdomain.Server) (Endpoint, error) {
	settings, err := inspectOpenSSHSettings(ctx, server.Host)
	if err != nil {
		return Endpoint{}, err
	}
	return resolveEndpoint(server, settings)
}

func inspectOpenSSHSettings(ctx context.Context, host string) (OpenSSHSettings, error) {
	output, err := exec.CommandContext(ctx, "ssh", "-G", "--", host).Output()
	if errors.Is(err, exec.ErrNotFound) {
		return OpenSSHSettings{}, nil
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			message := strings.TrimSpace(string(exitErr.Stderr))
			if message != "" {
				return OpenSSHSettings{}, fmt.Errorf("inspect host %q: %s", host, message)
			}
		}
		return OpenSSHSettings{}, fmt.Errorf("inspect host %q: %w", host, err)
	}
	return parseOpenSSHSettings(output)
}

func parseOpenSSHSettings(output []byte) (OpenSSHSettings, error) {
	var settings OpenSSHSettings
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), " ")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(key) {
		case "hostname":
			if settings.Hostname == "" {
				settings.Hostname = value
			}
		case "port":
			if settings.Port != 0 {
				continue
			}
			port, err := strconv.Atoi(value)
			if err != nil || port < 1 || port > 65535 {
				return OpenSSHSettings{}, fmt.Errorf("invalid OpenSSH port %q", value)
			}
			settings.Port = port
		case "hostkeyalias":
			if settings.HostKeyAlias == "" && !strings.EqualFold(value, "none") {
				settings.HostKeyAlias = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return OpenSSHSettings{}, fmt.Errorf("read effective OpenSSH configuration: %w", err)
	}
	return settings, nil
}

func resolveEndpoint(server serverdomain.Server, settings OpenSSHSettings) (Endpoint, error) {
	host := strings.TrimSpace(settings.Hostname)
	if host == "" {
		host = strings.TrimSpace(server.Host)
	}
	if host == "" {
		return Endpoint{}, errors.New("SSH host is required")
	}

	port := server.Port
	if port == 22 && settings.Port != 0 {
		port = settings.Port
	}
	if port < 1 || port > 65535 {
		return Endpoint{}, fmt.Errorf("SSH port must be between 1 and 65535")
	}

	hostKeyHost := strings.TrimSpace(settings.HostKeyAlias)
	if hostKeyHost == "" {
		hostKeyHost = host
	}
	portValue := strconv.Itoa(port)
	return Endpoint{
		DialAddress:    net.JoinHostPort(host, portValue),
		HostKeyAddress: net.JoinHostPort(hostKeyHost, portValue),
	}, nil
}
