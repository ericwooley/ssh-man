package connection

import (
	"net"
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

func TestResolveEndpointAppliesOpenSSHAliasSettings(t *testing.T) {
	server := serverdomain.Server{
		Host: "dev",
		Port: 22,
	}
	settings := OpenSSHSettings{
		Hostname:     "192.0.2.10",
		Port:         2202,
		HostKeyAlias: "dev-host-key",
	}

	got, err := resolveEndpoint(server, settings)
	if err != nil {
		t.Fatal(err)
	}

	if got.DialAddress != net.JoinHostPort("192.0.2.10", "2202") {
		t.Fatalf("DialAddress = %q", got.DialAddress)
	}
	if got.HostKeyAddress != net.JoinHostPort("dev-host-key", "2202") {
		t.Fatalf("HostKeyAddress = %q", got.HostKeyAddress)
	}
}

func TestResolveEndpointKeepsAnExplicitSavedPort(t *testing.T) {
	server := serverdomain.Server{
		Host: "dev",
		Port: 2022,
	}
	settings := OpenSSHSettings{
		Hostname: "192.0.2.10",
		Port:     2202,
	}

	got, err := resolveEndpoint(server, settings)
	if err != nil {
		t.Fatal(err)
	}

	if got.DialAddress != net.JoinHostPort("192.0.2.10", "2022") {
		t.Fatalf("DialAddress = %q", got.DialAddress)
	}
	if got.HostKeyAddress != net.JoinHostPort("192.0.2.10", "2022") {
		t.Fatalf("HostKeyAddress = %q", got.HostKeyAddress)
	}
}

func TestParseOpenSSHSettings(t *testing.T) {
	settings, err := parseOpenSSHSettings([]byte(`
user deploy
hostname 192.0.2.10
port 2202
hostkeyalias dev-host-key
identityfile ~/.ssh/id_ed25519
`))
	if err != nil {
		t.Fatal(err)
	}

	if settings.Hostname != "192.0.2.10" {
		t.Fatalf("Hostname = %q", settings.Hostname)
	}
	if settings.Port != 2202 {
		t.Fatalf("Port = %d", settings.Port)
	}
	if settings.HostKeyAlias != "dev-host-key" {
		t.Fatalf("HostKeyAlias = %q", settings.HostKeyAlias)
	}
}

func TestParseOpenSSHSettingsRejectsInvalidPort(t *testing.T) {
	if _, err := parseOpenSSHSettings([]byte("hostname example.test\nport nope\n")); err == nil {
		t.Fatal("invalid OpenSSH port unexpectedly accepted")
	}
}
