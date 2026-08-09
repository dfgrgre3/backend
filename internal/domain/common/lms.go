package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

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

// LmsSection represents a section/module in a course
type LmsSection struct {
	ID            uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID      uuid.UUID      `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	Title         string         `gorm:"not null;column:title" json:"title"`
	OrderIndex    int            `gorm:"default:0;index;column:order_index" json:"orderIndex"`
	AvailableFrom *time.Time     `gorm:"column:available_from" json:"availableFrom,omitempty"`
	DripDelayDays *int           `gorm:"column:drip_delay_days" json:"dripDelayDays,omitempty"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Associations
	Lessons []LmsLesson `gorm:"foreignKey:SectionID;constraint:OnDelete:CASCADE" json:"lessons,omitempty"`
}

func (LmsSection) TableName() string {
	return "LmsSection"
}

func (s *LmsSection) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}

// LmsLesson represents individual course lessons with drip content rules
type LmsLesson struct {
	ID               uuid.UUID        `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SectionID        uuid.UUID        `gorm:"not null;type:uuid;index;column:section_id;constraint:OnDelete:CASCADE" json:"sectionId"`
	Title            string           `gorm:"not null;column:title" json:"title"`
	Type             LessonType       `gorm:"default:'VIDEO';index;column:type" json:"type"`
	Content          *string          `gorm:"type:text;column:content" json:"content,omitempty"`
	MediaURL         *string          `gorm:"column:media_url" json:"mediaUrl,omitempty"`
	DurationSeconds  int              `gorm:"default:0;column:duration_seconds" json:"durationSeconds"`
	IsFreePreview    bool             `gorm:"default:false;index;column:is_free_preview" json:"isFreePreview"`
	OrderIndex       int              `gorm:"default:0;index;column:order_index" json:"orderIndex"`
	AvailabilityType AvailabilityType `gorm:"default:'CALENDAR_DATE';column:availability_type" json:"availabilityType"`
	AvailableFrom    *time.Time       `gorm:"column:available_from" json:"availableFrom,omitempty"`
	DripDelayDays    *int             `gorm:"column:drip_delay_days" json:"dripDelayDays,omitempty"`
	CreatedAt        time.Time        `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        time.Time        `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt        gorm.DeletedAt   `gorm:"index;column:deleted_at" json:"-"`

	// Associations
	Attachments []LmsAttachment      `gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE" json:"attachments,omitempty"`
	Subtitles   []LmsSubtitle        `gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE" json:"subtitles,omitempty"`
	Quizzes     []LmsInteractiveQuiz `gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE" json:"quizzes,omitempty"`
}

func (LmsLesson) TableName() string {
	return "LmsLesson"
}

func (l *LmsLesson) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return
}

// LmsAttachment represents downloadable files
type LmsAttachment struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID  uuid.UUID      `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	Title     string         `gorm:"not null;column:title" json:"title"`
	FileURL   string         `gorm:"not null;column:file_url" json:"fileUrl"`
	FileType  string         `gorm:"column:file_type" json:"fileType"`
	FileSize  *int64         `gorm:"column:file_size" json:"fileSize,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsAttachment) TableName() string {
	return "LmsAttachment"
}

func (a *LmsAttachment) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}

// LmsSubtitle represents VTT/SRT subtitles
type LmsSubtitle struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID  uuid.UUID      `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	Language  string         `gorm:"not null;index;column:language" json:"language"`
	VTTURL    string         `gorm:"not null;column:vtt_url" json:"vttUrl"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsSubtitle) TableName() string {
	return "LmsSubtitle"
}

func (s *LmsSubtitle) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}

