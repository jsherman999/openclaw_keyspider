-- Phase 2 key instance upsert support

CREATE UNIQUE INDEX IF NOT EXISTS key_instances_host_path_type_uq
  ON key_instances(host_id, path, instance_type);
