//go:build !windows

package urlrouting

import (
	"os/exec"
	"testing"
)

func TestExpandedCommandPassesMetacharactersAsLiteralURLData(t *testing.T) {
	rawURL := `https://example.com/a path?q='";touch /tmp/ssh-man-should-not-exist;$HOME&x=1`
	expanded, err := expandCommandTemplate(`printf '%s' <URL>`, rawURL)
	if err != nil {
		t.Fatalf("expand template: %v", err)
	}
	output, err := exec.Command("/bin/sh", "-c", expanded).Output()
	if err != nil {
		t.Fatalf("execute expanded command: %v", err)
	}
	if string(output) != rawURL {
		t.Fatalf("command output = %q, want literal URL %q", output, rawURL)
	}
}
