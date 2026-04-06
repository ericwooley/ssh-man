package tunnel

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"golang.org/x/crypto/ssh"
)

type SOCKSProxy struct {
	client    net.Conn
	sshClient *ssh.Client
	reader    *bufio.Reader
}

func NewSOCKSProxy(client net.Conn, sshClient *ssh.Client) (*SOCKSProxy, error) {
	return &SOCKSProxy{client: client, sshClient: sshClient, reader: bufio.NewReader(client)}, nil
}

func (p *SOCKSProxy) Serve() error {
	defer p.client.Close()
	if err := p.handshake(); err != nil {
		return err
	}
	address, err := p.readRequest()
	if err != nil {
		return err
	}

	remote, err := p.sshClient.Dial("tcp", address)
	if err != nil {
		_, _ = p.client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return err
	}
	defer remote.Close()
	_, _ = p.client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	go io.Copy(remote, p.client)
	_, _ = io.Copy(p.client, remote)
	return nil
}

func (p *SOCKSProxy) handshake() error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(p.reader, header); err != nil {
		return err
	}
	if header[0] != 0x05 {
		return fmt.Errorf("unsupported socks version")
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(p.reader, methods); err != nil {
		return err
	}
	_, err := p.client.Write([]byte{0x05, 0x00})
	return err
}

func (p *SOCKSProxy) readRequest() (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(p.reader, header); err != nil {
		return "", err
	}
	if header[1] != 0x01 {
		return "", fmt.Errorf("unsupported socks command")
	}

	var host string
	switch header[3] {
	case 0x01:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(p.reader, addr); err != nil {
			return "", err
		}
		host = net.IP(addr).String()
	case 0x03:
		length, err := p.reader.ReadByte()
		if err != nil {
			return "", err
		}
		addr := make([]byte, int(length))
		if _, err := io.ReadFull(p.reader, addr); err != nil {
			return "", err
		}
		host = string(addr)
	default:
		return "", fmt.Errorf("unsupported address type")
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(p.reader, portBytes); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBytes)
	return fmt.Sprintf("%s:%d", host, port), nil
}
