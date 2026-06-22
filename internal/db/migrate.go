package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration is a single ordered SQL migration.
type Migration struct {
	Name string
	SQL  string
}

// LoadMigrations reads and sorts all *.sql files from the given filesystem.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	migs := make([]Migration, 0, len(names))
	for _, n := range names {
		b, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", n, err)
		}
		migs = append(migs, Migration{Name: n, SQL: string(b)})
	}
	return migs, nil
}

// Migrate applies any not-yet-applied migrations within transactions, recording
// each in schema_migrations with a checksum. It is idempotent and safe to run on
// every boot. Applied migrations whose checksum changed are reported as errors
// to prevent silent drift.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migs []Migration) (applied []string, err error) {
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name        TEXT PRIMARY KEY,
			checksum    TEXT NOT NULL,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return nil, fmt.Errorf("ensure schema_migrations: %w", err)
	}

	for _, m := range migs {
		sum := checksum(m.SQL)
		var existing string
		err = pool.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name=$1`, m.Name).Scan(&existing)
		if err == nil {
			if existing != sum {
				return applied, fmt.Errorf("migration %s already applied with a different checksum (drift)", m.Name)
			}
			continue // already applied, unchanged
		}

		tx, txErr := pool.Begin(ctx)
		if txErr != nil {
			return applied, fmt.Errorf("begin tx for %s: %w", m.Name, txErr)
		}
		if _, execErr := tx.Exec(ctx, m.SQL); execErr != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("apply %s: %w", m.Name, execErr)
		}
		if _, recErr := tx.Exec(ctx,
			`INSERT INTO schema_migrations (name, checksum) VALUES ($1,$2)`, m.Name, sum); recErr != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("record %s: %w", m.Name, recErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return applied, fmt.Errorf("commit %s: %w", m.Name, commitErr)
		}
		applied = append(applied, m.Name)
	}
	return applied, nil
}

func checksum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
