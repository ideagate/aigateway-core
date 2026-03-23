package repository

import (
	"context"

	"github.com/ideagate/aigateway-core/internal/aigateway/models"
)

type Repository interface {
	CreateBatchJob(ctx context.Context, job *models.BatchJob) error
	// GetActiveBatchJobs returns all batch jobs whose status is pending,
	// submitted, or processing — i.e. jobs that still need to be polled.
	GetActiveBatchJobs(ctx context.Context) ([]*models.BatchJob, error)
	// UpdateBatchJob persists changes to an existing batch job row.
	UpdateBatchJob(ctx context.Context, job *models.BatchJob) error
}
