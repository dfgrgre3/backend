package course

import (
	"context"

	"github.com/google/uuid"
	"thanawy-backend/internal/domain/course"
)

// UpdateCourseCommand represents the command to update a course
type UpdateCourseCommand struct {
	CourseID              string
	Title                 *string
	Slug                  *string
	ShortDescription      *string
	LongDescription       *string
	CoverImageURL         *string
	PromoVideoURL         *string
	Level                 *course.CourseLevel
	Language              *string
	EstimatedDurationMins *int
	HasCertificate        *bool
	CertificateTemplate   *string
	MaxStudents           *int
	IsFeatured            *bool
	IsTrending            *bool
	IsNew                 *bool
	SEOTitle              *string
	SEODescription        *string
	SEOKeywords           []string
	PrerequisitesText     *string
	TargetAudience        *string
	LearningOutcomes      []string
	PrimaryInstructorID   *string
	CategoryIDs           []string
}

// UpdateCourseHandler handles the update course command
type UpdateCourseHandler struct {
	service *course.CourseService
}

// NewUpdateCourseHandler creates a new update course handler
func NewUpdateCourseHandler(service *course.CourseService) *UpdateCourseHandler {
	return &UpdateCourseHandler{service: service}
}

// Handle executes the update course command
func (h *UpdateCourseHandler) Handle(ctx context.Context, cmd UpdateCourseCommand) (*course.Course, error) {
	courseEntity, err := h.service.GetCourse(ctx, cmd.CourseID)
	if err != nil {
		return nil, err
	}
	
	// Update fields if provided
	if cmd.Title != nil {
		courseEntity.Title = *cmd.Title
	}
	if cmd.Slug != nil {
		courseEntity.Slug = *cmd.Slug
	}
	if cmd.ShortDescription != nil {
		courseEntity.ShortDescription = cmd.ShortDescription
	}
	if cmd.LongDescription != nil {
		courseEntity.LongDescription = cmd.LongDescription
	}
	if cmd.CoverImageURL != nil {
		courseEntity.CoverImageURL = cmd.CoverImageURL
	}
	if cmd.PromoVideoURL != nil {
		courseEntity.PromoVideoURL = cmd.PromoVideoURL
	}
	if cmd.Level != nil {
		courseEntity.Level = *cmd.Level
	}
	if cmd.Language != nil {
		courseEntity.Language = *cmd.Language
	}
	if cmd.EstimatedDurationMins != nil {
		courseEntity.EstimatedDurationMins = *cmd.EstimatedDurationMins
	}
	if cmd.HasCertificate != nil {
		courseEntity.HasCertificate = *cmd.HasCertificate
	}
	if cmd.CertificateTemplate != nil {
		courseEntity.CertificateTemplate = cmd.CertificateTemplate
	}
	if cmd.MaxStudents != nil {
		courseEntity.MaxStudents = cmd.MaxStudents
	}
	if cmd.IsFeatured != nil {
		courseEntity.IsFeatured = *cmd.IsFeatured
	}
	if cmd.IsTrending != nil {
		courseEntity.IsTrending = *cmd.IsTrending
	}
	if cmd.IsNew != nil {
		courseEntity.IsNew = *cmd.IsNew
	}
	if cmd.SEOTitle != nil {
		courseEntity.SEOTitle = cmd.SEOTitle
	}
	if cmd.SEODescription != nil {
		courseEntity.SEODescription = cmd.SEODescription
	}
	if len(cmd.SEOKeywords) > 0 {
		courseEntity.SEOKeywords = cmd.SEOKeywords
	}
	if cmd.PrerequisitesText != nil {
		courseEntity.PrerequisitesText = cmd.PrerequisitesText
	}
	if cmd.TargetAudience != nil {
		courseEntity.TargetAudience = cmd.TargetAudience
	}
	if len(cmd.LearningOutcomes) > 0 {
		courseEntity.LearningOutcomes = cmd.LearningOutcomes
	}
	if cmd.PrimaryInstructorID != nil {
		courseEntity.PrimaryInstructorID = uuid.MustParse(*cmd.PrimaryInstructorID)
	}
	
	updatedCourse, err := h.service.UpdateCourse(ctx, courseEntity)
	if err != nil {
		return nil, err
	}
	
	// Update categories if provided
	if cmd.CategoryIDs != nil {
		// Remove all existing categories
		// Note: This would require a bulk remove method in the service
		// For now, we'll add the new ones
		for _, categoryID := range cmd.CategoryIDs {
			if err := h.service.AddCategory(ctx, cmd.CourseID, categoryID); err != nil {
				continue
			}
		}
	}
	
	return updatedCourse, nil
}
