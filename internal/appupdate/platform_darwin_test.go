//go:build darwin

package appupdate

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestRelaunchIfRequestedUsesMacOSOpenOnlyForOneClickUpdate(t *testing.T) {
	var command string
	var arguments []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		command = name
		arguments = append([]string{}, args...)
		return nil, nil
	}

	err := relaunchIfRequested(false, "/Applications/ssh-man.app", run)
	if err != nil {
		t.Fatalf("skip relaunch: %v", err)
	}
	if command != "" || arguments != nil {
		t.Fatalf("normal quit command = %q %#v, want no relaunch", command, arguments)
	}

	err = relaunchIfRequested(true, "/Applications/ssh-man.app", run)
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

func TestUpdateHelperCarriesRelaunchIntentAcrossProcessBoundary(t *testing.T) {
	for _, requested := range []bool{false, true} {
		t.Run(strconv.FormatBool(requested), func(t *testing.T) {
			staged := &stagedUpdate{
				Version:        "1.2.3",
				AppPath:        "/tmp/updates/1.2.3-test/ssh-man.app",
				RootPath:       "/tmp/updates/1.2.3-test",
				CurrentAppPath: "/Applications/ssh-man.app",
			}
			arguments := updateHelperArguments(staged, 123, requested)
			hasRelaunchFlag := false
			for _, argument := range arguments {
				if argument == "--relaunch" {
					hasRelaunchFlag = true
				}
			}
			if hasRelaunchFlag != requested {
				t.Fatalf("relaunch flag present = %t, want %t", hasRelaunchFlag, requested)
			}

			var appliedRelaunch bool
			err := runApplyHelperWithDependencies(
				arguments[1:],
				func(parentPID int, timeout time.Duration) error {
					if parentPID != 123 || timeout != 5*time.Minute {
						t.Fatalf("wait input = (%d, %s)", parentPID, timeout)
					}
					return nil
				},
				func(_, _, _, _ string, _ int, relaunch bool) error {
					appliedRelaunch = relaunch
					return nil
				},
			)
			if err != nil {
				t.Fatalf("execute helper: %v", err)
			}
			if appliedRelaunch != requested {
				t.Fatalf("applied relaunch = %t, want %t", appliedRelaunch, requested)
			}
		})
	}
}
