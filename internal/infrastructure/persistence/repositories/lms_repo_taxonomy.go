package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Categories & Tags
// ----------------------------

func (r *LmsRepository) CreateCategory(c *models.LmsCategory) error {
	return r.db.Create(c).Error
}

func (r *LmsRepository) ListCategories() ([]models.LmsCategory, error) {
	var categories []models.LmsCategory
	err := r.db.Find(&categories).Error
	return categories, err
}

func (r *LmsRepository) CreateTag(t *models.LmsTag) error {
	return r.db.Create(t).Error
}

func (r *LmsRepository) ListTags() ([]models.LmsTag, error) {
	var tags []models.LmsTag
	err := r.db.Find(&tags).Error
	return tags, err
}

func (r *LmsRepository) SetCourseCategories(courseID uuid.UUID, categoryIDs []uuid.UUID) error {
	tx := r.db.Begin()
	if err := tx.Where("course_id = ?", courseID).Delete(&models.LmsCourseCategory{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, catID := range categoryIDs {
		if err := tx.Exec("INSERT INTO \"LmsCourseCategory\" (course_id, category_id) VALUES (?, ?) ON CONFLICT DO NOTHING", courseID, catID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *LmsRepository) SetCourseTags(courseID uuid.UUID, tagIDs []uuid.UUID) error {
	tx := r.db.Begin()
	if err := tx.Where("course_id = ?", courseID).Delete(&models.LmsCourseTag{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, tagID := range tagIDs {
		if err := tx.Exec("INSERT INTO \"LmsCourseTag\" (course_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING", courseID, tagID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
