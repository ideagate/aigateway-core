package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ReadsYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := "datastores:\n" +
		"  postgres:\n" +
		"    host: 127.0.0.1\n" +
		"    port: 5432\n" +
		"    username: app_user\n" +
		"    password: secret\n" +
		"    dbname: ai_gateway\n" +
		"scheduler:\n" +
		"  sync_batch_job_status_cron: \"*/5 * * * *\"\n" +
		"providers:\n" +
		"  gemini:\n" +
		"    api_key: abc123\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Datastores.Postgres.Host != "127.0.0.1" {
		t.Fatalf("postgres host = %q, want %q", cfg.Datastores.Postgres.Host, "127.0.0.1")
	}
	if cfg.Datastores.Postgres.Port != 5432 {
		t.Fatalf("postgres port = %d, want %d", cfg.Datastores.Postgres.Port, 5432)
	}
	if cfg.Providers.Gemini.APIKey != "abc123" {
		t.Fatalf("gemini api key = %q, want %q", cfg.Providers.Gemini.APIKey, "abc123")
	}
	if cfg.Scheduler.SyncBatchJobStatusCron != "*/5 * * * *" {
		t.Fatalf("sync batch job status cron = %q, want %q", cfg.Scheduler.SyncBatchJobStatusCron, "*/5 * * * *")
	}
}

func TestLoadConfig_EnvOverridesYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := "providers:\n" +
		"  gemini:\n" +
		"    api_key: from-file\n" +
		"scheduler:\n" +
		"  sync_batch_job_status_cron: \"*/10 * * * *\"\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("PROVIDERS_GEMINI_API_KEY", "from-env")
	t.Setenv("SCHEDULER_SYNC_BATCH_JOB_STATUS_CRON", "*/1 * * * *")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Providers.Gemini.APIKey != "from-env" {
		t.Fatalf("gemini api key = %q, want %q", cfg.Providers.Gemini.APIKey, "from-env")
	}
	if cfg.Scheduler.SyncBatchJobStatusCron != "*/1 * * * *" {
		t.Fatalf("sync batch job status cron = %q, want %q", cfg.Scheduler.SyncBatchJobStatusCron, "*/1 * * * *")
	}
}
