package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ----------------------------
// Section CRUD
// ----------------------------

func (r *LmsRepository) CreateSection(section *models.LmsSection) error {
	return r.db.Create(section).Error
}

func (r *LmsRepository) GetSectionByID(id uuid.UUID) (*models.LmsSection, error) {
	var s models.LmsSection
	err := r.db.Preload("Lessons").First(&s, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *LmsRepository) UpdateSection(section *models.LmsSection) error {
	return r.db.Save(section).Error
}

func (r *LmsRepository) DeleteSection(id uuid.UUID) error {
	return r.db.Delete(&models.LmsSection{}, "id = ?", id).Error
}

func (r *LmsRepository) ListSectionsByCourseID(courseID uuid.UUID) ([]models.LmsSection, error) {
	var sections []models.LmsSection
	err := r.db.Preload("Lessons", func(db *gorm.DB) *gorm.DB {
		return db.Order("order_index ASC")
	}).Where("course_id = ?", courseID).Order("order_index ASC").Find(&sections).Error
	return sections, err
}

// ReorderSections persists a new order_index for each section id in
// sectionIDs, in the order given (0-based), all in one transaction.
func (r *LmsRepository) ReorderSections(courseID uuid.UUID, sectionIDs []uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, sectionID := range sectionIDs {
			if err := tx.Model(&models.LmsSection{}).
				Where("id = ? AND course_id = ?", sectionID, courseID).
				Update("order_index", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
