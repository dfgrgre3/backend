package courseservice

import (
	"context"
	"errors"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrCertificateTemplateNotFound is returned when CertificateTemplate is set
// to an id that doesn't match any row in LmsCertificateTemplate. The column
// itself has no DB-level FK constraint (see LmsCertificateTemplate's doc
// comment), so this is the only place that catches a bad id before it's saved.
var ErrCertificateTemplateNotFound = errors.New("certificate template not found")

// UpdateCourseHandler handles course update commands
type UpdateCourseHandler struct {
	db *gorm.DB
}

// NewUpdateCourseHandler creates a new UpdateCourseHandler
func NewUpdateCourseHandler(db *gorm.DB) *UpdateCourseHandler {
	return &UpdateCourseHandler{db: db}
}

// Handle handles the update course command
func (h *UpdateCourseHandler) Handle(ctx context.Context, cmd UpdateCourseCommand) (interface{}, error) {
	var course models.LmsCourse
	q := h.db.WithContext(ctx)
	if cmd.ID != uuid.Nil {
		q = q.First(&course, "id = ?", cmd.ID)
	} else {
		q = q.First(&course, "id = ?", cmd.CourseID)
	}
	if err := q.Error; err != nil {
		return nil, err
	}

	if cmd.Title != nil {
		course.Title = *cmd.Title
	}
	if cmd.Slug != nil && strings.TrimSpace(*cmd.Slug) != "" {
		course.Slug = strings.TrimSpace(*cmd.Slug)
	}
	if cmd.ShortDescription != nil {
		course.ShortDescription = cmd.ShortDescription
	}
	if cmd.LongDescription != nil {
		course.LongDescription = cmd.LongDescription
	}
	if cmd.CoverImageURL != nil {
		course.CoverImageURL = cmd.CoverImageURL
	}
	if cmd.PromoVideoURL != nil {
		course.PromoVideoURL = cmd.PromoVideoURL
	}
	if cmd.Level != nil && *cmd.Level != "" {
		course.Level = models.CourseLevel(strings.ToUpper(*cmd.Level))
	}
	if cmd.Language != nil && *cmd.Language != "" {
		course.Language = *cmd.Language
	}
	if cmd.EstimatedDurationMins != nil {
		course.EstimatedDurationMins = *cmd.EstimatedDurationMins
	}
	if cmd.HasCertificate != nil {
		course.HasCertificate = *cmd.HasCertificate
	}
	if cmd.CertificateTemplate != nil {
		templateID, err := uuid.Parse(*cmd.CertificateTemplate)
		if err != nil {
			return nil, ErrCertificateTemplateNotFound
		}
		var exists int64
		if err := h.db.WithContext(ctx).Model(&models.LmsCertificateTemplate{}).
			Where("id = ?", templateID).Count(&exists).Error; err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, ErrCertificateTemplateNotFound
		}
		course.CertificateTemplate = cmd.CertificateTemplate
	} else if cmd.ClearCertificateTemplate {
		course.CertificateTemplate = nil
	}
	if cmd.MaxStudents != nil {
		course.MaxStudents = cmd.MaxStudents
	}
	if cmd.IsFeatured != nil {
		course.IsFeatured = *cmd.IsFeatured
	}
	if cmd.IsTrending != nil {
		course.IsTrending = *cmd.IsTrending
	}
	if cmd.IsNew != nil {
		course.IsNew = *cmd.IsNew
	}
	if cmd.SEOTitle != nil {
		course.SEOTitle = cmd.SEOTitle
	}
	if cmd.SEODescription != nil {
		course.SEODescription = cmd.SEODescription
	}
	if cmd.SEOKeywords != nil {
		course.SEOKeywords = models.PGStringArray(cmd.SEOKeywords)
	}
	if cmd.PrerequisitesText != nil {
		course.PrerequisitesText = cmd.PrerequisitesText
	}
	if cmd.TargetAudience != nil {
		course.TargetAudience = cmd.TargetAudience
	}
	if cmd.LearningOutcomes != nil {
		course.LearningOutcomes = models.PGStringArray(cmd.LearningOutcomes)
	}
	if cmd.PrimaryInstructorID != nil && strings.TrimSpace(*cmd.PrimaryInstructorID) != "" {
		if parsed, err := uuid.Parse(strings.TrimSpace(*cmd.PrimaryInstructorID)); err == nil {
			course.PrimaryInstructorID = parsed
		}
	}

	course.UpdatedAt = time.Now()
	if err := h.db.WithContext(ctx).Save(&course).Error; err != nil {
		return nil, err
	}
	return &course, nil
}
