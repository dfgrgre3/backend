package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LMS (course) domain models are split across several files in this package
// (all sharing package models), one model group per file, following the
// existing lms_course_availability.go convention: this file (PriceType +
// the primary LmsCourse model), lms_section.go, lms_lesson.go,
// lms_attachment.go, lms_subtitle.go, lms_assignment.go,
// lms_certificate_template.go, lms_interactive_quiz.go, lms_video_note.go,
// lms_lesson_interaction.go, lms_taxonomy.go, lms_course_relations.go,
// lms_instructor.go, lms_pricing.go, lms_bundle.go,
// lms_changelog_version.go, lms_certificate.go, lms_review.go and
// lms_enrollment.go.

type PriceType string

const (
	PriceTypeFree         PriceType = "FREE"
	PriceTypePaid         PriceType = "PAID"
	PriceTypeSubscription PriceType = "SUBSCRIPTION"
	PriceTypeBundle       PriceType = "BUNDLE"
	PriceTypeOneTime      PriceType = "ONE_TIME"
)

// LmsCourse represents the primary course model
type LmsCourse struct {
	ID                    uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Title                 string         `gorm:"not null;index;column:title" json:"title"`
	Slug                  string         `gorm:"uniqueIndex;not null;column:slug" json:"slug"`
	ShortDescription      *string        `gorm:"column:short_description" json:"shortDescription,omitempty"`
	LongDescription       *string        `gorm:"type:text;column:long_description" json:"longDescription,omitempty"`
	CoverImageURL         *string        `gorm:"column:cover_image_url" json:"coverImageUrl,omitempty"`
	PromoVideoURL         *string        `gorm:"column:promo_video_url" json:"promoVideoUrl,omitempty"`
	Status                CourseStatus   `gorm:"default:'DRAFT';index;column:status" json:"status"`
	Level                 CourseLevel    `gorm:"default:'BEGINNER';index;column:level" json:"level"`
	Language              string         `gorm:"default:'ar';index;column:language" json:"language"`
	EstimatedDurationMins int            `gorm:"default:0;column:estimated_duration_mins" json:"estimatedDurationMins"`
	HasCertificate        bool           `gorm:"default:false;column:has_certificate" json:"hasCertificate"`
	CertificateTemplate   *string        `gorm:"type:text;column:certificate_template" json:"certificateTemplate,omitempty"`
	MaxStudents           *int           `gorm:"column:max_students" json:"maxStudents,omitempty"`
	Version               int            `gorm:"default:1;column:version" json:"version"`
	IsFeatured            bool           `gorm:"default:false;index;column:is_featured" json:"isFeatured"`
	IsTrending            bool           `gorm:"default:false;index;column:is_trending" json:"isTrending"`
	IsNew                 bool           `gorm:"default:false;index;column:is_new" json:"isNew"`
	NewFrom               *time.Time     `gorm:"column:new_from" json:"newFrom,omitempty"`
	NewUntil              *time.Time     `gorm:"column:new_until" json:"newUntil,omitempty"`
	SEOTitle              *string        `gorm:"column:seo_title" json:"seoTitle,omitempty"`
	SEODescription        *string        `gorm:"type:text;column:seo_description" json:"seoDescription,omitempty"`
	SEOKeywords           PGStringArray  `gorm:"type:text[];column:seo_keywords" json:"seoKeywords"`
	PrerequisitesText     *string        `gorm:"type:text;column:prerequisites_text" json:"prerequisitesText,omitempty"`
	TargetAudience        *string        `gorm:"type:text;column:target_audience" json:"targetAudience,omitempty"`
	LearningOutcomes      PGStringArray  `gorm:"type:text[];column:learning_outcomes" json:"learningOutcomes"`
	PrimaryInstructorID   uuid.UUID      `gorm:"not null;type:uuid;index;column:primary_instructor_id" json:"primaryInstructorId"`
	AvailableFrom         *time.Time     `gorm:"column:available_from" json:"availableFrom,omitempty"`
	AvailableUntil        *time.Time     `gorm:"column:available_until" json:"availableUntil,omitempty"`
	CreatedAt             time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt             time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt             gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Associations
	Sections            []LmsSection                  `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"sections,omitempty"`
	Pricings            []LmsPricing                  `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"pricings,omitempty"`
	Instructors         []LmsInstructor               `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"instructors,omitempty"`
	AvailabilityWindows []LmsCourseAvailabilityWindow `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"availabilityWindows,omitempty"`
}

func (LmsCourse) TableName() string {
	return "LmsCourse"
}

func (c *LmsCourse) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}
