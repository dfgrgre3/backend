package services

import (
	"errors"
	"time"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BundleService handles business logic for course bundles
type BundleService struct{}

// NewBundleService creates a new BundleService
func NewBundleService() *BundleService {
	return &BundleService{}
}

// BundlePurchaseResult represents the result of purchasing a bundle
type BundlePurchaseResult struct {
	BundleID     string     `json:"bundleId"`
	Enrollments  []string   `json:"enrollments"` // IDs of created enrollments
	PricePaid    float64    `json:"pricePaid"`
	Currency     string     `json:"currency"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	TotalCourses int        `json:"totalCourses"`
}

// PurchaseBundle enrolls a user in all courses of a bundle
func (s *BundleService) PurchaseBundle(userID, bundleID string, pricePaid float64) (*BundlePurchaseResult, error) {
	var bundle models.CourseBundle
	if err := db.DB.Where("id = ? AND is_active = ? AND deleted_at IS NULL", bundleID, true).First(&bundle).Error; err != nil {
		return nil, errors.New("bundle not found or inactive")
	}

	if len(bundle.CourseIDs) == 0 {
		return nil, errors.New("bundle has no courses")
	}

	// Check if user already has this bundle
	var existing models.BundleEnrollment
	if db.DB.Where("user_id = ? AND bundle_id = ? AND status = ?", userID, bundleID, "ACTIVE").First(&existing).Error == nil {
		return nil, errors.New("user already owns this bundle")
	}

	result := &BundlePurchaseResult{
		BundleID:     bundleID,
		Enrollments:  make([]string, 0),
		PricePaid:    pricePaid,
		Currency:     string(bundle.Currency),
		TotalCourses: len(bundle.CourseIDs),
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Create bundle enrollment
		enrollment := models.BundleEnrollment{
			ID:         uuid.New().String(),
			UserID:     userID,
			BundleID:   bundleID,
			PricePaid:  pricePaid,
			Currency:   bundle.Currency,
			Status:     "ACTIVE",
			EnrolledAt: time.Now().UTC(),
		}
		if err := tx.Create(&enrollment).Error; err != nil {
			return err
		}

		// Enroll user in each course
		for _, courseID := range bundle.CourseIDs {
			// Check if already enrolled
			var existingEnrollment models.Enrollment
			if tx.Where("user_id = ? AND subject_id = ?", userID, courseID).First(&existingEnrollment).Error == nil {
				// Already enrolled - skip
				continue
			}

			newEnrollment := models.Enrollment{
				ID:        uuid.New().String(),
				UserID:    userID,
				SubjectID: courseID,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if err := tx.Create(&newEnrollment).Error; err != nil {
				return err
			}

			result.Enrollments = append(result.Enrollments, newEnrollment.ID)
		}

		// Update bundle stats
		return tx.Model(&bundle).Updates(map[string]interface{}{
			"total_students": gorm.Expr("total_students + 1"),
			"total_revenue":  gorm.Expr("total_revenue + ?", pricePaid),
			"updated_at":     time.Now().UTC(),
		}).Error
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// CheckBundleAccess checks if a user has access to a course through any bundle
func (s *BundleService) CheckBundleAccess(userID, courseID string) (*models.BundleEnrollment, error) {
	var enrollment models.BundleEnrollment
	err := db.DB.
		Where("user_id = ? AND bundle_id IN (SELECT bundle_id FROM bundle_courses WHERE course_id = ?)",
			userID, courseID).
		Where("status = ?", "ACTIVE").
		First(&enrollment).Error

	if err != nil {
		return nil, nil // No bundle access
	}
	return &enrollment, nil
}

// GetUserBundles returns all bundles a user has purchased
func (s *BundleService) GetUserBundles(userID string) ([]models.CourseBundle, error) {
	var enrollments []models.BundleEnrollment
	if err := db.DB.Where("user_id = ? AND status = ?", userID, "ACTIVE").
		Find(&enrollments).Error; err != nil {
		return nil, err
	}

	if len(enrollments) == 0 {
		return []models.CourseBundle{}, nil
	}

	bundleIDs := make([]string, len(enrollments))
	for i, e := range enrollments {
		bundleIDs[i] = e.BundleID
	}

	var bundles []models.CourseBundle
	if err := db.DB.Where("id IN ? AND is_active = ?", bundleIDs, true).
		Find(&bundles).Error; err != nil {
		return nil, err
	}

	return bundles, nil
}

// GetBundleWithCourses returns a bundle with its full course details
func (s *BundleService) GetBundleWithCourses(bundleID string) (*models.CourseBundle, error) {
	var bundle models.CourseBundle
	if err := db.DB.Where("id = ? AND deleted_at IS NULL", bundleID).First(&bundle).Error; err != nil {
		return nil, errors.New("bundle not found")
	}

	if len(bundle.CourseIDs) > 0 {
		var courses []models.Subject
		db.DB.Where("id = ANY(?)", bundle.CourseIDs).
			Select("id, title, title_ar, description, thumbnail, price, language, is_published, is_active").
			Find(&courses)
		bundle.Courses = courses
	}

	return &bundle, nil
}

// CalculateBundleSavings calculates the savings from buying a bundle vs individual prices
func (s *BundleService) CalculateBundleSavings(bundleID string) (float64, float64, error) {
	var bundle models.CourseBundle
	if err := db.DB.Where("id = ?", bundleID).First(&bundle).Error; err != nil {
		return 0, 0, errors.New("bundle not found")
	}

	if len(bundle.CourseIDs) == 0 {
		return 0, 0, nil
	}

	// Get total of individual prices
	var totalIndividual float64
	for _, courseID := range bundle.CourseIDs {
		var pricing models.CoursePricing
		if db.DB.Where("subject_id = ?", courseID).First(&pricing).Error == nil {
			totalIndividual += pricing.EffectivePrice()
		}
	}

	savings := totalIndividual - bundle.Price
	if savings < 0 {
		savings = 0
	}

	return totalIndividual, savings, nil
}
