package aigatewaycore

import (
	"context"
	"errors"
	"fmt"

	aigatewaymodels "github.com/ideagate/aigateway-core/internal/aigateway/models"
	aigatewayproviders "github.com/ideagate/aigateway-core/internal/aigateway/providers"
	aigatewayrepository "github.com/ideagate/aigateway-core/internal/aigateway/repository"
	aigatewayusecase "github.com/ideagate/aigateway-core/internal/aigateway/usecase"
	"github.com/ideagate/aigateway-core/internal/platform/config"
	platformdb "github.com/ideagate/aigateway-core/internal/platform/db"
	"github.com/ideagate/aigateway-core/models"
	"gorm.io/gorm"
)

type Client interface {
	Migrate(ctx context.Context) error
	CheckMigration(ctx context.Context) error
	ChatCompletion(ctx context.Context, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)
}

func NewClient(cfg *Config) (Client, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	// initialize provider
	if cfg.Providers == nil || cfg.Providers.Google == nil {
		return nil, errors.New("no google provider configured")
	}
	provider, err := aigatewayproviders.New(config.GeminiConfig{
		APIKey: cfg.Providers.Google.ApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}

	// initialize datastore (mysql only for now)
	if cfg.DataStores == nil || cfg.DataStores.Mysql == nil {
		return nil, errors.New("no data stores configured")
	}
	db, err := platformdb.Open(platformdb.DBTypeMySQL, config.SQLConfig{
		Host:     cfg.DataStores.Mysql.Host,
		Port:     cfg.DataStores.Mysql.Port,
		Username: cfg.DataStores.Mysql.User,
		Password: cfg.DataStores.Mysql.Password,
		DBName:   cfg.DataStores.Mysql.Database,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if cfg.DataStores.Redis == nil {
		return nil, errors.New("no redis configured")
	}
	redisClient, err := platformdb.NewRedis(config.RedisConfig{
		Host:     cfg.DataStores.Redis.Host,
		Port:     cfg.DataStores.Redis.Port,
		Password: cfg.DataStores.Redis.Password,
		Database: cfg.DataStores.Redis.Database,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	repo := aigatewayrepository.New(db)
	repoLock := aigatewayrepository.NewDistributedLock(redisClient)

	usecase := aigatewayusecase.New(provider, repo, repoLock)

	return &client{
		db:       db,
		provider: provider,
		usecase:  usecase,
	}, nil
}

type client struct {
	db       *gorm.DB
	provider aigatewayproviders.Provider
	usecase  aigatewayusecase.Usecase
}

func (c *client) Migrate(_ context.Context) error {
	migrationModels := aigatewaymodels.MigrationModels()
	return c.db.AutoMigrate(migrationModels...)
}

func (c *client) CheckMigration(_ context.Context) error {
	for _, model := range aigatewaymodels.MigrationModels() {
		if !c.db.Migrator().HasTable(model) {
			return fmt.Errorf("required table for %T does not exist", model)
		}
	}

	return nil
}

func (c *client) ChatCompletion(ctx context.Context, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	return c.provider.ChatCompletion(ctx, request)
}
