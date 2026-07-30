package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"ssh-man/internal/domain/preferences"
)

type PreferencesStore struct {
	db *sql.DB
}

func NewPreferencesStore(db *sql.DB) *PreferencesStore {
	return &PreferencesStore{db: db}
}

func (s *PreferencesStore) Load(ctx context.Context) (preferences.UserPreference, error) {
	var pref preferences.UserPreference
	var browserAppearancesJSON string
	var customBrowsersJSON string
	var commandBrowsersJSON sql.NullString
	var urlRulesJSON string
	var browserRoutingRulesJSON sql.NullString
	var urlPortAssignmentsJSON string
	var disabledBrowserIDsJSON string
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT theme, last_selected_server_id, browser_switcher_shortcut,
		       browser_switcher_backward_shortcut, browser_appearances_json,
		       default_browser_id, proxy_browser_id, custom_browsers_json,
		       command_browsers_json, url_rules_json, browser_routing_rules_json,
		       url_port_assignments_json, disabled_browser_ids_json, updated_at
		FROM user_preferences
		WHERE id = 1
	`).Scan(
		&pref.Theme,
		&pref.LastSelectedServerID,
		&pref.BrowserSwitcherShortcut,
		&pref.BrowserSwitcherBackwardShortcut,
		&browserAppearancesJSON,
		&pref.DefaultBrowserID,
		&pref.ProxyBrowserID,
		&customBrowsersJSON,
		&commandBrowsersJSON,
		&urlRulesJSON,
		&browserRoutingRulesJSON,
		&urlPortAssignmentsJSON,
		&disabledBrowserIDsJSON,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return preferences.Default(), nil
	}
	if err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load preferences: %w", err)
	}
	if err := json.Unmarshal([]byte(browserAppearancesJSON), &pref.BrowserAppearances); err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load browser appearances: %w", err)
	}
	if pref.BrowserAppearances == nil {
		pref.BrowserAppearances = map[string]preferences.BrowserAppearance{}
	}
	if err := json.Unmarshal([]byte(customBrowsersJSON), &pref.CustomBrowsers); err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load custom browsers: %w", err)
	}
	if pref.CustomBrowsers == nil {
		pref.CustomBrowsers = []preferences.CustomBrowser{}
	}
	if commandBrowsersJSON.Valid {
		var commandBrowsers []preferences.CustomBrowser
		if err := json.Unmarshal([]byte(commandBrowsersJSON.String), &commandBrowsers); err != nil {
			return preferences.UserPreference{}, fmt.Errorf("load command browsers: %w", err)
		}
		pref.CustomBrowsers, err = mergeCustomBrowsers(pref.CustomBrowsers, commandBrowsers)
		if err != nil {
			return preferences.UserPreference{}, err
		}
	}
	pref.URLRules, err = loadURLRules(urlRulesJSON, browserRoutingRulesJSON)
	if err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load URL rules: %w", err)
	}
	if pref.URLRules == nil {
		pref.URLRules = []preferences.URLRule{}
	}
	if err := json.Unmarshal([]byte(urlPortAssignmentsJSON), &pref.URLPortAssignments); err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load URL port assignments: %w", err)
	}
	if pref.URLPortAssignments == nil {
		pref.URLPortAssignments = []preferences.URLPortAssignment{}
	}
	if err := json.Unmarshal([]byte(disabledBrowserIDsJSON), &pref.DisabledBrowserIDs); err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load disabled browser ids: %w", err)
	}
	if pref.DisabledBrowserIDs == nil {
		pref.DisabledBrowserIDs = []string{}
	}
	pref.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return pref, nil
}

func (s *PreferencesStore) Save(ctx context.Context, pref preferences.UserPreference) error {
	browserAppearances := pref.BrowserAppearances
	if browserAppearances == nil {
		browserAppearances = map[string]preferences.BrowserAppearance{}
	}
	browserAppearancesJSON, err := json.Marshal(browserAppearances)
	if err != nil {
		return fmt.Errorf("save browser appearances: %w", err)
	}
	browserRoutingRules := pref.URLRules
	if browserRoutingRules == nil {
		browserRoutingRules = []preferences.URLRule{}
	}
	browserRoutingRulesJSON, err := json.Marshal(browserRoutingRules)
	if err != nil {
		return fmt.Errorf("save URL rules: %w", err)
	}
	legacyURLRulesJSON, err := json.Marshal(projectLegacyURLRules(browserRoutingRules))
	if err != nil {
		return fmt.Errorf("save legacy-compatible URL rules: %w", err)
	}
	legacyBrowsers, commandBrowsers := splitCustomBrowsers(pref.CustomBrowsers)
	customBrowsersJSON, err := json.Marshal(legacyBrowsers)
	if err != nil {
		return fmt.Errorf("save custom browsers: %w", err)
	}
	commandBrowsersJSON, err := json.Marshal(commandBrowsers)
	if err != nil {
		return fmt.Errorf("save command browsers: %w", err)
	}
	urlPortAssignments := pref.URLPortAssignments
	if urlPortAssignments == nil {
		urlPortAssignments = []preferences.URLPortAssignment{}
	}
	urlPortAssignmentsJSON, err := json.Marshal(urlPortAssignments)
	if err != nil {
		return fmt.Errorf("save URL port assignments: %w", err)
	}
	disabledBrowserIDs := pref.DisabledBrowserIDs
	if disabledBrowserIDs == nil {
		disabledBrowserIDs = []string{}
	}
	disabledBrowserIDsJSON, err := json.Marshal(disabledBrowserIDs)
	if err != nil {
		return fmt.Errorf("save disabled browser ids: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_preferences(
			id, theme, last_selected_server_id, browser_switcher_shortcut,
			browser_switcher_backward_shortcut, browser_appearances_json,
			default_browser_id, proxy_browser_id, custom_browsers_json,
			command_browsers_json, url_rules_json, browser_routing_rules_json,
			url_port_assignments_json, disabled_browser_ids_json, updated_at
		)
		VALUES(1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			theme = excluded.theme,
			last_selected_server_id = excluded.last_selected_server_id,
			browser_switcher_shortcut = excluded.browser_switcher_shortcut,
			browser_switcher_backward_shortcut = excluded.browser_switcher_backward_shortcut,
			browser_appearances_json = excluded.browser_appearances_json,
			default_browser_id = excluded.default_browser_id,
			proxy_browser_id = excluded.proxy_browser_id,
			custom_browsers_json = excluded.custom_browsers_json,
			command_browsers_json = excluded.command_browsers_json,
			url_rules_json = excluded.url_rules_json,
			browser_routing_rules_json = excluded.browser_routing_rules_json,
			url_port_assignments_json = excluded.url_port_assignments_json,
			disabled_browser_ids_json = excluded.disabled_browser_ids_json,
			updated_at = excluded.updated_at
	`, string(pref.Theme), pref.LastSelectedServerID, pref.BrowserSwitcherShortcut, pref.BrowserSwitcherBackwardShortcut, string(browserAppearancesJSON), pref.DefaultBrowserID, pref.ProxyBrowserID, string(customBrowsersJSON), string(commandBrowsersJSON), string(legacyURLRulesJSON), string(browserRoutingRulesJSON), string(urlPortAssignmentsJSON), string(disabledBrowserIDsJSON), pref.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	return nil
}

