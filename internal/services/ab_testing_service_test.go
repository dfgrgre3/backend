package services

import (
	"context"
	"errors"
	"testing"

	"thanawy-backend/internal/models"
)

type fakeABExperimentRepository struct {
	experiment *models.ABExperiment
	err        error
}

func (f *fakeABExperimentRepository) Create(context.Context, *models.ABExperiment) error { return nil }
func (f *fakeABExperimentRepository) GetByID(context.Context, string) (*models.ABExperiment, error) {
	return f.experiment, f.err
}
func (f *fakeABExperimentRepository) GetAll(context.Context) ([]*models.ABExperiment, error) {
	return nil, nil
}
func (f *fakeABExperimentRepository) Update(context.Context, *models.ABExperiment) error { return nil }
func (f *fakeABExperimentRepository) Delete(context.Context, string) error               { return nil }

func validABExperiment() *models.ABExperiment {
	return &models.ABExperiment{
		ID:       "experiment-1",
		Variants: `[{"name":"A"},{"name":"B"}]`,
	}
}

func TestResolveVariantIsStableAndServerKeyed(t *testing.T) {
	service := newABTestingService(
		&fakeABExperimentRepository{experiment: validABExperiment()},
		[]byte("server-only-assignment-secret"),
	)

	first, err := service.ResolveVariant(context.Background(), "experiment-1", "user-1")
	if err != nil {
		t.Fatalf("ResolveVariant returned an error: %v", err)
	}
	second, err := service.ResolveVariant(context.Background(), "experiment-1", "user-1")
	if err != nil {
		t.Fatalf("ResolveVariant returned an error: %v", err)
	}
	if first != second {
		t.Fatalf("expected stable assignment, got %q then %q", first, second)
	}
	if first != "A" && first != "B" {
		t.Fatalf("expected A or B, got %q", first)
	}
}

func TestResolveVariantFailsClosedWithoutSecret(t *testing.T) {
	service := newABTestingService(&fakeABExperimentRepository{experiment: validABExperiment()}, nil)

	_, err := service.ResolveVariant(context.Background(), "experiment-1", "user-1")
	if err == nil {
		t.Fatal("expected missing assignment secret to fail closed")
	}
}

func TestResolveVariantValidatesExperiment(t *testing.T) {
	service := newABTestingService(
		&fakeABExperimentRepository{err: errors.New("not found")},
		[]byte("server-only-assignment-secret"),
	)

	if _, err := service.ResolveVariant(context.Background(), "missing", "user-1"); err == nil {
		t.Fatal("expected a missing experiment to be rejected")
	}
}
