package models

import (
	"time"

	"github.com/google/uuid"
)

// Course represents the core course entity in the domain layer
type Course struct {
	ID                    uuid.UUID
	Title                 string
	Slug                  string
	ShortDescription      *string
	LongDescription       *string
	CoverImageURL         *string
	PromoVideoURL         *string
	Status                CourseStatus
	Level                 CourseLevel
	Language              string
	EstimatedDurationMins int
	HasCertificate        bool
	CertificateTemplate   *string
	MaxStudents           *int
	Version               int
	IsFeatured            bool
	IsTrending            bool
	IsNew                 bool
	NewFrom               *time.Time
	NewUntil              *time.Time
	SEOTitle              *string
	SEODescription        *string
	SEOKeywords           []string
	PrerequisitesText     *string
	TargetAudience        *string
	LearningOutcomes      []string
	PrimaryInstructorID   uuid.UUID
	AvailableFrom         *time.Time
	AvailableUntil        *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time

	// Domain associations
	Sections    []Section
	Pricings    []Pricing
	Instructors []Instructor
	Categories  []DomainCategory
	Tags        []Tag
}

// CourseStatus represents the lifecycle state of a course
type CourseStatus string

const (
	CourseStatusDraft       CourseStatus = "DRAFT"
	CourseStatusUnderReview CourseStatus = "UNDER_REVIEW"
	CourseStatusPublished   CourseStatus = "PUBLISHED"
	CourseStatusArchived    CourseStatus = "ARCHIVED"
	CourseStatusRejected    CourseStatus = "REJECTED"
)

// CourseLevel represents the difficulty level of a course
type CourseLevel string

const (
	CourseLevelBeginner     CourseLevel = "BEGINNER"
	CourseLevelIntermediate CourseLevel = "INTERMEDIATE"
	CourseLevelAdvanced     CourseLevel = "ADVANCED"
)

// Section represents a module/chapter in a course
type Section struct {
	ID            uuid.UUID
	CourseID      uuid.UUID
	Title         string
	OrderIndex    int
	AvailableFrom *time.Time
	DripDelayDays *int
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Lessons []Lesson
}

// Lesson represents an individual lesson
type Lesson struct {
	ID               uuid.UUID
	SectionID        uuid.UUID
	Title            string
	Type             LessonType
	Content          *string
	MediaURL         *string
	DurationSeconds  int
	IsFreePreview    bool
	OrderIndex       int
	AvailabilityType AvailabilityType
	AvailableFrom    *time.Time
	DripDelayDays    *int
	CreatedAt        time.Time
	UpdatedAt        time.Time

	Attachments []Attachment
	Subtitles   []Subtitle
	Quizzes     []Quiz
}

// LessonType represents the type of lesson content
type LessonType string

const (
	LessonTypeVideo           LessonType = "VIDEO"
	LessonTypeText            LessonType = "TEXT"
	LessonTypeAudio           LessonType = "AUDIO"
	LessonTypeFile            LessonType = "FILE"
	LessonTypeExternalLink    LessonType = "EXTERNAL_LINK"
	LessonTypeInteractiveQuiz LessonType = "INTERACTIVE_QUIZ"
)

// AvailabilityType represents how lesson availability is determined
type AvailabilityType string

const (
	AvailabilityTypeCalendarDate       AvailabilityType = "CALENDAR_DATE"
	AvailabilityTypeEnrollmentRelative AvailabilityType = "ENROLLMENT_RELATIVE"
)

// Attachment represents downloadable files for lessons
type Attachment struct {
	ID        uuid.UUID
	LessonID  uuid.UUID
	Title     string
	FileURL   string
	FileType  string
	FileSize  *int64
	CreatedAt time.Time
}

// Subtitle represents subtitle tracks for video lessons
type Subtitle struct {
	ID        uuid.UUID
	LessonID  uuid.UUID
	Language  string
	VTTURL    string
	CreatedAt time.Time
}

// Quiz represents interactive quizzes within lessons
type Quiz struct {
	ID           uuid.UUID
	LessonID     uuid.UUID
	TimestampSec int
	Question     string
	Options      []byte
	CorrectIndex int
	CreatedAt    time.Time
}

// Pricing represents course pricing configuration
type Pricing struct {
	ID                       uuid.UUID
	CourseID                 uuid.UUID
	Type                     DomainPricingType
	Amount                   float64
	CurrencyCode             string
	SubscriptionDurationDays *int
	DiscountPrice            *float64
	DiscountStartAt          *time.Time
	DiscountEndAt            *time.Time
	SubscriptionPlanID       *string
	IsActive                 bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// DomainPricingType represents the pricing model
type DomainPricingType string

const (
	PricingTypeFree         DomainPricingType = "FREE"
	PricingTypeOneTime      DomainPricingType = "ONE_TIME"
	PricingTypeSubscription DomainPricingType = "SUBSCRIPTION"
	PricingTypeBundle       DomainPricingType = "BUNDLE"
)

// Instructor represents course instructors
type Instructor struct {
	CourseID     uuid.UUID
	InstructorID uuid.UUID
	Role         string
	Permissions  []byte
	CreatedAt    time.Time
}

// DomainCategory represents course categories
type DomainCategory struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	ParentID  *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Tag represents course tags
type Tag struct {
	ID   uuid.UUID
	Name string
}

// DomainEnrollment represents student enrollment in a course
type DomainEnrollment struct {
	ID          uuid.UUID
	CourseID    uuid.UUID
	UserID      uuid.UUID
	Progress    float64
	EnrolledAt  time.Time
	CompletedAt *time.Time
	BundleID    *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Review represents course reviews
type Review struct {
	ID        uuid.UUID
	CourseID  uuid.UUID
	UserID    uuid.UUID
	Rating    int
	Comment   string
	Status    string
	Reply     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Certificate represents completion certificates
type Certificate struct {
	ID            uuid.UUID
	CourseID      uuid.UUID
	UserID        uuid.UUID
	CertificateNo string
	QRCodeURL     *string
	PDFURL        string
	IssuedAt      time.Time
	CreatedAt     time.Time
}

// Bundle represents course bundles
type Bundle struct {
	ID            uuid.UUID
	Title         string
	Slug          string
	Description   *string
	CoverURL      *string
	Price         float64
	CurrencyCode  string
	IsActive      bool
	IsFeatured    bool
	FeaturedUntil *time.Time
	TotalCourses  int
	TotalStudents int
	TotalRevenue  float64
	CourseIDs     []uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DomainCourseVersion represents a version snapshot of a course
type DomainCourseVersion struct {
	ID            uuid.UUID
	CourseID      uuid.UUID
	VersionNumber int
	Snapshot      []byte
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
}

// CourseChangelog represents a change log entry for a course
type CourseChangelog struct {
	ID        uuid.UUID
	CourseID  uuid.UUID
	UserID    uuid.UUID
	Field     string
	OldValue  *string
	NewValue  *string
	CreatedAt time.Time
}
