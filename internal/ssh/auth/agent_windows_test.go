//go:build windows

package auth

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

func TestWindowsOpenSSHAgentDefaultsToNamedPipe(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	endpoint := windowsAgentEndpoint()
	if endpoint != windowsOpenSSHAgentPipe {
		t.Fatalf("endpoint = %q, want %q", endpoint, windowsOpenSSHAgentPipe)
	}
}

func TestWindowsAgentUsesExplicitSocket(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", `\\.\pipe\custom-agent`)

	if endpoint := windowsAgentEndpoint(); endpoint != `\\.\pipe\custom-agent` {
		t.Fatalf("endpoint = %q, want explicit SSH_AUTH_SOCK", endpoint)
	}
}

func TestWindowsAgentRecognizesNamedPipeCaseInsensitively(t *testing.T) {
	for _, endpoint := range []string{
		`\\.\pipe\openssh-ssh-agent`,
		`\\.\PIPE\custom-agent`,
	} {
		if !isWindowsNamedPipe(endpoint) {
			t.Fatalf("isWindowsNamedPipe(%q) = false", endpoint)
		}
	}
	if isWindowsNamedPipe(`/tmp/ssh-agent.sock`) {
		t.Fatal("Unix socket was recognized as a Windows named pipe")
	}
}

func TestWindowsAgentDialsNamedPipe(t *testing.T) {
	endpoint := fmt.Sprintf(`\\.\pipe\ssh-man-agent-test-%d`, os.Getpid())
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		t.Fatalf("ListenPipe() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()

	t.Setenv("SSH_AUTH_SOCK", endpoint)
	client, gotEndpoint, err := dialWindowsAgent()
	if err != nil {
		t.Fatalf("dialWindowsAgent() error = %v", err)
	}
	defer client.Close()
	if gotEndpoint != endpoint {
		t.Fatalf("endpoint = %q, want %q", gotEndpoint, endpoint)
	}

	select {
	case server := <-accepted:
		if server == nil {
			t.Fatal("named-pipe listener returned a nil connection")
		}
		_ = server.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for named-pipe connection")
	}
}
