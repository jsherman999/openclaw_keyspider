package store

import (
	"context"
	"fmt"
	"time"
)

type ScanJob struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	TargetHost  string     `json:"target_host"`
	SinceSec    int        `json:"since_interval_seconds"`
	SpiderDepth int        `json:"spider_depth"`
	Status      string     `json:"status"`
	Error       *string    `json:"error"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

func (s *Store) EnqueueScanJob(ctx context.Context, host string, since time.Duration, depth int) (int64, error) {
	var id int64
	sinceSec := int(since.Seconds())
	if sinceSec <= 0 {
		sinceSec = 3600
	}
	err := s.db.Pool.QueryRow(ctx, `
INSERT INTO scan_jobs(kind, target_host, since_interval_seconds, spider_depth, status)
VALUES ('scan', $1, $2, $3, 'queued')
RETURNING id;
`, host, sinceSec, depth).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("enqueue scan job: %w", err)
	}
	return id, nil
}

// ClaimNextScanJob atomically claims a queued job.
func (s *Store) ClaimNextScanJob(ctx context.Context) (*ScanJob, error) {
	row := s.db.Pool.QueryRow(ctx, `
WITH next AS (
  SELECT id FROM scan_jobs
  WHERE status='queued'
  ORDER BY created_at ASC
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
UPDATE scan_jobs j
SET status='running', started_at=now()
FROM next
WHERE j.id=next.id
RETURNING j.id, j.kind, j.target_host, j.since_interval_seconds, j.spider_depth, j.status, j.error, j.created_at, j.started_at, j.finished_at;
`)

	var j ScanJob
	if err := row.Scan(&j.ID, &j.Kind, &j.TargetHost, &j.SinceSec, &j.SpiderDepth, &j.Status, &j.Error, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
		return nil, err
	}
	return &j, nil
}

func (s *Store) FinishScanJob(ctx context.Context, id int64, jobErr error) error {
	if jobErr == nil {
		_, err := s.db.Pool.Exec(ctx, `UPDATE scan_jobs SET status='done', finished_at=now() WHERE id=$1`, id)
		return err
	}
	msg := jobErr.Error()
	_, err := s.db.Pool.Exec(ctx, `UPDATE scan_jobs SET status='error', error=$2, finished_at=now() WHERE id=$1`, id, msg)
	return err
}

func (s *Store) GetScanJob(ctx context.Context, id int64) (*ScanJob, error) {
	row := s.db.Pool.QueryRow(ctx, `
SELECT id, kind, target_host, since_interval_seconds, spider_depth, status, error, created_at, started_at, finished_at
FROM scan_jobs WHERE id=$1
`, id)
	var j ScanJob
	if err := row.Scan(&j.ID, &j.Kind, &j.TargetHost, &j.SinceSec, &j.SpiderDepth, &j.Status, &j.Error, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
		return nil, err
	}
	return &j, nil
}
