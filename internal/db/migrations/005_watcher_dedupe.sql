-- Watcher dedupe + mode wiring

ALTER TABLE watchers
  ADD COLUMN IF NOT EXISTS last_event_sha256 text,
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS watchers_updated_idx ON watchers(updated_at);