// LmsInteractiveQuiz represents inline interactive video questions
type LmsInteractiveQuiz struct {
	ID           uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID     uuid.UUID       `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	TimestampSec int             `gorm:"not null;column:timestamp_sec" json:"timestampSec"`
	Question     string          `gorm:"not null;type:text;column:question" json:"question"`
	Options      json.RawMessage `gorm:"type:jsonb;column:options" json:"options"`
	CorrectIndex int             `gorm:"not null;column:correct_index" json:"correctIndex"`
	CreatedAt    time.Time       `gorm:"column:created_at" json:"createdAt"`
	DeletedAt    gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsInteractiveQuiz) TableName() string {
	return "LmsInteractiveQuiz"
}

func (q *LmsInteractiveQuiz) BeforeCreate(tx *gorm.DB) (err error) {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return
}

// LmsVideoNote represents user timestamped bookmarks and notes
type LmsVideoNote struct {
	ID           uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID     uuid.UUID `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	UserID       uuid.UUID `gorm:"not null;type:uuid;index;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	TimestampSec int       `gorm:"not null;column:timestamp_sec" json:"timestampSec"`
	Note         string    `gorm:"not null;type:text;column:note" json:"note"`
	CreatedAt    time.Time `gorm:"index;column:created_at" json:"createdAt"`
}

func (LmsVideoNote) TableName() string {
	return "LmsVideoNote"
}

func (n *LmsVideoNote) BeforeCreate(tx *gorm.DB) (err error) {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return
}

// LmsLessonInteraction tracks video watch progress & quiz answers
type LmsLessonInteraction struct {
	ID                 uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID           uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_interaction_user_lesson,unique;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	UserID             uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_interaction_user_lesson,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	WatchedDurationSec int             `gorm:"default:0;column:watched_duration_sec" json:"watchedDurationSec"`
	LastPositionSec    int             `gorm:"default:0;column:last_position_sec" json:"lastPositionSec"`
	PlayCount          int             `gorm:"default:0;column:play_count" json:"playCount"`
	IsCompleted        bool            `gorm:"default:false;index;column:is_completed" json:"isCompleted"`
	QuizAnswers        json.RawMessage `gorm:"type:jsonb;column:quiz_answers" json:"quizAnswers,omitempty"`
	CreatedAt          time.Time       `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (LmsLessonInteraction) TableName() string {
	return "LmsLessonInteraction"
}

func (i *LmsLessonInteraction) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return
}

// LmsCategory represents category tree hierarchy
type LmsCategory struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name      string         `gorm:"not null;column:name" json:"name"`
	Slug      string         `gorm:"uniqueIndex;not null;column:slug" json:"slug"`
	ParentID  *uuid.UUID     `gorm:"index;type:uuid;column:parent_id" json:"parentId,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsCategory) TableName() string {
	return "LmsCategory"
}

func (c *LmsCategory) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}

// LmsTag represents course tags
type LmsTag struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name      string         `gorm:"uniqueIndex;not null;column:name" json:"name"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsTag) TableName() string {
	return "LmsTag"
}

func (t *LmsTag) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return
}

// LmsCourseCategory is a join table for course-category many-to-many
type LmsCourseCategory struct {
	CourseID   uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	CategoryID uuid.UUID `gorm:"primaryKey;type:uuid;column:category_id;constraint:OnDelete:CASCADE" json:"categoryId"`
}

func (LmsCourseCategory) TableName() string {
	return "LmsCourseCategory"
}

// LmsCourseTag is a join table for course-tag many-to-many
type LmsCourseTag struct {
	CourseID uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	TagID    uuid.UUID `gorm:"primaryKey;type:uuid;column:tag_id;constraint:OnDelete:CASCADE" json:"tagId"`
}

func (LmsCourseTag) TableName() string {
	return "LmsCourseTag"
}

// LmsCoursePrerequisite is a join table for course prerequisites
type LmsCoursePrerequisite struct {
	CourseID             uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	PrerequisiteCourseID uuid.UUID `gorm:"primaryKey;type:uuid;column:prerequisite_course_id;constraint:OnDelete:CASCADE" json:"prerequisiteCourseId"`
}

func (LmsCoursePrerequisite) TableName() string {
	return "LmsCoursePrerequisite"
}

// LmsRelatedCourse is a join table for related courses
type LmsRelatedCourse struct {
	CourseID        uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	RelatedCourseID uuid.UUID `gorm:"primaryKey;type:uuid;column:related_course_id;constraint:OnDelete:CASCADE" json:"relatedCourseId"`
}

