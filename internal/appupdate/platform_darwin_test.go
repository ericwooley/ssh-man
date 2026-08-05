//go:build darwin

package appupdate

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestBundlePathFromExecutable(t *testing.T) {
	got, err := bundlePathFromExecutable("/Applications/SSH Man.app/Contents/MacOS/ssh-man")
	if err != nil {
		t.Fatalf("bundle path: %v", err)
	}
	if got != "/Applications/SSH Man.app" {
		t.Fatalf("bundle path = %q", got)
	}

	if _, err := bundlePathFromExecutable("/usr/local/bin/ssh-man"); err == nil {
		t.Fatal("non-bundle executable should be rejected")
	}
}

func TestRunApplyHelperRejectsBroadStagingRoot(t *testing.T) {
	err := runApplyHelper([]string{
		"--parent-pid", strconv.Itoa(os.Getpid()),
		"--current-app", "/Applications/ssh-man.app",
		"--staged-app", "/ssh-man.app",
		"--version", "1.2.3",
		"--root", "/",
	})
	if err == nil || !strings.Contains(err.Error(), "staging layout") {
		t.Fatalf("helper error = %v, want staging layout rejection", err)
	}
}

func TestReopenApplicationUsesMacOSOpen(t *testing.T) {
	var command string
	var arguments []string
	err := reopenApplication("/Applications/ssh-man.app", func(_ context.Context, name string, args ...string) ([]byte, error) {
		command = name
		arguments = append([]string{}, args...)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("reopen application: %v", err)
	}
	if command != "/usr/bin/open" {
		t.Fatalf("command = %q, want /usr/bin/open", command)
	}
	if !reflect.DeepEqual(arguments, []string{"/Applications/ssh-man.app"}) {
		t.Fatalf("arguments = %#v", arguments)
	}
}
