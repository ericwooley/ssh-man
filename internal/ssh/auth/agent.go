package auth

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

func LoadAgentAuthMethod() (ssh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("ssh agent auth unavailable: SSH_AUTH_SOCK is not set")
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("ssh agent auth unavailable: connect to SSH_AUTH_SOCK: %w", err)
	}

	agentClient := sshagent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}
