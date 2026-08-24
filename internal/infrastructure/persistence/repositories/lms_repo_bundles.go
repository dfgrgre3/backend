package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Bundle CRUD
// ----------------------------

func (r *LmsRepository) CreateBundle(b *models.LmsBundle) error {
	return r.db.Create(b).Error
}

func (r *LmsRepository) GetBundleByID(id uuid.UUID) (*models.LmsBundle, error) {
	var b models.LmsBundle
	err := r.db.Preload("Courses").First(&b, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *LmsRepository) UpdateBundle(b *models.LmsBundle) error {
	return r.db.Save(b).Error
}

func (r *LmsRepository) DeleteBundle(id uuid.UUID) error {
	return r.db.Delete(&models.LmsBundle{}, "id = ?", id).Error
}

func (r *LmsRepository) ListBundles(page, pageSize int) ([]models.LmsBundle, int64, error) {
	var bundles []models.LmsBundle
	var total int64
	q := r.db.Model(&models.LmsBundle{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&bundles).Error; err != nil {
		return nil, 0, err
	}
	return bundles, total, nil
}

// AddCourseToBundle links a course to a bundle.
func (r *LmsRepository) AddCourseToBundle(bundleID, courseID uuid.UUID) error {
	return r.db.Exec("INSERT INTO \"LmsBundleCourse\" (bundle_id, course_id) VALUES (?, ?) ON CONFLICT DO NOTHING", bundleID, courseID).Error
}

// RemoveCourseFromBundle unlinks a course from a bundle.
func (r *LmsRepository) RemoveCourseFromBundle(bundleID, courseID uuid.UUID) error {
	return r.db.Exec("DELETE FROM \"LmsBundleCourse\" WHERE bundle_id = ? AND course_id = ?", bundleID, courseID).Error
}
