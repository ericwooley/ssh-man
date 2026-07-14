package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverPrivateKeys returns readable private-key files in sshDir. It only
// returns paths; key material is never included in the result.
func DiscoverPrivateKeys(sshDir string) ([]string, error) {
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil, fmt.Errorf("read SSH directory: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}

		path := filepath.Join(sshDir, entry.Name())
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		contents, err := os.ReadFile(path)
		if err != nil || !looksLikePrivateKey(contents) {
			continue
		}
		paths = append(paths, path)
	}

	sort.Strings(paths)
	return paths, nil
}

func looksLikePrivateKey(contents []byte) bool {
	text := string(contents)
	return strings.Contains(text, "-----BEGIN OPENSSH PRIVATE KEY-----") ||
		strings.Contains(text, "-----BEGIN RSA PRIVATE KEY-----") ||
		strings.Contains(text, "-----BEGIN EC PRIVATE KEY-----") ||
		strings.Contains(text, "-----BEGIN PRIVATE KEY-----")
}
