package repositories

import (
	"context"
)

// Pricing operations
func (r *GormRepository) CreatePricing(ctx context.Context, pricing *Pricing) error {
	return r.repo.CreatePricing(r.toModelPricing(pricing))
}

func (r *GormRepository) GetPricing(ctx context.Context, courseID string) (*Pricing, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	modelPricings, err := r.repo.ListPricingsByCourseID(courseUUID)
	if err != nil || len(modelPricings) == 0 {
		return nil, err
	}
	return r.toDomainPricing(&modelPricings[0]), nil
}

func (r *GormRepository) UpdatePricing(ctx context.Context, pricing *Pricing) error {
	return r.repo.UpdatePricing(r.toModelPricing(pricing))
}

func (r *GormRepository) DeletePricing(ctx context.Context, courseID string) error {
	return nil
}
