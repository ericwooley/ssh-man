package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ssh-man/internal/domain/server"
)

type ServerStore struct {
	db *sql.DB
}

func NewServerStore(db *sql.DB) *ServerStore {
	return &ServerStore{db: db}
}

func (s *ServerStore) List(ctx context.Context) ([]server.Server, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, host, port, username, auth_mode, key_reference, created_at, updated_at FROM servers ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()

	var items []server.Server
	for rows.Next() {
		var item server.Server
		var authMode string
		var createdAt string
		var updatedAt string
		if err := rows.Scan(&item.ID, &item.Name, &item.Host, &item.Port, &item.Username, &authMode, &item.KeyReference, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}
		item.AuthMode = server.AuthMode(authMode)
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *ServerStore) Get(ctx context.Context, id string) (server.Server, error) {
	var item server.Server
	var authMode string
	var createdAt string
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, name, host, port, username, auth_mode, key_reference, created_at, updated_at FROM servers WHERE id = ?`, id).Scan(
		&item.ID,
		&item.Name,
		&item.Host,
		&item.Port,
		&item.Username,
		&authMode,
		&item.KeyReference,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return server.Server{}, err
	}
	item.AuthMode = server.AuthMode(authMode)
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return item, nil
}

func (s *ServerStore) Save(ctx context.Context, item server.Server) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO servers(id, name, host, port, username, auth_mode, key_reference, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			host = excluded.host,
			port = excluded.port,
			username = excluded.username,
			auth_mode = excluded.auth_mode,
			key_reference = excluded.key_reference,
			updated_at = excluded.updated_at
	`, item.ID, item.Name, item.Host, item.Port, item.Username, string(item.AuthMode), item.KeyReference, item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save server: %w", err)
	}
	return nil
}

func (s *ServerStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	return nil
}
