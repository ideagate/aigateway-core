package repository

import (
	"context"

	"github.com/ideagate/aigateway-core/internal/aigateway/models"
)

type Repository interface {
	CreateBatchJob(ctx context.Context, job *models.BatchJob) error
}
