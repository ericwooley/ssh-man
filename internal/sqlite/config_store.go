package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	configdomain "ssh-man/internal/domain/config"
)

type ConfigStore struct {
	db *sql.DB
}

func NewConfigStore(db *sql.DB) *ConfigStore {
	return &ConfigStore{db: db}
}

func (s *ConfigStore) ListByServer(ctx context.Context, serverID string) ([]configdomain.ConnectionConfiguration, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, label, connection_type, local_port, remote_host, remote_port, socks_port, auto_reconnect_enabled, start_on_launch, notes, created_at, updated_at FROM connection_configurations WHERE server_id = ? ORDER BY label ASC`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list configurations: %w", err)
	}
	defer rows.Close()

	var items []configdomain.ConnectionConfiguration
	for rows.Next() {
		item, err := scanConfiguration(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ConfigStore) ListAll(ctx context.Context) ([]configdomain.ConnectionConfiguration, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, label, connection_type, local_port, remote_host, remote_port, socks_port, auto_reconnect_enabled, start_on_launch, notes, created_at, updated_at FROM connection_configurations ORDER BY label ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all configurations: %w", err)
	}
	defer rows.Close()

	var items []configdomain.ConnectionConfiguration
	for rows.Next() {
		item, err := scanConfiguration(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ConfigStore) Get(ctx context.Context, id string) (configdomain.ConnectionConfiguration, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, server_id, label, connection_type, local_port, remote_host, remote_port, socks_port, auto_reconnect_enabled, start_on_launch, notes, created_at, updated_at FROM connection_configurations WHERE id = ?`, id)
	return scanConfiguration(row.Scan)
}

func (s *ConfigStore) Save(ctx context.Context, item configdomain.ConnectionConfiguration) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connection_configurations(id, server_id, label, connection_type, local_port, remote_host, remote_port, socks_port, auto_reconnect_enabled, start_on_launch, notes, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			server_id = excluded.server_id,
			label = excluded.label,
			connection_type = excluded.connection_type,
			local_port = excluded.local_port,
			remote_host = excluded.remote_host,
			remote_port = excluded.remote_port,
			socks_port = excluded.socks_port,
			auto_reconnect_enabled = excluded.auto_reconnect_enabled,
			start_on_launch = excluded.start_on_launch,
			notes = excluded.notes,
			updated_at = excluded.updated_at
	`, item.ID, item.ServerID, item.Label, string(item.ConnectionType), item.LocalPort, item.RemoteHost, item.RemotePort, item.SocksPort, boolToInt(item.AutoReconnectEnabled), boolToInt(item.StartOnLaunch), item.Notes, item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save configuration: %w", err)
	}
	return nil
}

func (s *ConfigStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM connection_configurations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete configuration: %w", err)
	}
	return nil
}

func scanConfiguration(scan func(dest ...any) error) (configdomain.ConnectionConfiguration, error) {
	var item configdomain.ConnectionConfiguration
	var connectionType string
	var autoReconnect int
	var startOnLaunch int
	var createdAt string
	var updatedAt string
	if err := scan(&item.ID, &item.ServerID, &item.Label, &connectionType, &item.LocalPort, &item.RemoteHost, &item.RemotePort, &item.SocksPort, &autoReconnect, &startOnLaunch, &item.Notes, &createdAt, &updatedAt); err != nil {
		return configdomain.ConnectionConfiguration{}, err
	}
	item.ConnectionType = configdomain.ConnectionType(connectionType)
	item.AutoReconnectEnabled = autoReconnect == 1
	item.StartOnLaunch = startOnLaunch == 1
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return item, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
