package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/db"
)

type Store struct{ db *db.DB }

func New(d *db.DB) *Store { return &Store{db: d} }

type Host struct {
	ID                int64      `json:"id"`
	Hostname          string     `json:"hostname"`
	FQDN              *string    `json:"fqdn"`
	OSType            string     `json:"os_type"`
	ReachableFromJump bool       `json:"reachable_from_jump"`
	CreatedAt         time.Time  `json:"created_at"`
	LastSeen          *time.Time `json:"last_seen"`
}

type AccessEvent struct {
	ID              int64     `json:"id"`
	TS              time.Time `json:"ts"`
	DestHostID      int64     `json:"dest_host_id"`
	DestUser        *string   `json:"dest_user"`
	SourceHost      *string   `json:"source_host"`
	SourceIP        *string   `json:"source_ip"`
	SourcePort      *int      `json:"source_port"`
	Fingerprint     *string   `json:"fingerprint_sha256"`
	AuthMethod      *string   `json:"auth_method"`
	Result          *string   `json:"result"`
	RawLine         string    `json:"raw_line"`
}

func (s *Store) UpsertHost(ctx context.Context, hostname string, fqdn *string, osType string, reachable bool) (int64, error) {
	var id int64
	err := s.db.Pool.QueryRow(ctx, `
INSERT INTO hosts(hostname,fqdn,os_type,reachable_from_jump,last_seen)
VALUES ($1,$2,$3,$4, now())
ON CONFLICT (hostname) DO UPDATE SET fqdn=COALESCE(EXCLUDED.fqdn, hosts.fqdn), os_type=EXCLUDED.os_type, reachable_from_jump=EXCLUDED.reachable_from_jump, last_seen=now()
RETURNING id;
`, hostname, fqdn, osType, reachable).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert host: %w", err)
	}
	return id, nil
}

func (s *Store) InsertAccessEvent(ctx context.Context, ev *AccessEvent) (int64, error) {
	var id int64
	err := s.db.Pool.QueryRow(ctx, `
INSERT INTO access_events(ts, dest_host_id, dest_user, source_host, source_ip, source_port, fingerprint_sha256, auth_method, result, raw_line)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id;
`, ev.TS, ev.DestHostID, ev.DestUser, ev.SourceHost, ev.SourceIP, ev.SourcePort, ev.Fingerprint, ev.AuthMethod, ev.Result, ev.RawLine).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert access_event: %w", err)
	}
	return id, nil
}

func (s *Store) ListHosts(ctx context.Context, limit int) ([]Host, error) {
	rows, err := s.db.Pool.Query(ctx, `SELECT id, hostname, fqdn, os_type, reachable_from_jump, created_at, last_seen FROM hosts ORDER BY hostname LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.Hostname, &h.FQDN, &h.OSType, &h.ReachableFromJump, &h.CreatedAt, &h.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) ListAccessEvents(ctx context.Context, hostID int64, limit int) ([]AccessEvent, error) {
	rows, err := s.db.Pool.Query(ctx, `
SELECT id, ts, dest_host_id, dest_user, source_host, source_ip::text, source_port, fingerprint_sha256, auth_method, result, raw_line
FROM access_events
WHERE dest_host_id=$1
ORDER BY ts DESC
LIMIT $2
`, hostID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccessEvent
	for rows.Next() {
		var ev AccessEvent
		if err := rows.Scan(&ev.ID, &ev.TS, &ev.DestHostID, &ev.DestUser, &ev.SourceHost, &ev.SourceIP, &ev.SourcePort, &ev.Fingerprint, &ev.AuthMethod, &ev.Result, &ev.RawLine); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
