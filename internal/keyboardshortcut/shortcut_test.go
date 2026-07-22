package keyboardshortcut

import "testing"

func TestCanonicalNormalizesAliasesAndModifierOrder(t *testing.T) {
	got, err := Canonical("shift+option+control+semicolon")
	if err == nil {
		t.Fatalf("expected unsupported word-form punctuation, got %q", got)
	}

	got, err = Canonical("shift+option+control+;")
	if err != nil {
		t.Fatalf("canonicalize shortcut: %v", err)
	}
	if got != "Ctrl+Alt+Shift+;" {
		t.Fatalf("canonical shortcut = %q, want %q", got, "Ctrl+Alt+Shift+;")
	}
}

func TestParseRejectsUnsafeOrAmbiguousShortcuts(t *testing.T) {
	tests := []string{
		";",
		"Shift+;",
		"Alt",
		"Alt+;+P",
		"Alt+VolumeUp",
		"Alt++",
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := Parse(value); err == nil {
				t.Fatalf("expected %q to be rejected", value)
			}
		})
	}
}

func TestParseAcceptsNamedAndFunctionKeys(t *testing.T) {
	tests := map[string]string{
		"Option+]":       "Alt+]",
		"Cmd+ArrowRight": "Meta+ArrowRight",
		"Ctrl+Alt+F12":   "Ctrl+Alt+F12",
		"Control+Space":  "Ctrl+Space",
	}
	for input, want := range tests {
		shortcut, err := Parse(input)
		if err != nil {
			t.Fatalf("parse %q: %v", input, err)
		}
		if got := shortcut.String(); got != want {
			t.Fatalf("Parse(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSameHeldModifiersIgnoresKeysAndShift(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "different keys", left: "Alt+X", right: "Alt+Z", want: true},
		{name: "shift may differ", left: "Alt+X", right: "Alt+Shift+Z", want: true},
		{name: "modifier order is irrelevant", left: "Ctrl+Alt+X", right: "Option+Control+Shift+Z", want: true},
		{name: "control and alt differ", left: "Ctrl+X", right: "Alt+Z", want: false},
		{name: "alt and meta differ", left: "Alt+X", right: "Meta+Z", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, err := Parse(tt.left)
			if err != nil {
				t.Fatalf("parse left shortcut: %v", err)
			}
			right, err := Parse(tt.right)
			if err != nil {
				t.Fatalf("parse right shortcut: %v", err)
			}
			if got := SameHeldModifiers(left, right); got != tt.want {
				t.Fatalf("SameHeldModifiers(%q, %q) = %t, want %t", tt.left, tt.right, got, tt.want)
			}
		})
	}
}
