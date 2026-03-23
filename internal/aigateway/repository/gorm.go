package repository

import (
	"context"

	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	"gorm.io/gorm"
)

type gormRepository struct {
	db *gorm.DB
}

// New constructs a Repository backed by the given GORM database.
func New(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) CreateBatchJob(ctx context.Context, job *models.BatchJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *gormRepository) GetActiveBatchJobs(ctx context.Context) ([]*models.BatchJob, error) {
	activeStatuses := []string{
		models.BatchJobStatusPending,
		models.BatchJobStatusSubmitted,
		models.BatchJobStatusProcessing,
	}
	var jobs []*models.BatchJob
	err := r.db.WithContext(ctx).
		Where("status IN ?", activeStatuses).
		Find(&jobs).Error
	return jobs, err
}

func (r *gormRepository) UpdateBatchJob(ctx context.Context, job *models.BatchJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

