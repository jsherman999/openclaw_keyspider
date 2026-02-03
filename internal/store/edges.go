package store

import (
	"context"
	"fmt"
)

func (s *Store) UpsertEdge(ctx context.Context, srcHostID *int64, srcLabel string, destHostID int64, evidenceType string, confidence int) (int64, error) {
	var id int64
	err := s.db.Pool.QueryRow(ctx, `
INSERT INTO edges(src_host_id, src_label, dest_host_id, evidence_type, confidence, first_seen, last_seen)
VALUES ($1,$2,$3,$4,$5, now(), now())
ON CONFLICT (src_label, dest_host_id)
DO UPDATE SET
  src_host_id=COALESCE(EXCLUDED.src_host_id, edges.src_host_id),
  evidence_type=EXCLUDED.evidence_type,
  confidence=GREATEST(edges.confidence, EXCLUDED.confidence),
  last_seen=now()
RETURNING id;
`, srcHostID, srcLabel, destHostID, evidenceType, confidence).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert edge: %w", err)
	}
	return id, nil
}

func (s *Store) InsertConcern(ctx context.Context, severity, ctype string, hostID *int64, keyID *int64, accessEventID *int64, details string) (int64, error) {
	var id int64
	err := s.db.Pool.QueryRow(ctx, `
INSERT INTO concerns(severity, type, host_id, key_id, access_event_id, details)
VALUES ($1,$2,$3,$4,$5,$6)
RETURNING id;
`, severity, ctype, hostID, keyID, accessEventID, details).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert concern: %w", err)
	}
	return id, nil
}
