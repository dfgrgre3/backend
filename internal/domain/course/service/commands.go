package courseservice

import "github.com/google/uuid"

// CreateCourseCommand represents a command to create a course
type CreateCourseCommand struct {
	Title                 string
	Slug                  string
	ShortDescription      *string
	LongDescription       *string
	CoverImageURL         *string
	PromoVideoURL         *string
	Level                 string
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

// UpdateCourseCommand represents a command to update a course
type UpdateCourseCommand struct {
	ID                    uuid.UUID
	CourseID              string
	Title                 *string
	Slug                  *string
	ShortDescription      *string
	LongDescription       *string
	CoverImageURL         *string
	PromoVideoURL         *string
	Level                 *string
	Language              *string
	EstimatedDurationMins *int
	HasCertificate        *bool
	CertificateTemplate   *string
	// ClearCertificateTemplate is set when the caller explicitly sent
	// certificateTemplate: null (as opposed to omitting the field), since a
	// nil CertificateTemplate can't otherwise distinguish "leave it" from
	// "clear it".
	ClearCertificateTemplate bool
	MaxStudents              *int
	IsFeatured               *bool
	IsTrending               *bool
	IsNew                    *bool
	SEOTitle                 *string
	SEODescription           *string
	SEOKeywords              []string
	PrerequisitesText        *string
	TargetAudience           *string
	LearningOutcomes         []string
	PrimaryInstructorID      *string
	CategoryIDs              []string
}

// EnrollUserCommand represents a command to enroll a user in a course
type EnrollUserCommand struct {
	UserID   string
	CourseID string
}

// UpdateProgressCommand represents a command to update user progress
type UpdateProgressCommand struct {
	UserID        string
	CourseID      string
	LessonID      string
	Progress      float64
	Completed     bool
	TimeSpentMins int
}
