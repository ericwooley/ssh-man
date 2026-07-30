//go:build !windows

package urlrouting

import (
	"testing"
)

func TestCommandTemplatePassesMetacharactersAsLiteralURLData(t *testing.T) {
	rawURL := `https://example.com/a path?q='";touch /tmp/ssh-man-should-not-exist;$HOME&x=1`
	arguments, err := commandTemplateArguments(`open <URL>`, rawURL)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	if len(arguments) != 2 {
		t.Fatalf("arguments = %#v, want executable and URL", arguments)
	}
	if arguments[0] != "/usr/bin/open" || arguments[1] != rawURL {
		t.Fatalf("arguments = %#v, want URL preserved as argv data", arguments)
	}
}
