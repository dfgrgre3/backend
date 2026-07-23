package services

import (
	"context"

	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"
)

type AchievementService interface {
	GetAllAchievements(ctx context.Context) ([]models.Achievement, error)
	GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error)
	CreateAchievement(ctx context.Context, achievement *models.Achievement) error
	UpdateAchievement(ctx context.Context, id string, updates map[string]interface{}) (*models.Achievement, error)
	DeleteAchievement(ctx context.Context, id string) error
}

type achievementService struct {
	achievementRepo repository.AchievementRepository
}

func NewAchievementService(achievementRepo repository.AchievementRepository) AchievementService {
	return &achievementService{
		achievementRepo: achievementRepo,
	}
}

func (s *achievementService) GetAllAchievements(ctx context.Context) ([]models.Achievement, error) {
	return s.achievementRepo.GetAll(ctx)
}

func (s *achievementService) GetAchievementByID(ctx context.Context, id string) (*models.Achievement, error) {
	return s.achievementRepo.GetByID(ctx, id)
}

func (s *achievementService) CreateAchievement(ctx context.Context, achievement *models.Achievement) error {
	return s.achievementRepo.Create(ctx, achievement)
}

func (s *achievementService) UpdateAchievement(ctx context.Context, id string, updates map[string]interface{}) (*models.Achievement, error) {
	err := s.achievementRepo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	return s.achievementRepo.GetByID(ctx, id)
}

func (s *achievementService) DeleteAchievement(ctx context.Context, id string) error {
	return s.achievementRepo.Delete(ctx, id)
}