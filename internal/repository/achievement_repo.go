package repository

import (
	"context"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
)

type AchievementRepository interface {
	GetAll(ctx context.Context) ([]models.Achievement, error)
	GetByID(ctx context.Context, id string) (*models.Achievement, error)
	Create(ctx context.Context, achievement *models.Achievement) error
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	Delete(ctx context.Context, id string) error
}

type achievementRepository struct{}

func NewAchievementRepository() AchievementRepository {
	return &achievementRepository{}
}

func (r *achievementRepository) GetAll(ctx context.Context) ([]models.Achievement, error) {
	var achievements []models.Achievement
	err := db.DB.WithContext(ctx).Order("created_at DESC").Find(&achievements).Error
	return achievements, err
}

func (r *achievementRepository) GetByID(ctx context.Context, id string) (*models.Achievement, error) {
	var achievement models.Achievement
	err := db.DB.WithContext(ctx).Where("id = ?", id).First(&achievement).Error
	if err != nil {
		return nil, err
	}
	return &achievement, nil
}

func (r *achievementRepository) Create(ctx context.Context, achievement *models.Achievement) error {
	return db.DB.WithContext(ctx).Create(achievement).Error
}

func (r *achievementRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	return db.DB.WithContext(ctx).Model(&models.Achievement{}).Where("id = ?", id).Updates(updates).Error
}

func (r *achievementRepository) Delete(ctx context.Context, id string) error {
	return db.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.Achievement{}).Error
}