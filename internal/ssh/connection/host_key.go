package connection

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var systemKnownHostsPaths = []string{
	"/etc/ssh/ssh_known_hosts",
	"/etc/ssh/ssh_known_hosts2",
}

// KnownHostsCallback verifies SSH server identities against the same default
// user and system known_hosts files used by OpenSSH.
func KnownHostsCallback() (ssh.HostKeyCallback, error) {
	candidates := make([]string, 0, 4)
	homeDirectory, homeErr := os.UserHomeDir()
	if homeErr == nil {
		candidates = append(candidates,
			filepath.Join(homeDirectory, ".ssh", "known_hosts"),
			filepath.Join(homeDirectory, ".ssh", "known_hosts2"),
		)
	}
	candidates = append(candidates, systemKnownHostsPaths...)

	files, err := existingKnownHostsFiles(candidates)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 && homeErr != nil {
		return nil, fmt.Errorf("find SSH home directory: %w", homeErr)
	}
	return hostKeyCallbackFromFiles(files)
}

func existingKnownHostsFiles(candidates []string) ([]string, error) {
	files := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect SSH known_hosts file %q: %w", candidate, err)
		}
		if info.Mode().IsRegular() {
			files = append(files, candidate)
		}
	}
	return files, nil
}

func hostKeyCallbackFromFiles(files []string) (ssh.HostKeyCallback, error) {
	if len(files) == 0 {
		return nil, errors.New("SSH host key verification requires an existing ~/.ssh/known_hosts file; connect with OpenSSH once to trust this server")
	}
	callback, err := knownhosts.New(files...)
	if err != nil {
		return nil, fmt.Errorf("load SSH known_hosts: %w", err)
	}
	return callback, nil
}
