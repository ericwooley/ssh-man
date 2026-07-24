package preferences

import (
	"fmt"
	"path/filepath"
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
	maxBrowserIDBytes            = 128
	maxBrowserDisplayNameBytes   = 256
	maxBrowserLaunchPathBytes    = 4096
	maxURLRuleIDBytes            = 128
	maxURLRulePatternBytes       = 4096
	maxURLRuleCommandBytes       = 8192
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

type BrowserEngine string

const (
	BrowserEngineChromium BrowserEngine = "chromium"
	BrowserEngineFirefox  BrowserEngine = "firefox"
	BrowserEngineRegular  BrowserEngine = "regular"
)

type CustomBrowser struct {
	ID              string        `json:"id"`
	DisplayName     string        `json:"displayName"`
	LaunchReference string        `json:"launchReference"`
	Engine          BrowserEngine `json:"engine"`
}

type URLRuleAction string

const (
	URLRuleActionBrowser URLRuleAction = "browser"
	URLRuleActionCommand URLRuleAction = "command"
)

type URLRule struct {
	ID        string        `json:"id"`
	Pattern   string        `json:"pattern"`
	Action    URLRuleAction `json:"action"`
	BrowserID string        `json:"browserId,omitempty"`
	Command   string        `json:"command,omitempty"`
}

type UserPreference struct {
	Theme                           Theme                        `json:"theme"`
	LastSelectedServerID            string                       `json:"lastSelectedServerId,omitempty"`
	BrowserSwitcherShortcut         string                       `json:"browserSwitcherShortcut"`
	BrowserSwitcherBackwardShortcut string                       `json:"browserSwitcherBackwardShortcut"`
	BrowserAppearances              map[string]BrowserAppearance `json:"browserAppearances"`
	DefaultBrowserID                string                       `json:"defaultBrowserId,omitempty"`
	ProxyBrowserID                  string                       `json:"proxyBrowserId,omitempty"`
	CustomBrowsers                  []CustomBrowser              `json:"customBrowsers"`
	URLRules                        []URLRule                    `json:"urlRules"`
	UpdatedAt                       time.Time                    `json:"updatedAt"`
}

func Default() UserPreference {
	return UserPreference{
		Theme:                           ThemeDark,
		BrowserSwitcherShortcut:         keyboardshortcut.DefaultBrowserSwitcher,
		BrowserSwitcherBackwardShortcut: keyboardshortcut.DefaultBrowserSwitcherBackward,
		BrowserAppearances:              map[string]BrowserAppearance{},
		CustomBrowsers:                  []CustomBrowser{},
		URLRules:                        []URLRule{},
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
	if err := validateBrowserID("default browser", p.DefaultBrowserID, false); err != nil {
		return err
	}
	if err := validateBrowserID("proxy browser", p.ProxyBrowserID, false); err != nil {
		return err
	}
	customBrowserIDs := make(map[string]struct{}, len(p.CustomBrowsers))
	customBrowserPaths := make(map[string]struct{}, len(p.CustomBrowsers))
	for index, customBrowser := range p.CustomBrowsers {
		if err := customBrowser.Validate(); err != nil {
			return fmt.Errorf("custom browser %d: %w", index+1, err)
		}
		if _, exists := customBrowserIDs[customBrowser.ID]; exists {
			return fmt.Errorf("custom browser duplicate id %q", customBrowser.ID)
		}
		customBrowserIDs[customBrowser.ID] = struct{}{}
		if _, exists := customBrowserPaths[customBrowser.LaunchReference]; exists {
			return fmt.Errorf("custom browser duplicate application path %q", customBrowser.LaunchReference)
		}
		customBrowserPaths[customBrowser.LaunchReference] = struct{}{}
	}
	ruleIDs := make(map[string]struct{}, len(p.URLRules))
	for index, rule := range p.URLRules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("URL rule %d: %w", index+1, err)
		}
		if _, exists := ruleIDs[rule.ID]; exists {
			return fmt.Errorf("URL rule id %q is duplicated", rule.ID)
		}
		ruleIDs[rule.ID] = struct{}{}
	}
	return nil
}

func (b CustomBrowser) Validate() error {
	if err := validateBrowserID("browser", b.ID, true); err != nil {
		return err
	}
	if b.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}
	if len(b.DisplayName) > maxBrowserDisplayNameBytes {
		return fmt.Errorf("display name must be at most %d bytes", maxBrowserDisplayNameBytes)
	}
	if !utf8.ValidString(b.DisplayName) {
		return fmt.Errorf("display name must be valid UTF-8")
	}
	for _, value := range b.DisplayName {
		if unicode.IsControl(value) {
			return fmt.Errorf("display name must not contain control characters")
		}
	}
	if b.LaunchReference == "" {
		return fmt.Errorf("application path is required")
	}
	if len(b.LaunchReference) > maxBrowserLaunchPathBytes {
		return fmt.Errorf("application path must be at most %d bytes", maxBrowserLaunchPathBytes)
	}
	if !filepath.IsAbs(b.LaunchReference) {
		return fmt.Errorf("application path must be an absolute path")
	}
	switch b.Engine {
	case BrowserEngineChromium, BrowserEngineFirefox, BrowserEngineRegular:
	default:
		return fmt.Errorf("engine must be chromium, firefox, or regular")
	}
	return nil
}

func (r URLRule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("id is required")
	}
	if len(r.ID) > maxURLRuleIDBytes {
		return fmt.Errorf("id must be at most %d bytes", maxURLRuleIDBytes)
	}
	if !browserAppearanceKeyPattern.MatchString(r.ID) {
		return fmt.Errorf("id contains unsupported characters")
	}
	if r.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	if len(r.Pattern) > maxURLRulePatternBytes {
		return fmt.Errorf("pattern must be at most %d bytes", maxURLRulePatternBytes)
	}
	if _, err := regexp.Compile(r.Pattern); err != nil {
		return fmt.Errorf("pattern must be a valid regular expression: %w", err)
	}
	switch r.Action {
	case URLRuleActionBrowser:
		if err := validateBrowserID("browser", r.BrowserID, true); err != nil {
			return err
		}
		if r.Command != "" {
			return fmt.Errorf("browser rule must not include a command")
		}
	case URLRuleActionCommand:
		if r.BrowserID != "" {
			return fmt.Errorf("command rule must not include a browser")
		}
		if r.Command == "" {
			return fmt.Errorf("command is required")
		}
		if len(r.Command) > maxURLRuleCommandBytes {
			return fmt.Errorf("command must be at most %d bytes", maxURLRuleCommandBytes)
		}
		if !strings.Contains(r.Command, "<URL>") {
			return fmt.Errorf("command must contain <URL>")
		}
	default:
		return fmt.Errorf("action must be browser or command")
	}
	return nil
}

func validateBrowserID(label, browserID string, required bool) error {
	if browserID == "" {
		if required {
			return fmt.Errorf("%s id is required", label)
		}
		return nil
	}
	if len(browserID) > maxBrowserIDBytes {
		return fmt.Errorf("%s id must be at most %d bytes", label, maxBrowserIDBytes)
	}
	if !browserAppearanceKeyPattern.MatchString(browserID) {
		return fmt.Errorf("%s id contains unsupported characters", label)
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
