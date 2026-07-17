// Package bootstrap wires schema migrations and idempotent seeding so the
// platform comes up ready to use.
package bootstrap

import (
	"context"

	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate applies all embedded migrations to the metadata database.
func Migrate(ctx context.Context, pool *pgxpool.Pool, log *logging.Logger) error {
	migs, err := db.LoadMigrations(migrations.FS)
	if err != nil {
		return err
	}
	applied, err := db.Migrate(ctx, pool, migs)
	if err != nil {
		return err
	}
	if len(applied) > 0 {
		log.Info("migrations_applied", "count", len(applied), "names", applied)
	} else {
		log.Info("migrations_up_to_date")
	}
	return nil
}
