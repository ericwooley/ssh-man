package tunnel

import (
	"errors"
	"strings"
	"testing"

	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
)

func TestDescribeStartErrorForBoundPortConflict(t *testing.T) {
	message := DescribeStartError(
		errors.New("bind local port 127.0.0.1:1080: address already in use"),
		serverdomain.Server{Host: "example.com", Port: 22, KeyReference: "~/.ssh/id_ed25519"},
		configdomain.ConnectionConfiguration{ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080},
	)
	if message == "" || message == "Tunnel start failed." {
		t.Fatalf("expected actionable message, got %q", message)
	}
	if want := "Another app may already be using that port"; !strings.Contains(message, want) {
		t.Fatalf("expected %q in %q", want, message)
	}
}

func TestDescribeDisconnectErrorForKeepaliveFailure(t *testing.T) {
	message := DescribeDisconnectError(errors.New("ssh keepalive failed: EOF"))
	if want := "keepalive"; !strings.Contains(message, want) {
		t.Fatalf("expected %q in %q", want, message)
	}
}

func TestDescribeDisconnectErrorForKeepaliveTimeout(t *testing.T) {
	message := DescribeDisconnectError(errors.New("ssh keepalive timed out after 5s: context deadline exceeded"))
	if want := "slept"; !strings.Contains(strings.ToLower(message), want) {
		t.Fatalf("expected %q in %q", want, message)
	}
}

func TestDescribeStartErrorForMissingSSHAgentSocket(t *testing.T) {
	message := DescribeStartError(
		errors.New("ssh agent auth unavailable: SSH_AUTH_SOCK is not set"),
		serverdomain.Server{Host: "example.com", Port: 22},
		configdomain.ConnectionConfiguration{ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080},
	)
	if want := "cannot reach your local SSH agent"; !strings.Contains(message, want) {
		t.Fatalf("expected %q in %q", want, message)
	}
}
