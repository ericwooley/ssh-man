package auth

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverPrivateKeysReturnsSortedPrivateKeyPathsOnly(t *testing.T) {
	sshDir := t.TempDir()
	files := map[string]string{
		"id_zeta":      "-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n-----END OPENSSH PRIVATE KEY-----\n",
		"id_alpha":     "-----BEGIN RSA PRIVATE KEY-----\nkey\n-----END RSA PRIVATE KEY-----\n",
		"id_alpha.pub": "ssh-rsa AAAA public\n",
		"config":       "Host example.com\n",
		"known_hosts":  "example.com ssh-ed25519 AAAA\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(sshDir, name), []byte(contents), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(sshDir, "nested"), 0o700); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}

	got, err := DiscoverPrivateKeys(sshDir)
	if err != nil {
		t.Fatalf("discover private keys: %v", err)
	}
	want := []string{filepath.Join(sshDir, "id_alpha"), filepath.Join(sshDir, "id_zeta")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DiscoverPrivateKeys() = %v, want %v", got, want)
	}
}

func TestDiscoverPrivateKeysReportsMissingDirectory(t *testing.T) {
	_, err := DiscoverPrivateKeys(filepath.Join(t.TempDir(), ".ssh"))
	if err == nil {
		t.Fatal("expected missing SSH directory error")
	}
}
