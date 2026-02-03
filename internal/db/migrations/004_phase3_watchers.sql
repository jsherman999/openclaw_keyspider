-- Phase 3 watcher state

CREATE TABLE IF NOT EXISTS watchers (
  id bigserial PRIMARY KEY,
  host_id bigint NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  enabled boolean NOT NULL DEFAULT true,
  mode text NOT NULL DEFAULT 'auto', -- auto|journal|tail
  cursor text,
  last_heartbeat timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS watchers_host_uq ON watchers(host_id);
