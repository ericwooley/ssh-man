package connection

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type deadlineSignalConn struct {
	net.Conn
	deadlineSet chan struct{}
	once        sync.Once
}

func (connection *deadlineSignalConn) SetDeadline(deadline time.Time) error {
	err := connection.Conn.SetDeadline(deadline)
	connection.once.Do(func() {
		close(connection.deadlineSet)
	})
	return err
}

func TestHandshakeSSHClientStopsWhenContextIsCanceled(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	connection := &deadlineSignalConn{
		Conn:        clientConnection,
		deadlineSet: make(chan struct{}),
	}
	t.Cleanup(func() {
		_ = clientConnection.Close()
		_ = serverConnection.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := handshakeSSHClient(ctx, connection, "example.com:22", &ssh.ClientConfig{
			User:            "eric",
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		result <- err
	}()

	select {
	case <-connection.deadlineSet:
	case <-time.After(time.Second):
		t.Fatal("SSH handshake did not set its deadline")
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handshake error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SSH handshake did not stop after context cancellation")
	}
}
