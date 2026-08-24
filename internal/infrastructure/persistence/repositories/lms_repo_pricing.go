package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Pricing CRUD
// ----------------------------

func (r *LmsRepository) CreatePricing(p *models.LmsPricing) error {
	return r.db.Create(p).Error
}

func (r *LmsRepository) ListPricingsByCourseID(courseID uuid.UUID) ([]models.LmsPricing, error) {
	var pricings []models.LmsPricing
	err := r.db.Where("course_id = ? AND deleted_at IS NULL", courseID).Find(&pricings).Error
	return pricings, err
}

func (r *LmsRepository) DeletePricing(id uuid.UUID) error {
	return r.db.Delete(&models.LmsPricing{}, "id = ?", id).Error
}

func (r *LmsRepository) UpdatePricing(p *models.LmsPricing) error {
	return r.db.Save(p).Error
}
