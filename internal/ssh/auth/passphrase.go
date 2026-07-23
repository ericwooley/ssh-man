package auth

import (
	"errors"
	"strings"
)

var ErrPassphraseRequired = errors.New("ssh key requires a passphrase")

// IsAuthenticationRejected reports whether an SSH server completed the
// handshake but rejected the credentials offered by the client.
func IsAuthenticationRejected(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unable to authenticate") ||
		strings.Contains(message, "permission denied (publickey")
}

func isPassphraseRequired(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrPassphraseRequired) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "cannot decode encrypted private keys") || strings.Contains(message, "passphrase")
}
