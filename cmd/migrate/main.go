// Command migrate applies metadata migrations and optionally seeds core data
// and demo data. Used by `make migrate` / `make seed`.
//
//	migrate            # apply migrations only
//	migrate --seed     # migrations + core seed (roles/permissions/super-admin)
//	migrate --demo     # migrations + core seed + demo data (ddag_demo, demo API/client)
package main

import (
	"context"
	"flag"
	"os"

	"github.com/ddag/ddag/internal/bootstrap"
	"github.com/ddag/ddag/internal/config"
	"github.com/ddag/ddag/internal/db"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/secret"
)

func main() {
	seedCore := flag.Bool("seed", false, "seed core data (roles, permissions, super-admin)")
	seedDemo := flag.Bool("demo", false, "seed demo data (implies --seed)")
	flag.Parse()

	cfg := config.Load("migrate")
	log := logging.New("migrate", cfg.LogLevel)
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.Metadata)
	if err != nil {
		log.Error("connect_metadata_failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	if err := bootstrap.Migrate(ctx, pool, log); err != nil {
		log.Error("migrate_failed", "error", err.Error())
		os.Exit(1)
	}

	if *seedCore || *seedDemo {
		if err := bootstrap.SeedCore(ctx, pool, log); err != nil {
			log.Error("seed_core_failed", "error", err.Error())
			os.Exit(1)
		}
	}

	if *seedDemo {
		sec, err := secret.NewEnvelopeStore(pool, cfg.Secret.MasterKeyB64)
		if err != nil {
			log.Error("secret_store_failed", "error", err.Error())
			os.Exit(1)
		}
		if err := bootstrap.SeedDemo(ctx, pool, sec, cfg, log); err != nil {
			log.Error("seed_demo_failed", "error", err.Error())
			os.Exit(1)
		}
	}

	log.Info("done")
}
