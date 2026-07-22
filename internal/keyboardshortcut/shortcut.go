package keyboardshortcut

import (
	"fmt"
	"strings"
)

const (
	DefaultBrowserSwitcher         = "Alt+X"
	DefaultBrowserSwitcherBackward = "Alt+Z"
)

type Shortcut struct {
	Control bool
	Alt     bool
	Shift   bool
	Meta    bool
	Key     string
}

var namedKeys = map[string]string{
	"space":      "Space",
	"tab":        "Tab",
	"enter":      "Enter",
	"return":     "Enter",
	"esc":        "Escape",
	"escape":     "Escape",
	"backspace":  "Backspace",
	"delete":     "Delete",
	"arrowup":    "ArrowUp",
	"up":         "ArrowUp",
	"arrowdown":  "ArrowDown",
	"down":       "ArrowDown",
	"arrowleft":  "ArrowLeft",
	"left":       "ArrowLeft",
	"arrowright": "ArrowRight",
	"right":      "ArrowRight",
}

var punctuationKeys = map[string]bool{
	";": true, ",": true, ".": true, "/": true, "\\": true,
	"'": true, "[": true, "]": true, "-": true, "=": true, "`": true,
}

func Parse(value string) (Shortcut, error) {
	parts := strings.Split(strings.TrimSpace(value), "+")
	if len(parts) < 2 {
		return Shortcut{}, fmt.Errorf("shortcut must include a modifier and a key")
	}

	var result Shortcut
	for _, rawPart := range parts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			return Shortcut{}, fmt.Errorf("shortcut contains an empty key")
		}
		switch strings.ToLower(part) {
		case "ctrl", "control":
			if result.Control {
				return Shortcut{}, fmt.Errorf("shortcut repeats the Control modifier")
			}
			result.Control = true
		case "alt", "option":
			if result.Alt {
				return Shortcut{}, fmt.Errorf("shortcut repeats the Alt modifier")
			}
			result.Alt = true
		case "shift":
			if result.Shift {
				return Shortcut{}, fmt.Errorf("shortcut repeats the Shift modifier")
			}
			result.Shift = true
		case "meta", "cmd", "command":
			if result.Meta {
				return Shortcut{}, fmt.Errorf("shortcut repeats the Meta modifier")
			}
			result.Meta = true
		default:
			if result.Key != "" {
				return Shortcut{}, fmt.Errorf("shortcut must contain exactly one non-modifier key")
			}
			key, err := normalizeKey(part)
			if err != nil {
				return Shortcut{}, err
			}
			result.Key = key
		}
	}

	if result.Key == "" {
		return Shortcut{}, fmt.Errorf("shortcut must include a non-modifier key")
	}
	if !result.Control && !result.Alt && !result.Meta {
		return Shortcut{}, fmt.Errorf("shortcut must include Control, Alt, or Meta")
	}
	return result, nil
}

func Canonical(value string) (string, error) {
	shortcut, err := Parse(value)
	if err != nil {
		return "", err
	}
	return shortcut.String(), nil
}

// SameHeldModifiers reports whether two shortcuts use the same modifiers that
// remain held while cycling. Shift is intentionally excluded so it can alter
// direction without ending the switcher session.
func SameHeldModifiers(left, right Shortcut) bool {
	return left.Control == right.Control &&
		left.Alt == right.Alt &&
		left.Meta == right.Meta
}

func (s Shortcut) String() string {
	parts := make([]string, 0, 5)
	if s.Control {
		parts = append(parts, "Ctrl")
	}
	if s.Alt {
		parts = append(parts, "Alt")
	}
	if s.Shift {
		parts = append(parts, "Shift")
	}
	if s.Meta {
		parts = append(parts, "Meta")
	}
	return strings.Join(append(parts, s.Key), "+")
}

func normalizeKey(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if key, ok := namedKeys[strings.ToLower(trimmed)]; ok {
		return key, nil
	}
	upper := strings.ToUpper(trimmed)
	if len(upper) == 1 && ((upper[0] >= 'A' && upper[0] <= 'Z') || (upper[0] >= '0' && upper[0] <= '9')) {
		return upper, nil
	}
	if punctuationKeys[trimmed] {
		return trimmed, nil
	}
	if len(upper) >= 2 && upper[0] == 'F' {
		var number int
		if _, err := fmt.Sscanf(upper, "F%d", &number); err == nil && number >= 1 && number <= 20 {
			return fmt.Sprintf("F%d", number), nil
		}
	}

	return "", fmt.Errorf("unsupported shortcut key %q", value)
}
