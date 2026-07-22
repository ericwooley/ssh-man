package connection

import (
	"fmt"

	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"

	"golang.org/x/crypto/ssh"
)

// AuthMethod builds the SSH authentication method for a saved server. Keeping
// this shared avoids subtle differences between tunnels and file explorers.
func AuthMethod(server serverdomain.Server, passphrase string) (ssh.AuthMethod, error) {
	switch server.AuthMode {
	case serverdomain.AuthModeAgent:
		return auth.LoadAgentAuthMethod()
	case serverdomain.AuthModePrivateKey:
		signer, err := auth.LoadSigner(server.KeyReference, passphrase)
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("unsupported auth mode %q", server.AuthMode)
	}
}
