package connection

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestHostKeyCallbackFromFilesVerifiesKnownHostIdentity(t *testing.T) {
	knownKey := testHostPublicKey(t)
	changedKey := testHostPublicKey(t)
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{"example.test"}, knownKey) + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	callback, err := hostKeyCallbackFromFiles([]string{knownHostsPath})
	if err != nil {
		t.Fatal(err)
	}
	remote := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}

	if err := callback("example.test:22", remote, knownKey); err != nil {
		t.Fatalf("known host key rejected: %v", err)
	}
	if err := callback("example.test:22", remote, changedKey); err == nil {
		t.Fatal("changed host key unexpectedly accepted")
	}
	if err := callback("unknown.test:22", remote, knownKey); err == nil {
		t.Fatal("unknown host unexpectedly accepted")
	}
}

func TestHostKeyCallbackFromFilesRequiresAKnownHostsSource(t *testing.T) {
	if _, err := hostKeyCallbackFromFiles(nil); err == nil {
		t.Fatal("empty known_hosts source unexpectedly accepted")
	}
}

func TestHostKeyAlgorithmsForAddressPrioritizesTheTrustedKeyType(t *testing.T) {
	knownKey := testHostPublicKey(t)
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{"mac.example.test"}, knownKey) + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	callback, err := hostKeyCallbackFromFiles([]string{knownHostsPath})
	if err != nil {
		t.Fatal(err)
	}
	algorithms, err := hostKeyAlgorithmsForAddress(callback, []string{knownHostsPath}, "mac.example.test:22")
	if err != nil {
		t.Fatal(err)
	}

	if len(algorithms) == 0 || algorithms[0] != ssh.KeyAlgoED25519 {
		t.Fatalf("host key algorithms = %v, want %q first", algorithms, ssh.KeyAlgoED25519)
	}
}

func testHostPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return hostKey
}
