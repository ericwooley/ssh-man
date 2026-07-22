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
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT theme, last_selected_server_id, browser_switcher_shortcut,
		       browser_switcher_backward_shortcut, browser_appearances_json,
		       updated_at
		FROM user_preferences
		WHERE id = 1
	`).Scan(
		&pref.Theme,
		&pref.LastSelectedServerID,
		&pref.BrowserSwitcherShortcut,
		&pref.BrowserSwitcherBackwardShortcut,
		&browserAppearancesJSON,
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
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_preferences(
			id, theme, last_selected_server_id, browser_switcher_shortcut,
			browser_switcher_backward_shortcut, browser_appearances_json,
			updated_at
		)
		VALUES(1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			theme = excluded.theme,
			last_selected_server_id = excluded.last_selected_server_id,
			browser_switcher_shortcut = excluded.browser_switcher_shortcut,
			browser_switcher_backward_shortcut = excluded.browser_switcher_backward_shortcut,
			browser_appearances_json = excluded.browser_appearances_json,
			updated_at = excluded.updated_at
	`, string(pref.Theme), pref.LastSelectedServerID, pref.BrowserSwitcherShortcut, pref.BrowserSwitcherBackwardShortcut, string(browserAppearancesJSON), pref.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	return nil
}
