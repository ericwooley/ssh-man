package tunnel

import (
	"bytes"
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

func TestDescribeDisconnectErrorForSOCKSHealthCheckFailure(t *testing.T) {
	message := DescribeDisconnectError(errors.New("socks proxy health check failed: read socks handshake response: EOF"))
	if want := "SOCKS5 proxy"; !strings.Contains(message, want) {
		t.Fatalf("expected %q in %q", want, message)
	}
}

func TestDescribeDisconnectErrorForLocalForwardHealthCheckFailure(t *testing.T) {
	message := DescribeDisconnectError(errors.New("local forward health check failed: connection closed immediately after accept"))
	if want := "forwarded port"; !strings.Contains(message, want) {
		t.Fatalf("expected %q in %q", want, message)
	}
}

func TestBuildSOCKSConnectRequestForIPv4(t *testing.T) {
	request, err := buildSOCKSConnectRequest("127.0.0.1", 22)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	want := []byte{0x05, 0x01, 0x00, 0x01, 127, 0, 0, 1, 0x00, 0x16}
	if !bytes.Equal(request, want) {
		t.Fatalf("unexpected request bytes: got %v want %v", request, want)
	}
}

func TestBuildSOCKSConnectRequestForDomain(t *testing.T) {
	request, err := buildSOCKSConnectRequest("example.com", 443)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	prefix := []byte{0x05, 0x01, 0x00, 0x03, byte(len("example.com"))}
	if !bytes.Equal(request[:len(prefix)], prefix) {
		t.Fatalf("unexpected request prefix: got %v want %v", request[:len(prefix)], prefix)
	}
	if got := string(request[len(prefix) : len(prefix)+len("example.com")]); got != "example.com" {
		t.Fatalf("unexpected host encoding: %q", got)
	}
	if request[len(request)-2] != 0x01 || request[len(request)-1] != 0xbb {
		t.Fatalf("unexpected port encoding: %v", request[len(request)-2:])
	}
}
