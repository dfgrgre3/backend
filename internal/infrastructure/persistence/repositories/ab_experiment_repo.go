package repositories

import (
	"context"
	models "thanawy-backend/internal/domain/common"
	"time"

	db "thanawy-backend/internal/infrastructure/database"
)

type ABExperimentRepository interface {
	Create(ctx context.Context, experiment *models.ABExperiment) error
	GetByID(ctx context.Context, id string) (*models.ABExperiment, error)
	GetAll(ctx context.Context) ([]*models.ABExperiment, error)
	Update(ctx context.Context, experiment *models.ABExperiment) error
	Delete(ctx context.Context, id string) error
}

type abExperimentRepository struct{}

func NewABExperimentRepository() ABExperimentRepository {
	return &abExperimentRepository{}
}

func (r *abExperimentRepository) Create(ctx context.Context, experiment *models.ABExperiment) error {
	return db.DB.WithContext(ctx).Create(experiment).Error
}

func (r *abExperimentRepository) GetByID(ctx context.Context, id string) (*models.ABExperiment, error) {
	var experiment models.ABExperiment
	err := db.DB.WithContext(ctx).Where("id = ?", id).First(&experiment).Error
	if err != nil {
		return nil, err
	}
	return &experiment, nil
}

func (r *abExperimentRepository) GetAll(ctx context.Context) ([]*models.ABExperiment, error) {
	var experiments []*models.ABExperiment
	err := db.DB.WithContext(ctx).Order("created_at DESC").Find(&experiments).Error
	return experiments, err
}

func (r *abExperimentRepository) Update(ctx context.Context, experiment *models.ABExperiment) error {
	experiment.UpdatedAt = time.Now()
	return db.DB.WithContext(ctx).Save(experiment).Error
}

func (r *abExperimentRepository) Delete(ctx context.Context, id string) error {
	return db.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.ABExperiment{}).Error
}
