//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ssh-man/internal/control"
	configdomain "ssh-man/internal/domain/config"
	serverdomain "ssh-man/internal/domain/server"
	sessiondomain "ssh-man/internal/domain/session"
)

func TestCLIUsesTheOwnerControlSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "ssh-man-cli-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "control.sock")

	state := control.State{
		Servers: []control.ServerRecord{{
			Server: serverdomain.Server{
				ID: "server-1", Name: "Production", Host: "ssh.example.com",
				Port: 22, Username: "deploy", AuthMode: serverdomain.AuthModePrivateKey,
				KeyReference: "~/.ssh/id_ed25519",
			},
			Configurations: []configdomain.ConnectionConfiguration{{
				ID: "tunnel-1", ServerID: "server-1", Label: "Browser",
				ConnectionType: configdomain.ConnectionTypeSOCKSProxy, SocksPort: 1080,
			}},
		}},
		Sessions: []sessiondomain.RuntimeSession{{
			ConfigurationID: "tunnel-1", Status: sessiondomain.StatusNeedsAttention,
		}},
	}
	secretReceived := ""
	server := control.NewServer(socketPath, control.Backend{
		State: func(context.Context) (control.State, error) { return state, nil },
		Unlock: func(_ context.Context, configurationID string, secret string) (sessiondomain.RuntimeSession, error) {
			secretReceived = secret
			return sessiondomain.RuntimeSession{
				ConfigurationID: configurationID,
				Status:          sessiondomain.StatusConnected,
				BoundPort:       1080,
			}, nil
		},
	})
	if err := server.Start(); err != nil {
		t.Fatalf("control Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Errorf("control Stop() error = %v", err)
		}
	})

	connect := func(context.Context, ConnectOptions) (Caller, error) {
		return control.NewClient(socketPath, time.Second), nil
	}
	var listOutput bytes.Buffer
	var listErrors bytes.Buffer
	listCode := Run(context.Background(), []string{"server", "list", "--output", "json"}, Dependencies{
		Connect: connect, Stdin: strings.NewReader(""), Stdout: &listOutput, Stderr: &listErrors,
	})
	if listCode != ExitOK || listErrors.Len() != 0 || !strings.Contains(listOutput.String(), `"id": "server-1"`) {
		t.Fatalf("server list code=%d stdout=%q stderr=%q", listCode, listOutput.String(), listErrors.String())
	}

	var unlockOutput bytes.Buffer
	var unlockErrors bytes.Buffer
	unlockCode := Run(context.Background(), []string{
		"tunnel", "unlock", "Browser", "--server", "Production", "--passphrase-stdin", "--output", "json",
	}, Dependencies{
		Connect: connect, Stdin: strings.NewReader("socket-only-secret\n"), Stdout: &unlockOutput, Stderr: &unlockErrors,
	})
	if unlockCode != ExitOK || unlockErrors.Len() != 0 {
		t.Fatalf("unlock code=%d stdout=%q stderr=%q", unlockCode, unlockOutput.String(), unlockErrors.String())
	}
	if secretReceived != "socket-only-secret" {
		t.Fatalf("owner received secret %q", secretReceived)
	}
	if strings.Contains(unlockOutput.String(), secretReceived) || strings.Contains(unlockErrors.String(), secretReceived) {
		t.Fatal("unlock passphrase leaked to CLI output")
	}
}
