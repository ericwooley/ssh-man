package tunnel

import (
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

func pipeConnections(left net.Conn, right net.Conn) {
	var wg sync.WaitGroup
	copyStream := func(dst net.Conn, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		_ = dst.Close()
	}
	wg.Add(2)
	go copyStream(left, right)
	go copyStream(right, left)
	wg.Wait()
}

func handleSOCKSProxy(clientConn net.Conn, sshClient *ssh.Client) {
	proxy, err := NewSOCKSProxy(clientConn, sshClient)
	if err != nil {
		return
	}
	_ = proxy.Serve()
}