func loadURLRules(legacyJSON string, modernJSON sql.NullString) ([]preferences.URLRule, error) {
	var legacyRules []preferences.URLRule
	if err := json.Unmarshal([]byte(legacyJSON), &legacyRules); err != nil {
		return nil, err
	}
	if legacyRules == nil {
		legacyRules = []preferences.URLRule{}
	}
	if !modernJSON.Valid {
		return legacyRules, nil
	}

	var modernRules []preferences.URLRule
	if err := json.Unmarshal([]byte(modernJSON.String), &modernRules); err != nil {
		return nil, err
	}
	if modernRules == nil {
		modernRules = []preferences.URLRule{}
	}
	var storedLegacyProjection []legacyURLRule
	if err := json.Unmarshal([]byte(legacyJSON), &storedLegacyProjection); err != nil {
		return nil, err
	}
	if storedLegacyProjection == nil {
		storedLegacyProjection = []legacyURLRule{}
	}
	if reflect.DeepEqual(storedLegacyProjection, projectLegacyURLRules(modernRules)) {
		return modernRules, nil
	}
	return legacyRules, nil
}

func splitCustomBrowsers(input []preferences.CustomBrowser) ([]preferences.CustomBrowser, []preferences.CustomBrowser) {
	legacyBrowsers := make([]preferences.CustomBrowser, 0, len(input))
	commandBrowsers := make([]preferences.CustomBrowser, 0, len(input))
	for _, browser := range input {
		if browser.Command != "" {
			commandBrowsers = append(commandBrowsers, browser)
			continue
		}
		legacyBrowsers = append(legacyBrowsers, browser)
	}
	return legacyBrowsers, commandBrowsers
}

func mergeCustomBrowsers(legacyBrowsers, commandBrowsers []preferences.CustomBrowser) ([]preferences.CustomBrowser, error) {
	if _, err := uniqueCustomBrowserIDs("legacy", legacyBrowsers); err != nil {
		return nil, err
	}
	for _, browser := range commandBrowsers {
		if strings.TrimSpace(browser.Command) == "" {
			return nil, fmt.Errorf("command browser storage contains a browser without a command")
		}
	}
	commandIDs, err := uniqueCustomBrowserIDs("command", commandBrowsers)
	if err != nil {
		return nil, err
	}
	merged := make([]preferences.CustomBrowser, 0, len(legacyBrowsers)+len(commandBrowsers))
	for _, browser := range legacyBrowsers {
		if _, replaced := commandIDs[strings.TrimSpace(browser.ID)]; replaced {
			continue
		}
		merged = append(merged, browser)
	}
	return append(merged, commandBrowsers...), nil
}

func uniqueCustomBrowserIDs(storageLabel string, browsers []preferences.CustomBrowser) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(browsers))
	for _, browser := range browsers {
		id := strings.TrimSpace(browser.ID)
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("duplicate %s custom browser id %q", storageLabel, id)
		}
		seen[id] = struct{}{}
	}
	return seen, nil
}

type legacyURLRule struct {
	ID        string                    `json:"id"`
	Pattern   string                    `json:"pattern"`
	Action    preferences.URLRuleAction `json:"action"`
	BrowserID string                    `json:"browserId,omitempty"`
	Command   string                    `json:"command,omitempty"`
}

func projectLegacyURLRules(input []preferences.URLRule) []legacyURLRule {
	projected := make([]legacyURLRule, 0, len(input))
	for _, rule := range input {
		pattern := rule.Pattern
		switch rule.MatchMode {
		case preferences.URLRuleMatchStartsWith:
			pattern = "^" + regexp.QuoteMeta(rule.Pattern)
		case preferences.URLRuleMatchEndsWith:
			pattern = regexp.QuoteMeta(rule.Pattern) + "$"
		case preferences.URLRuleMatchContains:
			pattern = regexp.QuoteMeta(rule.Pattern)
		}
		projected = append(projected, legacyURLRule{
			ID:        rule.ID,
			Pattern:   pattern,
			Action:    rule.Action,
			BrowserID: rule.BrowserID,
			Command:   rule.Command,
		})
	}
	return projected
}
