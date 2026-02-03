package store

import (
	"context"
	"fmt"
	"time"
)

type SSHKey struct {
	ID              int64      `json:"id"`
	KeyType          string     `json:"key_type"`
	PublicKey        *string    `json:"public_key"`
	FingerprintSHA256 string    `json:"fingerprint_sha256"`
	Comment          *string    `json:"comment"`
	CreatedAt        time.Time  `json:"created_at"`
}

type KeyInstance struct {
	ID           int64      `json:"id"`
	HostID       int64      `json:"host_id"`
	Username     *string    `json:"username"`
	Path         string     `json:"path"`
	KeyID        *int64     `json:"key_id"`
	InstanceType string     `json:"instance_type"`
	Owner        *string    `json:"owner"`
	Group        *string    `json:"group"`
	Perm         *string    `json:"perm"`
	SizeBytes    *int64     `json:"size_bytes"`
	Mtime        *time.Time `json:"mtime"`
	FirstSeen    time.Time  `json:"first_seen"`
	LastSeen     time.Time  `json:"last_seen"`
}

func (s *Store) UpsertSSHKey(ctx context.Context, keyType string, publicKey *string, fp256 string, comment *string) (int64, error) {
	var id int64
	err := s.db.Pool.QueryRow(ctx, `
INSERT INTO ssh_keys(key_type, public_key, fingerprint_sha256, comment)
VALUES ($1,$2,$3,$4)
ON CONFLICT (fingerprint_sha256) DO UPDATE
SET key_type=EXCLUDED.key_type,
    public_key=COALESCE(EXCLUDED.public_key, ssh_keys.public_key),
    comment=COALESCE(EXCLUDED.comment, ssh_keys.comment)
RETURNING id;
`, keyType, publicKey, fp256, comment).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert ssh_key: %w", err)
	}
	return id, nil
}

func (s *Store) UpsertKeyInstance(ctx context.Context, ki *KeyInstance) (int64, error) {
	var id int64
	err := s.db.Pool.QueryRow(ctx, `
INSERT INTO key_instances(host_id, username, path, key_id, instance_type, owner, "group", perm, size_bytes, mtime, first_seen, last_seen)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, COALESCE($11, now()), now())
ON CONFLICT (host_id, path, instance_type)
DO UPDATE SET
  key_id=COALESCE(EXCLUDED.key_id, key_instances.key_id),
  owner=COALESCE(EXCLUDED.owner, key_instances.owner),
  "group"=COALESCE(EXCLUDED."group", key_instances."group"),
  perm=COALESCE(EXCLUDED.perm, key_instances.perm),
  size_bytes=COALESCE(EXCLUDED.size_bytes, key_instances.size_bytes),
  mtime=COALESCE(EXCLUDED.mtime, key_instances.mtime),
  last_seen=now()
RETURNING id;
`, ki.HostID, ki.Username, ki.Path, ki.KeyID, ki.InstanceType, ki.Owner, ki.Group, ki.Perm, ki.SizeBytes, ki.Mtime, ki.FirstSeen).Scan(&id)
	if err != nil {
		// On conflict requires unique constraint; we will add it in migration 003.
		return 0, fmt.Errorf("upsert key_instance: %w", err)
	}
	return id, nil
}
