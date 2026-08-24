package courseservice

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ----------------------------
// Bundles
// ----------------------------

func (s *LmsService) CreateBundle(title, slug string, price float64, currencyCode string) (*models.LmsBundle, error) {
	decimalPrice := decimal.NewFromFloat(price)
	b := &models.LmsBundle{
		Title:        title,
		Slug:         slug,
		Price:        decimalPrice,
		CurrencyCode: currencyCode,
		IsActive:     true,
	}
	if err := s.repo.CreateBundle(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *LmsService) GetBundle(id uuid.UUID) (*models.LmsBundle, error) {
	return s.repo.GetBundleByID(id)
}

func (s *LmsService) ListBundles(page, pageSize int) ([]models.LmsBundle, int64, error) {
	return s.repo.ListBundles(page, pageSize)
}

func (s *LmsService) UpdateBundle(b *models.LmsBundle) error {
	return s.repo.UpdateBundle(b)
}

func (s *LmsService) DeleteBundle(id uuid.UUID) error {
	return s.repo.DeleteBundle(id)
}

func (s *LmsService) AddCourseToBundle(bundleID, courseID uuid.UUID) error {
	return s.repo.AddCourseToBundle(bundleID, courseID)
}

func (s *LmsService) RemoveCourseFromBundle(bundleID, courseID uuid.UUID) error {
	return s.repo.RemoveCourseFromBundle(bundleID, courseID)
}
