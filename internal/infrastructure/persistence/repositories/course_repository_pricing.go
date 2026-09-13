package repositories

import (
	"context"
	models "thanawy-backend/internal/domain/common"
)

// Pricing operations
func (r *GormRepository) CreatePricing(ctx context.Context, pricing *Pricing) error {
	return r.repo.db.WithContext(ctx).Create(r.toModelPricing(pricing)).Error
}

func (r *GormRepository) GetPricing(ctx context.Context, courseID string) (*Pricing, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	var modelPricings []models.LmsPricing
	err = r.repo.db.WithContext(ctx).Where("course_id = ? AND deleted_at IS NULL", courseUUID).Order("created_at DESC").Find(&modelPricings).Error
	if err != nil || len(modelPricings) == 0 {
		return nil, err
	}
	return r.toDomainPricing(&modelPricings[0]), nil
}

func (r *GormRepository) UpdatePricing(ctx context.Context, pricing *Pricing) error {
	return r.repo.db.WithContext(ctx).Save(r.toModelPricing(pricing)).Error
}

func (r *GormRepository) DeletePricing(ctx context.Context, courseID string) error {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return err
	}
	return r.repo.db.WithContext(ctx).Where("course_id = ?", courseUUID).Delete(&models.LmsPricing{}).Error
}
