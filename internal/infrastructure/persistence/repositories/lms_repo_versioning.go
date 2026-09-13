package repositories

import (
	"encoding/json"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ----------------------------
// Changelog & Versions
// ----------------------------

func (r *LmsRepository) AddChangelog(c *models.LmsCourseChangelog) error {
	return r.db.Create(c).Error
}

func (r *LmsRepository) ListChangelogs(courseID uuid.UUID) ([]models.LmsCourseChangelog, error) {
	var logs []models.LmsCourseChangelog
	err := r.db.Where("course_id = ?", courseID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func (r *LmsRepository) CreateVersion(v *models.LmsCourseVersion) error {
	return r.db.Create(v).Error
}

func (r *LmsRepository) ListVersions(courseID uuid.UUID) ([]models.LmsCourseVersion, error) {
	var versions []models.LmsCourseVersion
	err := r.db.Where("course_id = ?", courseID).Order("version_number DESC").Find(&versions).Error
	return versions, err
}

func (r *LmsRepository) GetVersion(courseID uuid.UUID, versionNumber int) (*models.LmsCourseVersion, error) {
	var v models.LmsCourseVersion
	err := r.db.Where("course_id = ? AND version_number = ?", courseID, versionNumber).First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// SnapshotCourse creates a JSON snapshot of the course for versioning.
func (r *LmsRepository) SnapshotCourse(courseID uuid.UUID) ([]byte, error) {
	c, err := r.GetCourseByID(courseID)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal course snapshot: %w", err)
	}
	return data, nil
}

// ----------------------------
// Clone & Version helpers
// ----------------------------

// CloneCourse duplicates a course with all sections/lessons into a new draft.
func (r *LmsRepository) CloneCourse(srcID uuid.UUID, newTitle string) (*models.LmsCourse, error) {
	src, err := r.GetCourseByID(srcID)
	if err != nil {
		return nil, err
	}
	tx := r.db.Begin()

	newCourse := *src
	newCourse.ID = uuid.New()
	newCourse.Title = newTitle
	newCourse.Slug = fmt.Sprintf("%s-copy-%s", src.Slug, uuid.New().String()[:8])
	newCourse.Status = models.CourseStatusDraft
	newCourse.Version = 1
	newCourse.CreatedAt = time.Now()
	newCourse.UpdatedAt = time.Now()
	newCourse.DeletedAt = gorm.DeletedAt{}

	if err := tx.Create(&newCourse).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	for _, s := range src.Sections {
		newSection := s
		newSection.ID = uuid.New()
		newSection.CourseID = newCourse.ID
		newSection.DeletedAt = gorm.DeletedAt{}
		if err := tx.Create(&newSection).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		for _, l := range s.Lessons {
			newLesson := l
			newLesson.ID = uuid.New()
			newLesson.SectionID = newSection.ID
			newLesson.DeletedAt = gorm.DeletedAt{}
			if err := tx.Create(&newLesson).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newCourse, nil
}

// RestoreVersion restores a course to a specific version
func (r *LmsRepository) RestoreVersion(courseID string, versionNumber int, userID string) (*models.LmsCourse, error) {
	courseUUID, err := uuid.Parse(courseID)
	if err != nil {
		return nil, err
	}

	version, err := r.GetVersion(courseUUID, versionNumber)
	if err != nil {
		return nil, err
	}

	// Get current course
	course, err := r.GetCourseByID(courseUUID)
	if err != nil {
		return nil, err
	}

	var restored models.LmsCourse
	if err := json.Unmarshal(version.Snapshot, &restored); err != nil {
		return nil, fmt.Errorf("decode course version %d: %w", versionNumber, err)
	}
	// Keep identity and immutable timestamps from the live row. Associations
	// are intentionally not saved here; their snapshots are informational until
	// a dedicated curriculum restore operation is requested.
	restored.ID = course.ID
	restored.CreatedAt = course.CreatedAt
	restored.UpdatedAt = time.Now()
	restored.Version = course.Version + 1
	restored.DeletedAt = course.DeletedAt
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.LmsCourse{}).Where("id = ?", courseUUID).Updates(map[string]interface{}{
			"title": restored.Title, "slug": restored.Slug, "short_description": restored.ShortDescription,
			"long_description": restored.LongDescription, "cover_image_url": restored.CoverImageURL,
			"promo_video_url": restored.PromoVideoURL, "status": restored.Status, "level": restored.Level,
			"language": restored.Language, "estimated_duration_mins": restored.EstimatedDurationMins,
			"has_certificate": restored.HasCertificate, "certificate_template": restored.CertificateTemplate,
			"max_students": restored.MaxStudents, "version": restored.Version, "is_featured": restored.IsFeatured,
			"is_trending": restored.IsTrending, "is_new": restored.IsNew, "new_from": restored.NewFrom,
			"new_until": restored.NewUntil, "seo_title": restored.SEOTitle, "seo_description": restored.SEODescription,
			"seo_keywords": restored.SEOKeywords, "prerequisites_text": restored.PrerequisitesText,
			"target_audience": restored.TargetAudience, "learning_outcomes": restored.LearningOutcomes,
			"primary_instructor_id": restored.PrimaryInstructorID, "available_from": restored.AvailableFrom,
			"available_until": restored.AvailableUntil, "updated_at": restored.UpdatedAt,
		}).Error; err != nil {
			return err
		}
		userUUID, parseErr := uuid.Parse(userID)
		if parseErr != nil {
			return parseErr
		}
		message := fmt.Sprintf("Restored to version %d", versionNumber)
		return tx.Create(&models.LmsCourseChangelog{CourseID: courseUUID, UserID: userUUID, Field: "version_restore", NewValue: &message}).Error
	}); err != nil {
		return nil, err
	}
	return r.GetCourseByID(courseUUID)
}

// GetChangelogByCourseID returns the changelog for a course (string ID version)
func (r *LmsRepository) GetChangelogByCourseID(courseID string) ([]*models.LmsCourseChangelog, error) {
	courseUUID, err := uuid.Parse(courseID)
	if err != nil {
		return nil, err
	}

	logs, err := r.ListChangelogs(courseUUID)
	if err != nil {
		return nil, err
	}

	// Convert []models.LmsCourseChangelog to []*models.LmsCourseChangelog
	result := make([]*models.LmsCourseChangelog, len(logs))
	for i := range logs {
		result[i] = &logs[i]
	}
	return result, nil
}
