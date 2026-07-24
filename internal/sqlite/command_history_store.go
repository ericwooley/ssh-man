package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ssh-man/internal/domain/commandhistory"
)

type CommandHistoryStore struct {
	db *sql.DB
}

func NewCommandHistoryStore(db *sql.DB) *CommandHistoryStore {
	return &CommandHistoryStore{db: db}
}

func (s *CommandHistoryStore) Add(ctx context.Context, entry commandhistory.Entry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO command_history(
			id, server_id, command, output, exit_code, started_at, ended_at, truncated, error
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.ID,
		entry.ServerID,
		entry.Command,
		entry.Output,
		entry.ExitCode,
		entry.StartedAt.Format(time.RFC3339Nano),
		entry.EndedAt.Format(time.RFC3339Nano),
		entry.Truncated,
		entry.Error,
	)
	if err != nil {
		return fmt.Errorf("add command history: %w", err)
	}
	return nil
}

func (s *CommandHistoryStore) ListByServer(ctx context.Context, serverID string) ([]commandhistory.Entry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, command, output, exit_code, started_at, ended_at, truncated, error
		FROM command_history
		WHERE server_id = ?
		ORDER BY started_at DESC
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list command history: %w", err)
	}
	defer rows.Close()

	entries := []commandhistory.Entry{}
	for rows.Next() {
		var entry commandhistory.Entry
		var startedAt, endedAt string
		if err := rows.Scan(
			&entry.ID,
			&entry.ServerID,
			&entry.Command,
			&entry.Output,
			&entry.ExitCode,
			&startedAt,
			&endedAt,
			&entry.Truncated,
			&entry.Error,
		); err != nil {
			return nil, fmt.Errorf("scan command history: %w", err)
		}
		entry.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
		entry.EndedAt, _ = time.Parse(time.RFC3339Nano, endedAt)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list command history: %w", err)
	}
	return entries, nil
}

func (s *CommandHistoryStore) DeleteByServer(ctx context.Context, serverID, entryID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM command_history WHERE id = ? AND server_id = ?`, entryID, serverID)
	if err != nil {
		return fmt.Errorf("delete command history: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirm command history deletion: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("command history entry %q was not found", entryID)
	}
	return nil
}
