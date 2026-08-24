package courseservice

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ----------------------------
// Pricing
// ----------------------------

func (s *LmsService) AddPricing(courseID uuid.UUID, priceType models.PriceType, amount float64, currencyCode string, subDurationDays *int) (*models.LmsPricing, error) {
	decimalAmount := decimal.NewFromFloat(amount)
	p := &models.LmsPricing{
		CourseID:                 courseID,
		Type:                     priceType,
		Amount:                   decimalAmount,
		CurrencyCode:             currencyCode,
		SubscriptionDurationDays: subDurationDays,
		IsActive:                 true,
	}
	if err := s.repo.CreatePricing(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *LmsService) UpdatePricing(pricing *models.LmsPricing) (*models.LmsPricing, error) {
	if err := s.repo.UpdatePricing(pricing); err != nil {
		return nil, err
	}
	return pricing, nil
}

// CreatePricing inserts a fully-populated pricing row (including discount and
// subscription-plan fields, which AddPricing's narrower signature drops).
func (s *LmsService) CreatePricing(pricing *models.LmsPricing) (*models.LmsPricing, error) {
	if err := s.repo.CreatePricing(pricing); err != nil {
		return nil, err
	}
	return pricing, nil
}

func (s *LmsService) ListPricings(courseID uuid.UUID) ([]models.LmsPricing, error) {
	return s.repo.ListPricingsByCourseID(courseID)
}
