// Package migrations applies embedded, versioned SQL in a single transaction.
package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed *.sql
var files embed.FS

func Apply(ctx context.Context, db *sql.DB, seed bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serializes migrations and seed runs across independently started processes.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(8988001)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}
	names, err := fs.Glob(files, "[0-9]*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		body, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(body))
		var recorded string
		err = tx.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE name = $1`, name).Scan(&recorded)
		if err == nil {
			if recorded != checksum {
				return fmt.Errorf("migration %s checksum mismatch", name)
			}
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`, name, checksum); err != nil {
			return err
		}
	}
	if seed {
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM site_releases)`).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			body, err := files.ReadFile("seed.sql")
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("seed: %w", err)
			}
		}
	}
	return tx.Commit()
}
