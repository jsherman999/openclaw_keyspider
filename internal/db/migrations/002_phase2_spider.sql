-- Phase 2 additions

CREATE TABLE IF NOT EXISTS scan_jobs (
  id bigserial PRIMARY KEY,
  kind text NOT NULL,
  target_host text NOT NULL,
  since_interval_seconds int NOT NULL DEFAULT 604800,
  spider_depth int NOT NULL DEFAULT 0,
  status text NOT NULL DEFAULT 'queued',
  error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz
);

CREATE INDEX IF NOT EXISTS scan_jobs_status_idx ON scan_jobs(status, created_at);

-- De-dup edges by (src_label, dest_host_id)
CREATE UNIQUE INDEX IF NOT EXISTS edges_src_dest_uq ON edges(src_label, dest_host_id);
