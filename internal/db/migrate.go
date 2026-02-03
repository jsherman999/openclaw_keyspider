package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	Name    string
	Content string
	Hash    string
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		h := sha256.Sum256(b)
		out = append(out, migration{Name: e.Name(), Content: string(b), Hash: hex.EncodeToString(h[:])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func EnsureMigrationsTable(ctx context.Context, d *DB) error {
	_, err := d.Pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  name text PRIMARY KEY,
  sha256 text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
);
`)
	return err
}

func ApplyMigrations(ctx context.Context, d *DB) error {
	if err := EnsureMigrationsTable(ctx, d); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, m := range migs {
		var existingHash string
		err := d.Pool.QueryRow(ctx, `SELECT sha256 FROM schema_migrations WHERE name=$1`, m.Name).Scan(&existingHash)
		if err == nil {
			if existingHash != m.Hash {
				return fmt.Errorf("migration %s hash mismatch (db=%s fs=%s)", m.Name, existingHash, m.Hash)
			}
			continue
		}
		// pgx returns error on no rows; simplest is attempt insert after exec

		tx, err := d.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		_, execErr := tx.Exec(ctx, m.Content)
		if execErr != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", m.Name, execErr)
		}
		_, insErr := tx.Exec(ctx, `INSERT INTO schema_migrations(name, sha256) VALUES ($1,$2)`, m.Name, m.Hash)
		if insErr != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record %s: %w", m.Name, insErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", m.Name, err)
		}
	}
	return nil
}
