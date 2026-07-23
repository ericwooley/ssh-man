package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	var urlRulesJSON string
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT theme, last_selected_server_id, browser_switcher_shortcut,
		       browser_switcher_backward_shortcut, browser_appearances_json,
		       default_browser_id, proxy_browser_id, custom_browsers_json,
		       url_rules_json,
		       updated_at
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
		&urlRulesJSON,
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
	if err := json.Unmarshal([]byte(urlRulesJSON), &pref.URLRules); err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load URL rules: %w", err)
	}
	if pref.URLRules == nil {
		pref.URLRules = []preferences.URLRule{}
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
	urlRules := pref.URLRules
	if urlRules == nil {
		urlRules = []preferences.URLRule{}
	}
	urlRulesJSON, err := json.Marshal(urlRules)
	if err != nil {
		return fmt.Errorf("save URL rules: %w", err)
	}
	customBrowsers := pref.CustomBrowsers
	if customBrowsers == nil {
		customBrowsers = []preferences.CustomBrowser{}
	}
	customBrowsersJSON, err := json.Marshal(customBrowsers)
	if err != nil {
		return fmt.Errorf("save custom browsers: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_preferences(
			id, theme, last_selected_server_id, browser_switcher_shortcut,
			browser_switcher_backward_shortcut, browser_appearances_json,
			default_browser_id, proxy_browser_id, custom_browsers_json,
			url_rules_json,
			updated_at
		)
		VALUES(1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			theme = excluded.theme,
			last_selected_server_id = excluded.last_selected_server_id,
			browser_switcher_shortcut = excluded.browser_switcher_shortcut,
			browser_switcher_backward_shortcut = excluded.browser_switcher_backward_shortcut,
			browser_appearances_json = excluded.browser_appearances_json,
			default_browser_id = excluded.default_browser_id,
			proxy_browser_id = excluded.proxy_browser_id,
			custom_browsers_json = excluded.custom_browsers_json,
			url_rules_json = excluded.url_rules_json,
			updated_at = excluded.updated_at
	`, string(pref.Theme), pref.LastSelectedServerID, pref.BrowserSwitcherShortcut, pref.BrowserSwitcherBackwardShortcut, string(browserAppearancesJSON), pref.DefaultBrowserID, pref.ProxyBrowserID, string(customBrowsersJSON), string(urlRulesJSON), pref.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	return nil
}
