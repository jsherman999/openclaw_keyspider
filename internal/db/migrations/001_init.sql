-- Phase 1 schema (PostgreSQL 13)

CREATE TABLE IF NOT EXISTS hosts (
  id bigserial PRIMARY KEY,
  hostname text NOT NULL,
  fqdn text,
  os_type text NOT NULL DEFAULT 'linux',
  reachable_from_jump boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS hosts_hostname_uq ON hosts(hostname);
CREATE INDEX IF NOT EXISTS hosts_fqdn_idx ON hosts(fqdn);

CREATE TABLE IF NOT EXISTS ssh_keys (
  id bigserial PRIMARY KEY,
  key_type text NOT NULL,
  public_key text,
  fingerprint_sha256 text NOT NULL,
  comment text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ssh_keys_fp_uq ON ssh_keys(fingerprint_sha256);

CREATE TABLE IF NOT EXISTS key_instances (
  id bigserial PRIMARY KEY,
  host_id bigint NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  username text,
  path text NOT NULL,
  key_id bigint REFERENCES ssh_keys(id) ON DELETE SET NULL,
  instance_type text NOT NULL, -- authorized_key|public|private
  owner text,
  "group" text,
  perm text,
  size_bytes bigint,
  mtime timestamptz,
  first_seen timestamptz NOT NULL DEFAULT now(),
  last_seen timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS key_instances_host_idx ON key_instances(host_id);
CREATE INDEX IF NOT EXISTS key_instances_key_idx ON key_instances(key_id);
CREATE INDEX IF NOT EXISTS key_instances_path_idx ON key_instances(path);

CREATE TABLE IF NOT EXISTS access_events (
  id bigserial PRIMARY KEY,
  ts timestamptz NOT NULL,
  dest_host_id bigint NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  dest_user text,
  source_host text,
  source_ip inet,
  source_port int,
  fingerprint_sha256 text,
  key_id bigint REFERENCES ssh_keys(id) ON DELETE SET NULL,
  auth_method text,
  result text,
  raw_line text NOT NULL
);

CREATE INDEX IF NOT EXISTS access_events_ts_idx ON access_events(ts);
CREATE INDEX IF NOT EXISTS access_events_dest_idx ON access_events(dest_host_id, ts);
CREATE INDEX IF NOT EXISTS access_events_fp_idx ON access_events(fingerprint_sha256);

CREATE TABLE IF NOT EXISTS edges (
  id bigserial PRIMARY KEY,
  src_host_id bigint REFERENCES hosts(id) ON DELETE SET NULL,
  src_label text NOT NULL,
  dest_host_id bigint NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  first_seen timestamptz NOT NULL DEFAULT now(),
  last_seen timestamptz NOT NULL DEFAULT now(),
  evidence_type text NOT NULL,
  confidence int NOT NULL DEFAULT 50
);

CREATE INDEX IF NOT EXISTS edges_dest_idx ON edges(dest_host_id);
CREATE INDEX IF NOT EXISTS edges_src_idx ON edges(src_host_id);

CREATE TABLE IF NOT EXISTS concerns (
  id bigserial PRIMARY KEY,
  severity text NOT NULL,
  type text NOT NULL,
  host_id bigint REFERENCES hosts(id) ON DELETE SET NULL,
  key_id bigint REFERENCES ssh_keys(id) ON DELETE SET NULL,
  access_event_id bigint REFERENCES access_events(id) ON DELETE SET NULL,
  details text,
  created_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz
);

CREATE INDEX IF NOT EXISTS concerns_unresolved_idx ON concerns(resolved_at) WHERE resolved_at IS NULL;
