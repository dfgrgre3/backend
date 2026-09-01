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

// CreateCourseHandler handles course creation commands
type CreateCourseHandler struct {
	db *gorm.DB
}

// NewCreateCourseHandler creates a new CreateCourseHandler
func NewCreateCourseHandler(db *gorm.DB) *CreateCourseHandler {
	return &CreateCourseHandler{db: db}
}

// slugify converts a title into a URL-safe slug (ASCII only).
func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// ensureUniqueSlug appends a short random suffix if the slug already exists.
func (h *CreateCourseHandler) ensureUniqueSlug(ctx context.Context, slug string) string {
	base := slug
	for i := 0; i < 10; i++ {
		var count int64
		h.db.WithContext(ctx).Model(&models.LmsCourse{}).Where("slug = ?", slug).Count(&count)
		if count == 0 {
			return slug
		}
		slug = base + "-" + uuid.New().String()[:6]
	}
	return slug
}

// Handle handles the create course command
func (h *CreateCourseHandler) Handle(ctx context.Context, cmd CreateCourseCommand) (interface{}, error) {
	slug := strings.TrimSpace(cmd.Slug)
	if slug == "" {
		slug = slugify(cmd.Title)
		if slug == "" {
			slug = "course-" + uuid.New().String()[:8]
		}
	}
	slug = h.ensureUniqueSlug(ctx, slug)

	instructorID := uuid.Nil
	if strings.TrimSpace(cmd.PrimaryInstructorID) != "" {
		if parsed, err := uuid.Parse(strings.TrimSpace(cmd.PrimaryInstructorID)); err == nil {
			var count int64
			if err := h.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", parsed).Count(&count).Error; err != nil {
				return nil, err
			}
			if count == 0 {
				return nil, errors.New("instructor not found")
			}
			instructorID = parsed
		} else {
			return nil, errors.New("invalid instructor ID format")
		}
	}

	level := models.CourseLevelBeginner
	switch strings.ToUpper(cmd.Level) {
	case "INTERMEDIATE":
		level = models.CourseLevelIntermediate
	case "ADVANCED":
		level = models.CourseLevelAdvanced
	}

	language := cmd.Language
	if language == "" {
		language = "ar"
	}

	course := &models.LmsCourse{
		ID:                    uuid.New(),
		Title:                 cmd.Title,
		Slug:                  slug,
		ShortDescription:      cmd.ShortDescription,
		LongDescription:       cmd.LongDescription,
		CoverImageURL:         cmd.CoverImageURL,
		PromoVideoURL:         cmd.PromoVideoURL,
		Status:                models.CourseStatusDraft,
		Level:                 level,
		Language:              language,
		EstimatedDurationMins: cmd.EstimatedDurationMins,
		HasCertificate:        cmd.HasCertificate,
		CertificateTemplate:   cmd.CertificateTemplate,
		MaxStudents:           cmd.MaxStudents,
		Version:               1,
		SEOTitle:              cmd.SEOTitle,
		SEODescription:        cmd.SEODescription,
		SEOKeywords:           models.PGStringArray(cmd.SEOKeywords),
		PrerequisitesText:     cmd.PrerequisitesText,
		TargetAudience:        cmd.TargetAudience,
		LearningOutcomes:      models.PGStringArray(cmd.LearningOutcomes),
		PrimaryInstructorID:   instructorID,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if err := h.db.WithContext(ctx).Create(course).Error; err != nil {
		return nil, err
	}
	return course, nil
}
