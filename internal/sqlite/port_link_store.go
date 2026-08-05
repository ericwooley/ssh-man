package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	portlinkdomain "ssh-man/internal/domain/portlink"
)

type PortLinkStore struct {
	db *sql.DB
}

func NewPortLinkStore(db *sql.DB) *PortLinkStore {
	return &PortLinkStore{db: db}
}

func (store *PortLinkStore) ListByServer(ctx context.Context, serverID string) ([]portlinkdomain.Link, error) {
	rows, err := store.db.QueryContext(ctx, `
		SELECT id, server_id, port, name, scheme, favicon_data_url, created_at, updated_at
		FROM port_links
		WHERE server_id = ?
		ORDER BY port ASC
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list port links: %w", err)
	}
	defer rows.Close()

	var items []portlinkdomain.Link
	for rows.Next() {
		item, err := scanPortLink(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (store *PortLinkStore) GetByServerPort(ctx context.Context, serverID string, port int) (portlinkdomain.Link, error) {
	row := store.db.QueryRowContext(ctx, `
		SELECT id, server_id, port, name, scheme, favicon_data_url, created_at, updated_at
		FROM port_links
		WHERE server_id = ? AND port = ?
	`, serverID, port)
	item, err := scanPortLink(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return portlinkdomain.Link{}, portlinkdomain.ErrNotFound
	}
	return item, err
}

func (store *PortLinkStore) Save(ctx context.Context, item portlinkdomain.Link) error {
	_, err := store.db.ExecContext(ctx, `
		INSERT INTO port_links(id, server_id, port, name, scheme, favicon_data_url, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, port) DO UPDATE SET
			name = excluded.name,
			scheme = excluded.scheme,
			favicon_data_url = excluded.favicon_data_url,
			updated_at = excluded.updated_at
	`, item.ID, item.ServerID, item.Port, item.Name, string(item.Scheme), item.FaviconDataURL, item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save port link: %w", err)
	}
	return nil
}

func (store *PortLinkStore) Delete(ctx context.Context, id string) error {
	if _, err := store.db.ExecContext(ctx, `DELETE FROM port_links WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete port link: %w", err)
	}
	return nil
}

func scanPortLink(scan func(dest ...any) error) (portlinkdomain.Link, error) {
	var item portlinkdomain.Link
	var scheme string
	var createdAt string
	var updatedAt string
	if err := scan(
		&item.ID,
		&item.ServerID,
		&item.Port,
		&item.Name,
		&scheme,
		&item.FaviconDataURL,
		&createdAt,
		&updatedAt,
	); err != nil {
		return portlinkdomain.Link{}, err
	}
	item.Scheme = portlinkdomain.Scheme(scheme)
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return item, nil
}
