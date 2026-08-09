package protected

// CreateCourseRequest represents the REST request body for creating a course
type CreateCourseRequest struct {
	Title                 string   `json:"title"`
	Name                  string   `json:"name"`
	NameAr                string   `json:"nameAr"`
	Slug                  string   `json:"slug"`
	ShortDescription      *string  `json:"shortDescription"`
	LongDescription       *string  `json:"longDescription"`
	CoverImageURL         *string  `json:"coverImageUrl"`
	PromoVideoURL         *string  `json:"promoVideoUrl"`
	Level                 string   `json:"level"`
	Language              string   `json:"language"`
	EstimatedDurationMins int      `json:"estimatedDurationMins"`
	HasCertificate        bool     `json:"hasCertificate"`
	CertificateTemplate   *string  `json:"certificateTemplate"`
	MaxStudents           *int     `json:"maxStudents"`
	SEOTitle              *string  `json:"seoTitle"`
	SEODescription        *string  `json:"seoDescription"`
	SEOKeywords           []string `json:"seoKeywords"`
	PrerequisitesText     *string  `json:"prerequisitesText"`
	TargetAudience        *string  `json:"targetAudience"`
	LearningOutcomes      []string `json:"learningOutcomes"`
	PrimaryInstructorID   string   `json:"primaryInstructorId"`
	InstructorID          string   `json:"instructorId"`
	CategoryIDs           []string `json:"categoryIds"`
}

// UpdateCourseRequest represents the REST request body for updating a course
type UpdateCourseRequest struct {
	Title                 *string  `json:"title"`
	Name                  *string  `json:"name"`
	NameAr                *string  `json:"nameAr"`
	Slug                  *string  `json:"slug"`
	ShortDescription      *string  `json:"shortDescription"`
	LongDescription       *string  `json:"longDescription"`
	CoverImageURL         *string  `json:"coverImageUrl"`
	PromoVideoURL         *string  `json:"promoVideoUrl"`
	Level                 *string  `json:"level"`
	Language              *string  `json:"language"`
	EstimatedDurationMins *int     `json:"estimatedDurationMins"`
	HasCertificate        *bool    `json:"hasCertificate"`
	CertificateTemplate   *string  `json:"certificateTemplate"`
	MaxStudents           *int     `json:"maxStudents"`
	IsFeatured            *bool    `json:"isFeatured"`
	IsTrending            *bool    `json:"isTrending"`
	IsNew                 *bool    `json:"isNew"`
	SEOTitle              *string  `json:"seoTitle"`
	SEODescription        *string  `json:"seoDescription"`
	SEOKeywords           []string `json:"seoKeywords"`
	PrerequisitesText     *string  `json:"prerequisitesText"`
	TargetAudience        *string  `json:"targetAudience"`
	LearningOutcomes      []string `json:"learningOutcomes"`
	PrimaryInstructorID   *string  `json:"primaryInstructorId"`
	InstructorID          *string  `json:"instructorId"`
	CategoryIDs           []string `json:"categoryIds"`
}
