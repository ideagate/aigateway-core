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
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
}

type ProvidersConfig struct {
	Gemini   GeminiConfig   `mapstructure:"gemini"`
	VertexAI VertexAIConfig `mapstructure:"vertexai"`
}

type GeminiConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type VertexAIConfig struct {
	Project  string `mapstructure:"project"`
	Location string `mapstructure:"location"`
}

type SchedulerConfig struct {
	SyncBatchJobStatusCron string `mapstructure:"sync_batch_job_status_cron"`
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
