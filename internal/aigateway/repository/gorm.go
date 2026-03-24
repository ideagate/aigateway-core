package repository

import (
	"context"
	"errors"

	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *gormRepository) UpsertTokenJobs(ctx context.Context, jobs []*models.TokenJob) error {
	if len(jobs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "job_type"}, {Name: "job_id"}, {Name: "token_type"}},
			DoUpdates: clause.AssignmentColumns([]string{"token_count"}),
		}).
		Create(jobs).Error
}

func (r *gormRepository) UpsertPromptConfig(ctx context.Context, cfg *models.PromptConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

func (r *gormRepository) GetPromptConfig(ctx context.Context, id string) (*models.PromptConfig, error) {
	var cfg models.PromptConfig
	err := r.db.WithContext(ctx).First(&cfg, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *gormRepository) ListPromptConfigs(ctx context.Context) ([]*models.PromptConfig, error) {
	var cfgs []*models.PromptConfig
	err := r.db.WithContext(ctx).Find(&cfgs).Error
	return cfgs, err
}

func (r *gormRepository) DeletePromptConfig(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.PromptConfig{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
