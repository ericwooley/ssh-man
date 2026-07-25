package bindings

import "testing"

func TestPreviewLauncherValidatesAndNormalizesRequest(t *testing.T) {
	gotServerID := ""
	gotPath := ""
	focusedPath := ""
	binding := NewPreviewLauncherBindingsWithDependencies(
		" server-1 ",
		func(serverID, remotePath string) error {
			gotServerID = serverID
			gotPath = remotePath
			return nil
		},
		func(remotePath string) error {
			focusedPath = remotePath
			return nil
		},
		func(remotePath string) bool {
			return remotePath == "/var/www/index.html"
		},
	)

	state, err := binding.Open("/var/www/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if gotServerID != "server-1" || gotPath != "/var/www/index.html" {
		t.Fatalf("launched preview = %q %q", gotServerID, gotPath)
	}
	if !state.Open || state.RemotePath != "/var/www/index.html" {
		t.Fatalf("open state = %#v", state)
	}
	if state := binding.State("/var/www/index.html"); !state.Open {
		t.Fatalf("queried state = %#v", state)
	}
	if _, err := binding.Focus("/var/www/index.html"); err != nil {
		t.Fatal(err)
	}
	if focusedPath != "/var/www/index.html" {
		t.Fatalf("focused preview = %q", focusedPath)
	}
}

func TestPreviewLauncherRejectsBlankPath(t *testing.T) {
	launched := false
	binding := NewPreviewLauncherBindingsWithDependencies(
		"server-1",
		func(string, string) error {
			launched = true
			return nil
		},
		nil,
		nil,
	)

	_, err := binding.Open(" ")

	if err == nil || launched {
		t.Fatalf("Open() error = %v, launched = %v", err, launched)
	}
}
