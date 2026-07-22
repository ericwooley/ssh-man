//go:build darwin && cgo

package menubar

import (
	"strings"
	"testing"
)

func TestParseDarwinBrowserShortcutsMapsForwardAndBackward(t *testing.T) {
	forward, backward, err := parseDarwinBrowserShortcuts("Alt+X", "Alt+Z")
	if err != nil {
		t.Fatalf("parse browser shortcuts: %v", err)
	}

	if forward.canonical != "Alt+X" || forward.keyCode != 7 || forward.modifiers != carbonOptionKey {
		t.Fatalf("forward shortcut = %#v, want Alt+X key code 7 with Option", forward)
	}
	if backward.canonical != "Alt+Z" || backward.keyCode != 6 || backward.modifiers != carbonOptionKey {
		t.Fatalf("backward shortcut = %#v, want Alt+Z key code 6 with Option", backward)
	}
}

func TestParseDarwinBrowserShortcutsRejectsDuplicateCanonicalShortcut(t *testing.T) {
	_, _, err := parseDarwinBrowserShortcuts("Alt+X", "option+x")
	if err == nil || !strings.Contains(err.Error(), "must be different") {
		t.Fatalf("duplicate shortcut error = %v, want shortcuts-must-differ error", err)
	}
}

func TestParseDarwinBrowserShortcutsAllowsShiftToDiffer(t *testing.T) {
	forward, backward, err := parseDarwinBrowserShortcuts("Alt+X", "Alt+Shift+Z")
	if err != nil {
		t.Fatalf("parse browser shortcuts: %v", err)
	}
	if forward.modifiers != carbonOptionKey {
		t.Fatalf("forward modifiers = %d, want Option", forward.modifiers)
	}
	if backward.modifiers != carbonOptionKey|carbonShiftKey {
		t.Fatalf("backward modifiers = %d, want Option+Shift", backward.modifiers)
	}
}

func TestParseDarwinBrowserShortcutsRejectsDifferentHeldModifiers(t *testing.T) {
	_, _, err := parseDarwinBrowserShortcuts("Ctrl+X", "Alt+Z")
	if err == nil || !strings.Contains(err.Error(), "same Control, Alt, and Command modifiers") {
		t.Fatalf("modifier mismatch error = %v, want shared-held-modifiers error", err)
	}
}

func TestParseDarwinBrowserShortcutsIdentifiesInvalidDirection(t *testing.T) {
	_, _, err := parseDarwinBrowserShortcuts("Alt+X", "Shift+Z")
	if err == nil || !strings.Contains(err.Error(), "backward browser shortcut") {
		t.Fatalf("invalid shortcut error = %v, want backward shortcut context", err)
	}
}

func TestOutsideClickDismissalRequiresInactiveVisiblePopup(t *testing.T) {
	tests := []struct {
		name              string
		applicationActive bool
		popupVisible      bool
		want              bool
	}{
		{
			name:              "stale outside click after popup regains focus",
			applicationActive: true,
			popupVisible:      true,
			want:              false,
		},
		{
			name:              "outside click while popup is inactive",
			applicationActive: false,
			popupVisible:      true,
			want:              true,
		},
		{
			name:              "outside click while popup is hidden",
			applicationActive: false,
			popupVisible:      false,
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDismissPopupForOutsideClick(tt.applicationActive, tt.popupVisible); got != tt.want {
				t.Fatalf("shouldDismissPopupForOutsideClick(%t, %t) = %t, want %t", tt.applicationActive, tt.popupVisible, got, tt.want)
			}
		})
	}
}

func TestBrowserShortcutCallbacksRouteDirectionAndCommit(t *testing.T) {
	type request struct {
		direction BrowserSwitchDirection
		sessionID uint64
	}
	var requests []request
	var commits []uint64
	var cancels []uint64

	callbackMu.Lock()
	previousSwitchHandler := browserSwitchHandler
	previousCommitHandler := browserSwitchCommitHandler
	previousCancelHandler := browserSwitchCancelHandler
	browserSwitchHandler = func(direction BrowserSwitchDirection, sessionID uint64) {
		requests = append(requests, request{direction: direction, sessionID: sessionID})
	}
	browserSwitchCommitHandler = func(sessionID uint64) {
		commits = append(commits, sessionID)
	}
	browserSwitchCancelHandler = func(sessionID uint64) {
		cancels = append(cancels, sessionID)
	}
	callbackMu.Unlock()
	t.Cleanup(func() {
		callbackMu.Lock()
		browserSwitchHandler = previousSwitchHandler
		browserSwitchCommitHandler = previousCommitHandler
		browserSwitchCancelHandler = previousCancelHandler
		callbackMu.Unlock()
	})

	SSHManBrowserShortcutRequested(1, 41)
	SSHManBrowserShortcutRequested(2, 41)
	SSHManBrowserShortcutRequested(99, 42)
	SSHManBrowserShortcutCommitted(41)
	SSHManBrowserShortcutCanceled(42)

	want := []request{
		{direction: BrowserSwitchForward, sessionID: 41},
		{direction: BrowserSwitchBackward, sessionID: 41},
	}
	if len(requests) != len(want) || requests[0] != want[0] || requests[1] != want[1] {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
	if len(commits) != 1 || commits[0] != 41 {
		t.Fatalf("commit callbacks = %#v, want session 41", commits)
	}
	if len(cancels) != 1 || cancels[0] != 42 {
		t.Fatalf("cancel callbacks = %#v, want session 42", cancels)
	}
}
