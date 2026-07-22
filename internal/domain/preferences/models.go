package preferences

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"ssh-man/internal/keyboardshortcut"
)

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"

	maxBrowserAppearanceKeyBytes = 256
	maxBrowserIconBytes          = 64
	maxBrowserIconGraphemes      = 2
)

var (
	browserAppearanceKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	browserColorPattern         = regexp.MustCompile(`(?i)^#[0-9A-F]{6}$`)
	allowedBrowserIconTokens    = map[string]struct{}{
		"icon:x":         {},
		"icon:shield":    {},
		"icon:terminal":  {},
		"icon:globe":     {},
		"icon:network":   {},
		"icon:star":      {},
		"icon:briefcase": {},
		"icon:code":      {},
	}
)

type BrowserAppearance struct {
	Icon         string `json:"icon,omitempty"`
	PrimaryColor string `json:"primaryColor,omitempty"`
}

type UserPreference struct {
	Theme                           Theme                        `json:"theme"`
	LastSelectedServerID            string                       `json:"lastSelectedServerId,omitempty"`
	BrowserSwitcherShortcut         string                       `json:"browserSwitcherShortcut"`
	BrowserSwitcherBackwardShortcut string                       `json:"browserSwitcherBackwardShortcut"`
	BrowserAppearances              map[string]BrowserAppearance `json:"browserAppearances"`
	UpdatedAt                       time.Time                    `json:"updatedAt"`
}

func Default() UserPreference {
	return UserPreference{
		Theme:                           ThemeDark,
		BrowserSwitcherShortcut:         keyboardshortcut.DefaultBrowserSwitcher,
		BrowserSwitcherBackwardShortcut: keyboardshortcut.DefaultBrowserSwitcherBackward,
		BrowserAppearances:              map[string]BrowserAppearance{},
		UpdatedAt:                       time.Now().UTC(),
	}
}

func (p UserPreference) Validate() error {
	if p.Theme != ThemeLight && p.Theme != ThemeDark {
		return fmt.Errorf("theme must be light or dark")
	}
	forward, err := keyboardshortcut.Parse(p.BrowserSwitcherShortcut)
	if err != nil {
		return fmt.Errorf("browser switcher shortcut: %w", err)
	}
	backward, err := keyboardshortcut.Parse(p.BrowserSwitcherBackwardShortcut)
	if err != nil {
		return fmt.Errorf("browser switcher backward shortcut: %w", err)
	}
	if forward.String() == backward.String() {
		return fmt.Errorf("browser switcher shortcuts must be different")
	}
	if !keyboardshortcut.SameHeldModifiers(forward, backward) {
		return fmt.Errorf("browser switcher shortcuts must use the same Control, Alt, and Command modifiers")
	}
	for key, appearance := range p.BrowserAppearances {
		if err := validateBrowserAppearanceKey(key); err != nil {
			return err
		}
		if err := appearance.Validate(); err != nil {
			return fmt.Errorf("browser appearance %q: %w", key, err)
		}
	}
	return nil
}

func (a BrowserAppearance) Validate() error {
	if a.PrimaryColor != "" && !browserColorPattern.MatchString(a.PrimaryColor) {
		return fmt.Errorf("primary color must use #RRGGBB format")
	}
	if a.Icon == "" {
		return nil
	}
	if strings.HasPrefix(a.Icon, "icon:") {
		if _, ok := allowedBrowserIconTokens[a.Icon]; !ok {
			return fmt.Errorf("icon token is not supported")
		}
		return nil
	}
	if !utf8.ValidString(a.Icon) {
		return fmt.Errorf("custom mark must be valid UTF-8")
	}
	if strings.ContainsAny(a.Icon, "<>") {
		return fmt.Errorf("custom mark must not contain markup characters")
	}
	for _, value := range a.Icon {
		if unicode.IsControl(value) {
			return fmt.Errorf("custom mark must not contain control characters")
		}
	}
	if len(a.Icon) > maxBrowserIconBytes {
		return fmt.Errorf("custom mark must be at most %d bytes", maxBrowserIconBytes)
	}
	if uniseg.GraphemeClusterCount(a.Icon) > maxBrowserIconGraphemes {
		return fmt.Errorf("custom mark must be at most %d characters", maxBrowserIconGraphemes)
	}
	return nil
}

func validateBrowserAppearanceKey(key string) error {
	if key == "" {
		return fmt.Errorf("browser appearance key is required")
	}
	if len(key) > maxBrowserAppearanceKeyBytes {
		return fmt.Errorf("browser appearance key must be at most %d characters", maxBrowserAppearanceKeyBytes)
	}
	if !browserAppearanceKeyPattern.MatchString(key) {
		return fmt.Errorf("browser appearance key %q contains unsupported characters", key)
	}
	return nil
}
