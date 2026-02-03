package store

import (
	"context"
	"database/sql"
	"fmt"
)

type WatcherState struct {
	HostID int64
	Mode   string
	Cursor *string
}

func (s *Store) EnsureWatcher(ctx context.Context, hostID int64, mode string) error {
	_, err := s.db.Pool.Exec(ctx, `
INSERT INTO watchers(host_id, enabled, mode)
VALUES ($1, true, $2)
ON CONFLICT (host_id) DO UPDATE SET enabled=true, mode=EXCLUDED.mode;
`, hostID, mode)
	return err
}

func (s *Store) GetWatcherState(ctx context.Context, hostID int64) (*WatcherState, error) {
	var mode string
	var cursor sql.NullString
	err := s.db.Pool.QueryRow(ctx, `SELECT mode, cursor FROM watchers WHERE host_id=$1`, hostID).Scan(&mode, &cursor)
	if err != nil {
		return nil, fmt.Errorf("get watcher state: %w", err)
	}
	var c *string
	if cursor.Valid {
		c = &cursor.String
	}
	return &WatcherState{HostID: hostID, Mode: mode, Cursor: c}, nil
}

func (s *Store) UpdateWatcherCursor(ctx context.Context, hostID int64, cursor string) error {
	_, err := s.db.Pool.Exec(ctx, `UPDATE watchers SET cursor=$2, last_heartbeat=now() WHERE host_id=$1`, hostID, cursor)
	return err
}
