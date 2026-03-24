package repository

import (
	"context"
	"errors"

	"github.com/ideagate/aigateway-core/internal/aigateway/models"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

type Repository interface {
	CreateBatchJob(ctx context.Context, job *models.BatchJob) error
	// GetActiveBatchJobs returns all batch jobs whose status is pending,
	// submitted, or processing — i.e. jobs that still need to be polled.
	GetActiveBatchJobs(ctx context.Context) ([]*models.BatchJob, error)
	// UpdateBatchJob persists changes to an existing batch job row.
	UpdateBatchJob(ctx context.Context, job *models.BatchJob) error

	// UpsertTokenJobs creates or updates token count rows for a job.
	// On primary-key conflict the token_count column is overwritten.
	UpsertTokenJobs(ctx context.Context, jobs []*models.TokenJob) error

	// UpsertPromptConfig creates or fully replaces a prompt config row.
	UpsertPromptConfig(ctx context.Context, cfg *models.PromptConfig) error
	// GetPromptConfig returns a prompt config by ID.
	// Returns ErrNotFound when no row with that ID exists.
	GetPromptConfig(ctx context.Context, id string) (*models.PromptConfig, error)
	// ListPromptConfigs returns all prompt config rows (unfiltered).
	ListPromptConfigs(ctx context.Context) ([]*models.PromptConfig, error)
	// DeletePromptConfig removes a prompt config by ID.
	// Returns ErrNotFound when no row with that ID exists.
	DeletePromptConfig(ctx context.Context, id string) error
}
