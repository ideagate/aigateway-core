package main

import (
	"flag"
	"log"

	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	platformdb "github.com/ideagate/aigateway-core/internal/platform/db"
)

func main() {
	configPath := flag.String("config", "config/default.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := platformconfig.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := platformdb.Open(cfg.Datastores.Postgres)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	migrationModels := models.MigrationModels()
	if err := db.AutoMigrate(migrationModels...); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	log.Printf("database migration completed for %d model(s)", len(migrationModels))
}
