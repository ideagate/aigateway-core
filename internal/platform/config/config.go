package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Datastores DatastoresConfig `mapstructure:"datastores"`
	Providers  ProvidersConfig  `mapstructure:"providers"`
	Scheduler  SchedulerConfig  `mapstructure:"scheduler"`
}

type DatastoresConfig struct {
	DBType   string         `mapstructure:"db_type"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

type SQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	Params   string `mapstructure:"params"`
}

type PostgresConfig = SQLConfig
type MySQLConfig = SQLConfig

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
}

type ProvidersConfig struct {
	Gemini   GeminiConfig   `mapstructure:"gemini"`
	VertexAI VertexAIConfig `mapstructure:"vertexai"`
	Claude   ClaudeConfig   `mapstructure:"claude"`
}

type GeminiConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type VertexAIConfig struct {
	Project  string `mapstructure:"project"`
	Location string `mapstructure:"location"`
}

type ClaudeConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type SchedulerConfig struct {
	SyncBatchJobStatusCron string `mapstructure:"sync_batch_job_status_cron"`
}

// ResolveSQLConfig picks the SQL datastore type and its config.
// Defaults to postgres when datastores.db_type is omitted.
func (d DatastoresConfig) ResolveSQLConfig() (string, SQLConfig, error) {
	dbType := strings.ToLower(strings.TrimSpace(d.DBType))
	if dbType == "" {
		dbType = "postgres"
	}

	switch dbType {
	case "postgres":
		return dbType, d.Postgres, nil
	case "mysql":
		return dbType, d.MySQL, nil
	default:
		return "", SQLConfig{}, fmt.Errorf("datastores.db_type must be one of: postgres, mysql")
	}
}

func LoadConfig(configPaths ...string) (*Config, error) {
	if len(configPaths) == 0 {
		return nil, fmt.Errorf("no config paths provided")
	}

	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	for i, path := range configPaths {
		v.SetConfigFile(path)

		var err error
		if i == 0 {
			err = v.ReadInConfig()
		} else {
			err = v.MergeInConfig()
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
