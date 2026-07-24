package tunnel

import (
	"bufio"
	"net"
	"testing"
)

func TestSOCKSProxyReadsIPv6ConnectTarget(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	proxy := &SOCKSProxy{client: server, reader: bufio.NewReader(server)}

	request := []byte{0x05, 0x01, 0x00, 0x04}
	request = append(request, net.ParseIP("::1").To16()...)
	request = append(request, 0x0b, 0xb8)
	go func() {
		_, _ = client.Write(request)
	}()

	address, err := proxy.readRequest()
	if err != nil {
		t.Fatalf("read IPv6 SOCKS request: %v", err)
	}
	if address != "[::1]:3000" {
		t.Fatalf("address = %q, want [::1]:3000", address)
	}
}
