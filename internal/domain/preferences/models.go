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

	"ssh-man/internal/domain/browsercommand"
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
	maxBrowserCommandBytes       = 8192
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
	Command         string        `json:"command,omitempty"`
	Icon            string        `json:"icon,omitempty"`
	LaunchReference string        `json:"launchReference,omitempty"`
	Engine          BrowserEngine `json:"engine,omitempty"`
}

type URLRuleMatchMode string

const (
	URLRuleMatchRegex      URLRuleMatchMode = "regex"
	URLRuleMatchStartsWith URLRuleMatchMode = "starts_with"
	URLRuleMatchEndsWith   URLRuleMatchMode = "ends_with"
	URLRuleMatchContains   URLRuleMatchMode = "contains"
)

type URLRuleAction string

const (
	URLRuleActionBrowser URLRuleAction = "browser"
	URLRuleActionCommand URLRuleAction = "command"
)

type URLRule struct {
	ID         string           `json:"id"`
	MatchMode  URLRuleMatchMode `json:"matchMode"`
	Pattern    string           `json:"pattern"`
	Action     URLRuleAction    `json:"action"`
	BrowserID  string           `json:"browserId,omitempty"`
	Command    string           `json:"command,omitempty"`
	OpenDirect bool             `json:"openDirect,omitempty"`
}

type URLPortAssignment struct {
	ID        string `json:"id"`
	Port      int    `json:"port"`
	ServerID  string `json:"serverId"`
	BrowserID string `json:"browserId"`
}

type UserPreference struct {
	Theme                           Theme                        `json:"theme"`
	LastSelectedServerID            string                       `json:"lastSelectedServerId,omitempty"`
	BrowserSwitcherShortcut         string                       `json:"browserSwitcherShortcut"`
	BrowserSwitcherBackwardShortcut string                       `json:"browserSwitcherBackwardShortcut"`
	BrowserAppearances              map[string]BrowserAppearance `json:"browserAppearances"`
	DefaultBrowserID                string                       `json:"defaultBrowserId,omitempty"`
	ProxyBrowserID                  string                       `json:"proxyBrowserId,omitempty"`
	DisabledBrowserIDs              []string                     `json:"disabledBrowserIds"`
	CustomBrowsers                  []CustomBrowser              `json:"customBrowsers"`
	URLRules                        []URLRule                    `json:"urlRules"`
	URLPortAssignments              []URLPortAssignment          `json:"urlPortAssignments"`
	UpdatedAt                       time.Time                    `json:"updatedAt"`
}

func Default() UserPreference {
	return UserPreference{
		Theme:                           ThemeDark,
		BrowserSwitcherShortcut:         keyboardshortcut.DefaultBrowserSwitcher,
		BrowserSwitcherBackwardShortcut: keyboardshortcut.DefaultBrowserSwitcherBackward,
		BrowserAppearances:              map[string]BrowserAppearance{},
		DisabledBrowserIDs:              []string{},
		CustomBrowsers:                  []CustomBrowser{},
		URLRules:                        []URLRule{},
		URLPortAssignments:              []URLPortAssignment{},
	}
}

func (p UserPreference) Validate() error {
	return p.validate(false)
}

// ValidateAllowingLegacyCommandSyntax validates structurally repairable
// preferences. Save and execution boundaries separately reject new or changed
// legacy command syntax.
func (p UserPreference) ValidateAllowingLegacyCommandSyntax() error {
	return p.validate(true)
}

