package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"official-site-system/internal/config"
	"official-site-system/internal/store"
	"official-site-system/migrations"
)

func main() {
	seed := flag.Bool("seed", false, "insert demo content only when no release exists")
	flag.Parse()
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	repo, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("database connection failed; check DATABASE_URL and database availability")
	}
	defer repo.DB.Close()
	if err = migrations.Apply(ctx, repo.DB, *seed); err != nil {
		log.Fatal("migration failed; inspect database schema and migration checksums")
	}
	log.Print("migrations complete")
}