func (LmsRelatedCourse) TableName() string {
	return "LmsRelatedCourse"
}

// LmsInstructor mapping with roles
type LmsInstructor struct {
	CourseID     uuid.UUID       `gorm:"primaryKey;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	InstructorID uuid.UUID       `gorm:"primaryKey;type:uuid;index;column:instructor_id;constraint:OnDelete:CASCADE" json:"instructorId"`
	Role         string          `gorm:"default:'INSTRUCTOR';column:role" json:"role"`
	Permissions  json.RawMessage `gorm:"type:jsonb;column:permissions" json:"permissions"`
	CreatedAt    time.Time       `gorm:"column:created_at" json:"createdAt"`
}

func (LmsInstructor) TableName() string {
	return "LmsInstructor"
}

// LmsPricing for multi-currency & price types
type LmsPricing struct {
	ID                       uuid.UUID        `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID                 uuid.UUID        `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	Type                     PriceType        `gorm:"default:'FREE';index;column:type" json:"type"`
	Amount                   decimal.Decimal  `gorm:"default:0;type:numeric(19,4);column:amount" json:"amount"`
	CurrencyCode             string           `gorm:"default:'USD';index;column:currency_code" json:"currencyCode"`
	SubscriptionDurationDays *int             `gorm:"column:subscription_duration_days" json:"subscriptionDurationDays,omitempty"`
	DiscountPrice            *decimal.Decimal `gorm:"type:numeric(19,4);column:discount_price" json:"discountPrice,omitempty"`
	DiscountStartAt          *time.Time       `gorm:"column:discount_start_at" json:"discountStartAt,omitempty"`
	DiscountEndAt            *time.Time       `gorm:"column:discount_end_at" json:"discountEndAt,omitempty"`
	SubscriptionPlanID       *string          `gorm:"column:subscription_plan_id" json:"subscriptionPlanId,omitempty"`
	IsActive                 bool             `gorm:"default:true;column:is_active" json:"isActive"`
	CreatedAt                time.Time        `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                time.Time        `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt                gorm.DeletedAt   `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsPricing) TableName() string {
	return "LmsPricing"
}

func (p *LmsPricing) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return
}

// LmsBundle represents course bundles
type LmsBundle struct {
	ID           uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Title        string          `gorm:"not null;index;column:title" json:"title"`
	Slug         string          `gorm:"uniqueIndex;not null;column:slug" json:"slug"`
	Description  *string         `gorm:"type:text;column:description" json:"description,omitempty"`
	CoverURL     *string         `gorm:"column:cover_url" json:"coverUrl,omitempty"`
	Price        decimal.Decimal `gorm:"default:0;type:numeric(19,4);column:price" json:"price"`
	CurrencyCode string          `gorm:"default:'USD';column:currency_code" json:"currencyCode"`
	IsActive     bool            `gorm:"default:true;index;column:is_active" json:"isActive"`
	CreatedAt    time.Time       `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt    time.Time       `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`

	// Associations
	Courses []LmsCourse `gorm:"many2many:LmsBundleCourse;" json:"courses,omitempty"`
}

func (LmsBundle) TableName() string {
	return "LmsBundle"
}

func (b *LmsBundle) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return
}

// LmsCourseChangelog tracks detailed modifications
type LmsCourseChangelog struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID  uuid.UUID `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID    uuid.UUID `gorm:"not null;type:uuid;index;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Field     string    `gorm:"not null;column:field" json:"field"`
	OldValue  *string   `gorm:"column:old_value" json:"oldValue,omitempty"`
	NewValue  *string   `gorm:"column:new_value" json:"newValue,omitempty"`
	CreatedAt time.Time `gorm:"index;column:created_at" json:"createdAt"`
}

func (LmsCourseChangelog) TableName() string {
	return "LmsCourseChangelog"
}

func (cl *LmsCourseChangelog) BeforeCreate(tx *gorm.DB) (err error) {
	if cl.ID == uuid.Nil {
		cl.ID = uuid.New()
	}
	return
}

// LmsCourseVersion stores version snapshot
type LmsCourseVersion struct {
	ID            uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID      uuid.UUID       `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	VersionNumber int             `gorm:"not null;column:version_number" json:"versionNumber"`
	Snapshot      json.RawMessage `gorm:"type:jsonb;column:snapshot" json:"snapshot"`
	CreatedAt     time.Time       `gorm:"index;column:created_at" json:"createdAt"`
}

