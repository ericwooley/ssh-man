package auth

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

func LoadSigner(path string, passphrase string) (ssh.Signer, error) {
	privateKey, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	if passphrase == "" {
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err == nil {
			return signer, nil
		}
		if isPassphraseRequired(err) {
			return nil, ErrPassphraseRequired
		}
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKeyWithPassphrase(privateKey, []byte(passphrase))
	if err != nil {
		if isPassphraseRequired(err) {
			return nil, ErrPassphraseRequired
		}
		return nil, fmt.Errorf("parse encrypted private key: %w", err)
	}
	return signer, nil
}