func (p UserPreference) validate(allowLegacyCommandSyntax bool) error {
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
	disabledBrowserIDs := make(map[string]struct{}, len(p.DisabledBrowserIDs))
	for index, browserID := range p.DisabledBrowserIDs {
		if err := validateBrowserID(fmt.Sprintf("disabled browser %d", index+1), browserID, true); err != nil {
			return err
		}
		if _, exists := disabledBrowserIDs[browserID]; exists {
			return fmt.Errorf("disabled browser id %q is duplicated", browserID)
		}
		disabledBrowserIDs[browserID] = struct{}{}
	}
	customBrowserIDs := make(map[string]struct{}, len(p.CustomBrowsers))
	customBrowserLaunches := make(map[string]struct{}, len(p.CustomBrowsers))
	for index, customBrowser := range p.CustomBrowsers {
		if err := customBrowser.validate(allowLegacyCommandSyntax); err != nil {
			return fmt.Errorf("custom browser %d: %w", index+1, err)
		}
		if _, exists := customBrowserIDs[customBrowser.ID]; exists {
			return fmt.Errorf("custom browser duplicate id %q", customBrowser.ID)
		}
		customBrowserIDs[customBrowser.ID] = struct{}{}
		launchKey := customBrowser.Command
		launchLabel := "command"
		if launchKey == "" {
			launchKey = customBrowser.LaunchReference
			launchLabel = "application path"
		}
		if _, exists := customBrowserLaunches[launchKey]; exists {
			return fmt.Errorf("custom browser duplicate %s %q", launchLabel, launchKey)
		}
		customBrowserLaunches[launchKey] = struct{}{}
	}
	ruleIDs := make(map[string]struct{}, len(p.URLRules))
	for index, rule := range p.URLRules {
		if err := rule.validate(allowLegacyCommandSyntax); err != nil {
			return fmt.Errorf("URL rule %d: %w", index+1, err)
		}
		if _, exists := ruleIDs[rule.ID]; exists {
			return fmt.Errorf("URL rule id %q is duplicated", rule.ID)
		}
		ruleIDs[rule.ID] = struct{}{}
	}
	assignmentIDs := make(map[string]struct{}, len(p.URLPortAssignments))
	assignmentPorts := make(map[int]struct{}, len(p.URLPortAssignments))
	for index, assignment := range p.URLPortAssignments {
		if err := assignment.Validate(); err != nil {
			return fmt.Errorf("URL port assignment %d: %w", index+1, err)
		}
		if _, exists := assignmentIDs[assignment.ID]; exists {
			return fmt.Errorf("URL port assignment id %q is duplicated", assignment.ID)
		}
		assignmentIDs[assignment.ID] = struct{}{}
		if _, exists := assignmentPorts[assignment.Port]; exists {
			return fmt.Errorf("URL port %d is assigned more than once", assignment.Port)
		}
		assignmentPorts[assignment.Port] = struct{}{}
	}
	return nil
}

func (b CustomBrowser) Validate() error {
	return b.validate(false)
}

func (b CustomBrowser) validate(allowLegacyCommandSyntax bool) error {
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
	if err := (BrowserAppearance{Icon: b.Icon}).Validate(); err != nil {
		return fmt.Errorf("icon: %w", err)
	}
	if b.Command != "" {
		if len(b.Command) > maxBrowserCommandBytes {
			return fmt.Errorf("command must be at most %d bytes", maxBrowserCommandBytes)
		}
		if !strings.Contains(b.Command, "<URL>") {
			return fmt.Errorf("command must contain <URL>")
		}
		if !allowLegacyCommandSyntax {
			if err := browsercommand.Validate(b.Command); err != nil {
				return fmt.Errorf("command: %w", err)
			}
		}
		if b.LaunchReference != "" || b.Engine != "" {
			return fmt.Errorf("command browser must not include legacy application settings")
		}
		return nil
	}
	if b.LaunchReference == "" {
		return fmt.Errorf("command is required")
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
	return r.validate(false)
}

func (r URLRule) validate(allowLegacyCommandSyntax bool) error {
	if r.ID == "" {
		return fmt.Errorf("id is required")
	}
	if len(r.ID) > maxURLRuleIDBytes {
		return fmt.Errorf("id must be at most %d bytes", maxURLRuleIDBytes)
	}
	if !browserAppearanceKeyPattern.MatchString(r.ID) {
		return fmt.Errorf("id contains unsupported characters")
	}
	if err := ValidateURLRulePattern(r.MatchMode, r.Pattern); err != nil {
		return err
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
		if !allowLegacyCommandSyntax {
			if err := browsercommand.Validate(r.Command); err != nil {
				return fmt.Errorf("command: %w", err)
			}
		}
	default:
		return fmt.Errorf("action must be browser or command")
	}
	return nil
}

func ValidateURLRulePattern(matchMode URLRuleMatchMode, pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	if len(pattern) > maxURLRulePatternBytes {
		return fmt.Errorf("pattern must be at most %d bytes", maxURLRulePatternBytes)
	}
	switch matchMode {
	case "", URLRuleMatchRegex:
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("pattern must be a valid regular expression: %w", err)
		}
	case URLRuleMatchStartsWith, URLRuleMatchEndsWith, URLRuleMatchContains:
	default:
		return fmt.Errorf("match mode must be starts_with, ends_with, contains, or regex")
	}
	return nil
}

func (a URLPortAssignment) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("id is required")
	}
	if len(a.ID) > maxURLRuleIDBytes {
		return fmt.Errorf("id must be at most %d bytes", maxURLRuleIDBytes)
	}
	if !browserAppearanceKeyPattern.MatchString(a.ID) {
		return fmt.Errorf("id contains unsupported characters")
	}
	if a.Port < 1 || a.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if err := validateBrowserID("server", a.ServerID, true); err != nil {
		return err
	}
	if err := validateBrowserID("browser", a.BrowserID, true); err != nil {
		return err
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