func (LmsCourseVersion) TableName() string {
	return "LmsCourseVersion"
}

func (v *LmsCourseVersion) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return
}

// LmsCertificate stores completion certificates
type LmsCertificate struct {
	ID            uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID      uuid.UUID `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID        uuid.UUID `gorm:"not null;type:uuid;index:idx_lms_cert_user_course,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	CertificateNo string    `gorm:"uniqueIndex;not null;column:certificate_no" json:"certificateNo"`
	QRCodeURL     *string   `gorm:"column:qr_code_url" json:"qrCodeUrl,omitempty"`
	PDFURL        string    `gorm:"not null;column:pdf_url" json:"pdfUrl"`
	IssuedAt      time.Time `gorm:"index;column:issued_at" json:"issuedAt"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (LmsCertificate) TableName() string {
	return "LmsCertificate"
}

func (cert *LmsCertificate) BeforeCreate(tx *gorm.DB) (err error) {
	if cert.ID == uuid.Nil {
		cert.ID = uuid.New()
	}
	if cert.IssuedAt.IsZero() {
		cert.IssuedAt = time.Now()
	}
	return
}

// LmsReview represents course reviews
type LmsReview struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID  uuid.UUID      `gorm:"not null;type:uuid;index:idx_lms_review_user_course,unique;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID    uuid.UUID      `gorm:"not null;type:uuid;index:idx_lms_review_user_course,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Rating    int            `gorm:"not null;default:5;column:rating" json:"rating"`
	Comment   string         `gorm:"type:text;column:comment" json:"comment"`
	Status    string         `gorm:"default:'APPROVED';index;column:status" json:"status"` // APPROVED, PENDING, REJECTED
	Reply     *string        `gorm:"type:text;column:reply" json:"reply,omitempty"`
	CreatedAt time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsReview) TableName() string {
	return "LmsReview"
}

func (r *LmsReview) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return
}

// LmsReviewComment holds reviewer comments during the workflow review process.
type LmsReviewComment struct {
	ID         uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID   uuid.UUID `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	ReviewerID uuid.UUID `gorm:"not null;type:uuid;index;column:reviewer_id;constraint:OnDelete:CASCADE" json:"reviewerId"`
	Comment    string    `gorm:"not null;type:text;column:comment" json:"comment"`
	Status     string    `gorm:"default:'pending';column:status" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LmsReviewComment) TableName() string {
	return "LmsReviewComment"
}

func (rc *LmsReviewComment) BeforeCreate(tx *gorm.DB) (err error) {
	if rc.ID == uuid.Nil {
		rc.ID = uuid.New()
	}
	return
}

// LmsEnrollment represents student enrollment in a course
type LmsEnrollment struct {
	ID          uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID    uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_enroll_user_course,unique;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID      uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_enroll_user_course,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Progress    decimal.Decimal `gorm:"default:0;type:numeric(5,2);check:progress >= 0 AND progress <= 100;column:progress" json:"progress"`
	EnrolledAt  time.Time       `gorm:"index;column:enrolled_at" json:"enrolledAt"`
	CompletedAt *time.Time      `gorm:"column:completed_at" json:"completedAt,omitempty"`
	BundleID    *uuid.UUID      `gorm:"type:uuid;index;column:bundle_id" json:"bundleId,omitempty"`
	CreatedAt   time.Time       `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsEnrollment) TableName() string {
	return "LmsEnrollment"
}

func (e *LmsEnrollment) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.EnrolledAt.IsZero() {
		e.EnrolledAt = time.Now()
	}
	return
}
