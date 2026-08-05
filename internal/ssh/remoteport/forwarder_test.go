package remoteport

import (
	"context"
	"io"
	"net"
	"strconv"
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

type echoRemoteClient struct{}

func (echoRemoteClient) Dial(network, address string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		_, _ = io.Copy(server, server)
	}()
	return client, nil
}

func (echoRemoteClient) Close() error {
	return nil
}

func TestForwarderOpensReusableLoopbackForward(t *testing.T) {
	forwarder := NewForwarderWithDialer(
		serverdomain.Server{ID: "server-1"},
		func(context.Context, serverdomain.Server, string) (RemoteClient, error) {
			return echoRemoteClient{}, nil
		},
	)
	t.Cleanup(func() {
		_ = forwarder.Close()
	})

	first, err := forwarder.Open(context.Background(), "", 3000, []string{"0.0.0.0", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := forwarder.Open(context.Background(), "", 3000, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.LocalPort < 1 || first.RemoteHost != "127.0.0.1" {
		t.Fatalf("forwards = %#v and %#v", first, second)
	}

	connection, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(first.LocalPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 5)
	if _, err := io.ReadFull(connection, reply); err != nil {
		t.Fatal(err)
	}
	if string(reply) != "hello" {
		t.Fatalf("forward reply = %q", reply)
	}
}

func TestRemoteHostForAddressesPrefersReachableLoopback(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
		want      string
	}{
		{name: "ipv4 wildcard", addresses: []string{"0.0.0.0"}, want: "127.0.0.1"},
		{name: "ipv6 wildcard", addresses: []string{"::"}, want: "::1"},
		{name: "ipv6 loopback", addresses: []string{"::1"}, want: "::1"},
		{name: "host address", addresses: []string{"10.0.0.4"}, want: "10.0.0.4"},
		{name: "star", addresses: []string{"*"}, want: "127.0.0.1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := remoteHostForAddresses(test.addresses); got != test.want {
				t.Fatalf("remoteHostForAddresses(%#v) = %q, want %q", test.addresses, got, test.want)
			}
		})
	}
}
