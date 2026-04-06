package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ssh-man/internal/domain/session"
)

type SessionHistoryStore struct {
	db *sql.DB
}

func NewSessionHistoryStore(db *sql.DB) *SessionHistoryStore {
	return &SessionHistoryStore{db: db}
}

func (s *SessionHistoryStore) Add(ctx context.Context, entry session.SessionHistoryEntry) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO session_history(id, configuration_id, started_at, ended_at, outcome, message) VALUES(?, ?, ?, ?, ?, ?)`, entry.ID, entry.ConfigurationID, entry.StartedAt.Format(time.RFC3339Nano), entry.EndedAt.Format(time.RFC3339Nano), string(entry.Outcome), entry.Message)
	if err != nil {
		return fmt.Errorf("add session history: %w", err)
	}
	return nil
}

func (s *SessionHistoryStore) ListByConfiguration(ctx context.Context, configurationID string) ([]session.SessionHistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, configuration_id, started_at, ended_at, outcome, message FROM session_history WHERE configuration_id = ? ORDER BY started_at DESC`, configurationID)
	if err != nil {
		return nil, fmt.Errorf("list session history: %w", err)
	}
	defer rows.Close()

	var entries []session.SessionHistoryEntry
	for rows.Next() {
		var entry session.SessionHistoryEntry
		var startedAt string
		var endedAt string
		if err := rows.Scan(&entry.ID, &entry.ConfigurationID, &startedAt, &endedAt, &entry.Outcome, &entry.Message); err != nil {
			return nil, fmt.Errorf("scan session history: %w", err)
		}
		entry.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
		entry.EndedAt, _ = time.Parse(time.RFC3339Nano, endedAt)
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
