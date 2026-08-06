package remoteport

import (
	"context"
	"errors"
	"io"
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

func TestDialSSHClientUsesResolvedConnectionDialer(t *testing.T) {
	server := serverdomain.Server{
		ID:       "server-1",
		Host:     "production-alias",
		Port:     22,
		Username: "eric",
	}
	wantErr := errors.New("resolved dial failed")
	var gotAuthServer serverdomain.Server
	var gotPassphrase string
	var gotDialServer serverdomain.Server

	originalDependencies := defaultSSHClientDependencies
	defaultSSHClientDependencies = sshClientDependencies{
		authFactory: func(input serverdomain.Server, passphrase string) (ssh.AuthMethod, error) {
			gotAuthServer = input
			gotPassphrase = passphrase
			return ssh.Password("password"), nil
		},
		dial: func(_ context.Context, input serverdomain.Server, _ ssh.AuthMethod) (*ssh.Client, error) {
			gotDialServer = input
			return nil, wantErr
		},
	}
	t.Cleanup(func() {
		defaultSSHClientDependencies = originalDependencies
	})

	_, err := dialSSHClient(context.Background(), server, "secret")

	if !errors.Is(err, wantErr) {
		t.Fatalf("dial error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(gotAuthServer, server) || gotPassphrase != "secret" {
		t.Fatalf("auth input = %#v, %q", gotAuthServer, gotPassphrase)
	}
	if !reflect.DeepEqual(gotDialServer, server) {
		t.Fatalf("resolved dial server = %#v, want %#v", gotDialServer, server)
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
