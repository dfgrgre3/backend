package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"strings"
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
	ResolveVariant(ctx context.Context, experimentID, userID string) (string, error)
	TrackEvent(ctx context.Context, experimentID, userID, event string) error
}

type abTestingService struct {
	repo             repository.ABExperimentRepository
	assignmentSecret []byte
}

func NewABTestingService() ABTestingService {
	secret := strings.TrimSpace(os.Getenv("AB_TESTING_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
	}
	return newABTestingService(repository.NewABExperimentRepository(), []byte(secret))
}

func newABTestingService(repo repository.ABExperimentRepository, assignmentSecret []byte) *abTestingService {
	return &abTestingService{repo: repo, assignmentSecret: assignmentSecret}
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

func (s *abTestingService) loadExperimentVariants(ctx context.Context, experimentID string) (*models.ABExperiment, []models.Variant, error) {
	if strings.TrimSpace(experimentID) == "" {
		return nil, nil, errors.New("experiment ID is required")
	}

	experiment, err := s.repo.GetByID(ctx, experimentID)
	if err != nil {
		return nil, nil, err
	}
	if experiment == nil {
		return nil, nil, errors.New("experiment not found")
	}

	var variants []models.Variant
	if err := json.Unmarshal([]byte(experiment.Variants), &variants); err != nil {
		return nil, nil, err
	}
	if len(variants) < 2 {
		return nil, nil, errors.New("invalid experiment variants")
	}
	return experiment, variants, nil
}

func (s *abTestingService) resolveVariantForExperiment(experimentID, userID string) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errors.New("user ID is required")
	}
	if len(s.assignmentSecret) == 0 {
		return "", errors.New("A/B assignment secret is not configured")
	}

	// Domain-separated keyed assignment keeps allocation stable without exposing
	// a client-computable rule. The NUL separator prevents ambiguous concatenation.
	mac := hmac.New(sha256.New, s.assignmentSecret)
	_, _ = mac.Write([]byte("ab-variant-v1\x00"))
	_, _ = mac.Write([]byte(experimentID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userID))
	if mac.Sum(nil)[0]&1 == 0 {
		return "A", nil
	}
	return "B", nil
}

func (s *abTestingService) ResolveVariant(ctx context.Context, experimentID, userID string) (string, error) {
	if _, _, err := s.loadExperimentVariants(ctx, experimentID); err != nil {
		return "", err
	}
	return s.resolveVariantForExperiment(experimentID, userID)
}

func (s *abTestingService) TrackEvent(ctx context.Context, experimentID, userID, event string) error {
	experiment, variants, err := s.loadExperimentVariants(ctx, experimentID)
	if err != nil {
		return err
	}
	assigned, err := s.resolveVariantForExperiment(experimentID, userID)
	if err != nil {
		return err
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
