package db

import (
	"fmt"
	"os"

	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open opens a Postgres connection from typed config values.
func Open(cfg platformconfig.PostgresConfig) (*gorm.DB, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("datastores.postgres.host is required")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("datastores.postgres.port is required")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("datastores.postgres.username is required")
	}
	if cfg.DBName == "" {
		return nil, fmt.Errorf("datastores.postgres.dbname is required")
	}

	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.Username,
		cfg.Password,
		cfg.DBName,
		sslMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	return db, nil
}

// OpenFromEnv opens a Postgres connection using the DATABASE_URL env var.
func OpenFromEnv() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	return db, nil
}
