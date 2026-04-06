package sqlite

import (
	"context"
	"database/sql"
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
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT theme, last_selected_server_id, updated_at FROM user_preferences WHERE id = 1`).Scan(&pref.Theme, &pref.LastSelectedServerID, &updatedAt)
	if err == sql.ErrNoRows {
		return preferences.Default(), nil
	}
	if err != nil {
		return preferences.UserPreference{}, fmt.Errorf("load preferences: %w", err)
	}
	pref.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return pref, nil
}

func (s *PreferencesStore) Save(ctx context.Context, pref preferences.UserPreference) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_preferences(id, theme, last_selected_server_id, updated_at)
		VALUES(1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			theme = excluded.theme,
			last_selected_server_id = excluded.last_selected_server_id,
			updated_at = excluded.updated_at
	`, string(pref.Theme), pref.LastSelectedServerID, pref.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	return nil
}
