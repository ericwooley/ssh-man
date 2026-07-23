package connection

import (
	"errors"
	"fmt"
	"io"
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
	files, err := knownHostsFiles()
	if err != nil {
		return nil, err
	}
	return hostKeyCallbackFromFiles(files)
}

func knownHostsConfiguration(address string) (ssh.HostKeyCallback, []string, error) {
	files, err := knownHostsFiles()
	if err != nil {
		return nil, nil, err
	}
	callback, err := hostKeyCallbackFromFiles(files)
	if err != nil {
		return nil, nil, err
	}
	algorithms, err := hostKeyAlgorithmsForAddress(callback, files, address)
	if err != nil {
		return nil, nil, err
	}
	return callback, algorithms, nil
}

func knownHostsFiles() ([]string, error) {
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
	return files, nil
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

func hostKeyAlgorithmsForAddress(callback ssh.HostKeyCallback, files []string, address string) ([]string, error) {
	var preferred []string
	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read SSH known_hosts file %q: %w", file, err)
		}
		for {
			_, _, key, _, rest, err := ssh.ParseKnownHosts(contents)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("parse SSH known_hosts file %q: %w", file, err)
			}
			contents = rest
			if err := callback(address, knownHostAddress(address), key); err == nil {
				preferred = appendUnique(preferred, algorithmsForKnownHostKey(key)...)
			}
		}
	}
	if len(preferred) == 0 {
		return nil, nil
	}

	algorithms := append([]string{}, preferred...)
	algorithms = appendUnique(algorithms, ssh.SupportedAlgorithms().HostKeys...)
	algorithms = appendUnique(algorithms, ssh.InsecureAlgorithms().HostKeys...)
	return algorithms, nil
}

func algorithmsForKnownHostKey(key ssh.PublicKey) []string {
	if key.Type() == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSA}
	}
	return []string{key.Type()}
}

func appendUnique(values []string, candidates ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(candidates))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		values = append(values, candidate)
	}
	return values
}

type knownHostAddress string

func (a knownHostAddress) Network() string {
	return "tcp"
}

func (a knownHostAddress) String() string {
	return string(a)
}
