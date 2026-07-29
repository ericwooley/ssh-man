//go:build windows

package auth

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

const windowsOpenSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`
const windowsAgentDialTimeout = 2 * time.Second

func LoadAgentAuthMethod() (ssh.AuthMethod, error) {
	conn, endpoint, err := dialWindowsAgent()
	if err != nil {
		return nil, fmt.Errorf("ssh agent auth unavailable: connect to %s: %w", endpoint, err)
	}

	agentClient := sshagent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

func dialWindowsAgent() (net.Conn, string, error) {
	endpoint := windowsAgentEndpoint()

	if isWindowsNamedPipe(endpoint) {
		timeout := windowsAgentDialTimeout
		conn, err := winio.DialPipe(endpoint, &timeout)
		return conn, endpoint, err
	}

	conn, err := net.Dial("unix", endpoint)
	return conn, endpoint, err
}

func windowsAgentEndpoint() string {
	endpoint := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	if endpoint != "" {
		return endpoint
	}
	return windowsOpenSSHAgentPipe
}

func isWindowsNamedPipe(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(endpoint), `\\.\pipe\`)
}
