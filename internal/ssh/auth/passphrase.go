package auth

import (
	"errors"
	"strings"
)

var ErrPassphraseRequired = errors.New("ssh key requires a passphrase")

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
