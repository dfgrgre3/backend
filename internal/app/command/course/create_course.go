package course

import (
	"context"
	"time"

	"github.com/google/uuid"
	"thanawy-backend/internal/domain/course"
)

// CreateCourseCommand represents the command to create a course
type CreateCourseCommand struct {
	Title                 string
	Slug                  string
	ShortDescription      *string
	LongDescription       *string
	CoverImageURL         *string
	PromoVideoURL         *string
	Level                 course.CourseLevel
	Language              string
	EstimatedDurationMins int
	HasCertificate        bool
	CertificateTemplate   *string
	MaxStudents           *int
	SEOTitle              *string
	SEODescription        *string
	SEOKeywords           []string
	PrerequisitesText     *string
	TargetAudience        *string
	LearningOutcomes      []string
	PrimaryInstructorID   string
	CategoryIDs           []string
}

// CreateCourseHandler handles the create course command
type CreateCourseHandler struct {
	service *course.CourseService
}

// NewCreateCourseHandler creates a new create course handler
func NewCreateCourseHandler(service *course.CourseService) *CreateCourseHandler {
	return &CreateCourseHandler{service: service}
}

// Handle executes the create course command
func (h *CreateCourseHandler) Handle(ctx context.Context, cmd CreateCourseCommand) (*course.Course, error) {
	courseID := uuid.New()
	now := time.Now()
	
	courseEntity := &course.Course{
		ID:                    courseID,
		Title:                 cmd.Title,
		Slug:                  cmd.Slug,
		ShortDescription:      cmd.ShortDescription,
		LongDescription:       cmd.LongDescription,
		CoverImageURL:         cmd.CoverImageURL,
		PromoVideoURL:         cmd.PromoVideoURL,
		Status:                course.CourseStatusDraft,
		Level:                 cmd.Level,
		Language:              cmd.Language,
		EstimatedDurationMins: cmd.EstimatedDurationMins,
		HasCertificate:        cmd.HasCertificate,
		CertificateTemplate:   cmd.CertificateTemplate,
		MaxStudents:           cmd.MaxStudents,
		Version:               1,
		IsFeatured:            false,
		IsTrending:            false,
		IsNew:                 true,
		NewFrom:               &now,
		SEOTitle:              cmd.SEOTitle,
		SEODescription:        cmd.SEODescription,
		SEOKeywords:           cmd.SEOKeywords,
		PrerequisitesText:     cmd.PrerequisitesText,
		TargetAudience:        cmd.TargetAudience,
		LearningOutcomes:      cmd.LearningOutcomes,
		PrimaryInstructorID:   uuid.MustParse(cmd.PrimaryInstructorID),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	
	createdCourse, err := h.service.CreateCourse(ctx, courseEntity)
	if err != nil {
		return nil, err
	}
	
	// Add categories
	for _, categoryID := range cmd.CategoryIDs {
		if err := h.service.AddCategory(ctx, courseID.String(), categoryID); err != nil {
			// Log error but don't fail the whole operation
			continue
		}
	}
	
	return createdCourse, nil
}
