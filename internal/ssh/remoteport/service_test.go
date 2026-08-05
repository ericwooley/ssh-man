package remoteport

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	serverdomain "ssh-man/internal/domain/server"

	"golang.org/x/crypto/ssh"
)

func TestParseListeningPortsGroupsAddressesAndSortsPorts(t *testing.T) {
	output := []byte(`
127.0.0.1:3000
[::]:443
0.0.0.0:3000
*.8080
invalid
127.0.0.1:70000
`)

	got := parseListeningPorts(output)
	want := []ListeningPort{
		{Port: 443, Addresses: []string{"::"}, SuggestedScheme: SchemeHTTPS},
		{Port: 3000, Addresses: []string{"0.0.0.0", "127.0.0.1"}, SuggestedScheme: SchemeHTTP},
		{Port: 8080, Addresses: []string{"*"}, SuggestedScheme: SchemeHTTP},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseListeningPorts() = %#v, want %#v", got, want)
	}
}

func TestServiceDiscoverUsesStaticCommandAndReturnsPorts(t *testing.T) {
	server := serverdomain.Server{ID: "server-1"}
	var gotServer serverdomain.Server
	var gotPassphrase string
	var gotCommand string
	service := NewServiceWithRunner(server, func(_ context.Context, inputServer serverdomain.Server, passphrase, command string) ([]byte, error) {
		gotServer = inputServer
		gotPassphrase = passphrase
		gotCommand = command
		return []byte("127.0.0.1:5173\n"), nil
	})

	got, err := service.Discover(context.Background(), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if gotServer.ID != server.ID || gotPassphrase != "secret" {
		t.Fatalf("runner input = %#v, %q", gotServer, gotPassphrase)
	}
	if gotCommand != discoveryCommand {
		t.Fatalf("command = %q, want static discovery command", gotCommand)
	}
	if len(got) != 1 || got[0].Port != 5173 {
		t.Fatalf("ports = %#v", got)
	}
}

func TestHandshakeSSHClientStopsWhenContextExpires(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	t.Cleanup(func() {
		_ = clientConnection.Close()
		_ = serverConnection.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	_, err := handshakeSSHClient(ctx, clientConnection, "example.com:22", &ssh.ClientConfig{
		User:            "eric",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("handshake error = %v, want context deadline", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("handshake stopped after %v, want less than one second", elapsed)
	}
}
