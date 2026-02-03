package store

import (
	"context"
	"time"
)

type Edge struct {
	ID          int64     `json:"id"`
	SrcHostID   *int64    `json:"src_host_id"`
	SrcLabel    string    `json:"src_label"`
	DestHostID  int64     `json:"dest_host_id"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Evidence    string    `json:"evidence_type"`
	Confidence  int       `json:"confidence"`
}

func (s *Store) ListEdges(ctx context.Context, limit int) ([]Edge, error) {
	rows, err := s.db.Pool.Query(ctx, `
SELECT id, src_host_id, src_label, dest_host_id, first_seen, last_seen, evidence_type, confidence
FROM edges
ORDER BY last_seen DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ID, &e.SrcHostID, &e.SrcLabel, &e.DestHostID, &e.FirstSeen, &e.LastSeen, &e.Evidence, &e.Confidence); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
