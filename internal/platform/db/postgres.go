package db

import (
	"fmt"
	"os"
	"strings"

	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	DBTypePostgres = "postgres"
	DBTypeMySQL    = "mysql"
)

// Open opens a SQL connection from typed config values.
// Supported dbType values: postgres, mysql.
func Open(dbType string, cfg platformconfig.SQLConfig) (*gorm.DB, error) {
	normalizedDBType := strings.ToLower(strings.TrimSpace(dbType))

	switch normalizedDBType {
	case DBTypePostgres:
		if err := validateSQLConfig(cfg, "datastores.postgres"); err != nil {
			return nil, err
		}

		db, err := gorm.Open(postgres.Open(buildPostgresDSN(cfg)), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("open postgres connection: %w", err)
		}

		return db, nil
	case DBTypeMySQL:
		if err := validateSQLConfig(cfg, "datastores.mysql"); err != nil {
			return nil, err
		}

		db, err := gorm.Open(mysql.Open(buildMySQLDSN(cfg)), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("open mysql connection: %w", err)
		}

		return db, nil
	default:
		return nil, fmt.Errorf("unsupported database type %q: expected postgres or mysql", dbType)
	}
}

func validateSQLConfig(cfg platformconfig.SQLConfig, configPrefix string) error {
	if cfg.Host == "" {
		return fmt.Errorf("%s.host is required", configPrefix)
	}
	if cfg.Port == 0 {
		return fmt.Errorf("%s.port is required", configPrefix)
	}
	if cfg.Username == "" {
		return fmt.Errorf("%s.username is required", configPrefix)
	}
	if cfg.DBName == "" {
		return fmt.Errorf("%s.dbname is required", configPrefix)
	}

	return nil
}

func buildPostgresDSN(cfg platformconfig.SQLConfig) string {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.Username,
		cfg.Password,
		cfg.DBName,
		sslMode,
	)

}

func buildMySQLDSN(cfg platformconfig.SQLConfig) string {
	params := strings.TrimSpace(cfg.Params)
	if params == "" {
		params = "?parseTime=true"
	} else if !strings.HasPrefix(params, "?") {
		params = "?" + params
	}

	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		params,
	)
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
