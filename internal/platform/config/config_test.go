package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ReadsYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `datastores:
  postgres:
    host: 127.0.0.1
    port: 5432
    username: app_user
    password: secret
    dbname: ai_gateway
providers:
  gemini:
    api_key: abc123
`
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
}

func TestLoadConfig_EnvOverridesYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `providers:
  gemini:
    api_key: from-file
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("PROVIDERS_GEMINI_API_KEY", "from-env")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Providers.Gemini.APIKey != "from-env" {
		t.Fatalf("gemini api key = %q, want %q", cfg.Providers.Gemini.APIKey, "from-env")
	}
}
