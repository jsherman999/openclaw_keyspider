package store

import (
	"context"
	"database/sql"
	"fmt"
)

type WatcherState struct {
	HostID          int64
	Mode            string
	Cursor          *string
	LastEventSHA256 *string
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
	var lastHash sql.NullString
	err := s.db.Pool.QueryRow(ctx, `SELECT mode, cursor, last_event_sha256 FROM watchers WHERE host_id=$1`, hostID).Scan(&mode, &cursor, &lastHash)
	if err != nil {
		return nil, fmt.Errorf("get watcher state: %w", err)
	}
	var c *string
	if cursor.Valid {
		c = &cursor.String
	}
	var lh *string
	if lastHash.Valid {
		lh = &lastHash.String
	}
	return &WatcherState{HostID: hostID, Mode: mode, Cursor: c, LastEventSHA256: lh}, nil
}

func (s *Store) UpdateWatcherCursor(ctx context.Context, hostID int64, cursor string) error {
	_, err := s.db.Pool.Exec(ctx, `UPDATE watchers SET cursor=$2, last_heartbeat=now(), updated_at=now() WHERE host_id=$1`, hostID, cursor)
	return err
}

func (s *Store) UpdateWatcherLastHash(ctx context.Context, hostID int64, sha256 string) error {
	_, err := s.db.Pool.Exec(ctx, `UPDATE watchers SET last_event_sha256=$2, updated_at=now() WHERE host_id=$1`, hostID, sha256)
	return err
}
