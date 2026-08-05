package remoteport

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	serverdomain "ssh-man/internal/domain/server"

	"golang.org/x/crypto/ssh"
)

func TestParseListeningPortsGroupsAddressesAndSortsPorts(t *testing.T) {
	output := []byte(`
127.0.0.1:3000
[::]:443
::1.8021
0.0.0.0:3000
*.8080
invalid
127.0.0.1:70000
`)

	got := parseListeningPorts(output)
	want := []ListeningPort{
		{Port: 443, Addresses: []string{"::"}, SuggestedScheme: SchemeHTTPS},
		{Port: 3000, Addresses: []string{"0.0.0.0", "127.0.0.1"}, SuggestedScheme: SchemeHTTP},
		{Port: 8021, Addresses: []string{"::1"}, SuggestedScheme: SchemeHTTP},
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

type blockingClient struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingClient() *blockingClient {
	return &blockingClient{closed: make(chan struct{})}
}

func (client *blockingClient) Close() error {
	client.once.Do(func() {
		close(client.closed)
	})
	return nil
}

type blockingSession struct {
	start func() error
}

func (session *blockingSession) Start(string) error {
	return session.start()
}

func (*blockingSession) Wait() error {
	return nil
}

func (*blockingSession) Close() error {
	return nil
}

func TestRunSSHClientCommandStopsStalledSessionOpen(t *testing.T) {
	client := newBlockingClient()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	_, err := runSSHClientCommand(ctx, client, func(io.Writer, io.Writer) (commandSession, error) {
		<-client.closed
		return nil, errors.New("client closed")
	}, discoveryCommand)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("session-open error = %v, want context deadline", err)
	}
}

func TestRunSSHClientCommandStopsStalledCommandReply(t *testing.T) {
	client := newBlockingClient()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	session := &blockingSession{start: func() error {
		<-client.closed
		return errors.New("client closed")
	}}

	_, err := runSSHClientCommand(ctx, client, func(io.Writer, io.Writer) (commandSession, error) {
		return session, nil
	}, discoveryCommand)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("command-start error = %v, want context deadline", err)
	}
}

type writingSession struct {
	stdout    io.Writer
	stderr    io.Writer
	useStderr bool
}

func (session *writingSession) Start(string) error {
	output := session.stdout
	if session.useStderr {
		output = session.stderr
	}
	_, _ = output.Write(make([]byte, 2<<20))
	return nil
}

func (*writingSession) Wait() error {
	return nil
}

func (*writingSession) Close() error {
	return nil
}

func TestRunSSHClientCommandRejectsUnboundedRemoteOutput(t *testing.T) {
	for _, useStderr := range []bool{false, true} {
		stream := "stdout"
		if useStderr {
			stream = "stderr"
		}
		t.Run(stream, func(t *testing.T) {
			client := newBlockingClient()
			_, err := runSSHClientCommand(
				context.Background(),
				client,
				func(stdout, stderr io.Writer) (commandSession, error) {
					return &writingSession{stdout: stdout, stderr: stderr, useStderr: useStderr}, nil
				},
				discoveryCommand,
			)
			if !errors.Is(err, errDiscoveryOutputLimit) {
				t.Fatalf("output error = %v, want local limit error", err)
			}
		})
	}
}
