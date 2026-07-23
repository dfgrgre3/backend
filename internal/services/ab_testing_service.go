package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"
)

type ABTestingService interface {
	CreateExperiment(ctx context.Context, name, description, status string, variantsJSON string, trafficPct int) (*models.ABExperiment, error)
	GetExperiment(ctx context.Context, id string) (*models.ABExperiment, error)
	ListExperiments(ctx context.Context) ([]*models.ABExperiment, error)
	UpdateExperiment(ctx context.Context, experiment *models.ABExperiment) error
	DeleteExperiment(ctx context.Context, id string) error
	TrackEvent(ctx context.Context, experimentID, userID, event string) error
}

type abTestingService struct {
	repo repository.ABExperimentRepository
}

func NewABTestingService() ABTestingService {
	return &abTestingService{
		repo: repository.NewABExperimentRepository(),
	}
}

func (s *abTestingService) CreateExperiment(ctx context.Context, name, description, status string, variantsJSON string, trafficPct int) (*models.ABExperiment, error) {
	experiment := &models.ABExperiment{
		Name:        name,
		Description: description,
		Status:      status,
		Variants:    variantsJSON,
		TrafficPct:  trafficPct,
	}
	if err := s.repo.Create(ctx, experiment); err != nil {
		return nil, err
	}
	return experiment, nil
}

func (s *abTestingService) GetExperiment(ctx context.Context, id string) (*models.ABExperiment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *abTestingService) ListExperiments(ctx context.Context) ([]*models.ABExperiment, error) {
	return s.repo.GetAll(ctx)
}

func (s *abTestingService) UpdateExperiment(ctx context.Context, experiment *models.ABExperiment) error {
	return s.repo.Update(ctx, experiment)
}

func (s *abTestingService) DeleteExperiment(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *abTestingService) TrackEvent(ctx context.Context, experimentID, userID, event string) error {
	experiment, err := s.repo.GetByID(ctx, experimentID)
	if err != nil {
		return err
	}
	if experiment == nil {
		return errors.New("experiment not found")
	}

	var variants []models.Variant
	if err := json.Unmarshal([]byte(experiment.Variants), &variants); err != nil {
		return err
	}
	if len(variants) < 2 {
		return errors.New("invalid experiment variants")
	}

	// Simple deterministic assignment
	assigned := "A"
	if (len(experimentID)+len(userID))%2 == 0 {
		assigned = "B"
	}
	idx := 0
	if assigned == "B" {
		idx = 1
	}

	variants[idx].Views++
	if event == "complete" {
		variants[idx].CompletionRate = 100
	}

	out, err := json.Marshal(variants)
	if err != nil {
		return err
	}
	experiment.Variants = string(out)
	experiment.UpdatedAt = time.Now()
	return s.repo.Update(ctx, experiment)
}