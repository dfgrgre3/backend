package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Level string

const (
	LevelBeginner     Level = "BEGINNER"
	LevelIntermediate Level = "INTERMEDIATE"
	LevelAdvanced     Level = "ADVANCED"
)

type Subject struct {
	ID                     string  `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name                   string  `gorm:"uniqueIndex;not null;index;column:name" json:"name"`
	NameAr                 *string `gorm:"index;column:name_ar" json:"nameAr"`
	Code                   *string `gorm:"uniqueIndex;index;column:code" json:"code"`
	Description            *string `gorm:"column:description" json:"description"`
	Icon                   *string `gorm:"column:icon" json:"icon"`
	Color                  *string `gorm:"default:'#3b82f6';column:color" json:"color"`
	IsActive               bool    `gorm:"default:true;index;column:is_active" json:"isActive"`
	IsPublished            bool    `gorm:"default:false;index;column:is_published" json:"isPublished"`
	Price                  float64 `gorm:"default:0;index;column:price" json:"price"`
	Rating                 float64 `gorm:"default:0;column:rating" json:"rating"`
	EnrolledCount          int     `gorm:"default:0;column:enrolled_count" json:"enrolledCount"`
	ThumbnailUrl           *string `gorm:"column:thumbnail_url" json:"thumbnailUrl"`
	TrailerUrl             *string `gorm:"column:trailer_url" json:"trailerUrl"`
	TrailerDurationMinutes int     `gorm:"default:0;column:trailer_duration_minutes" json:"trailerDurationMinutes"`
	Slug                   *string `gorm:"uniqueIndex;column:slug" json:"slug"`
	Level                  Level   `gorm:"default:'INTERMEDIATE';index;column:level" json:"level"`
	InstructorName         *string `gorm:"column:instructor_name" json:"instructorName"`
	InstructorId           *string `gorm:"index;type:uuid;column:instructor_id" json:"instructorId"`
	CategoryId             *string `gorm:"index;type:uuid;column:category_id" json:"categoryId"`
	DurationHours          int     `gorm:"default:0;column:duration_hours" json:"durationHours"`
	Requirements           *string `gorm:"column:requirements" json:"requirements"`
	LearningObjectives     *string `gorm:"column:learning_objectives" json:"learningObjectives"`
	SeoTitle               *string `gorm:"column:seo_title" json:"seoTitle"`
	SeoDescription         *string `gorm:"column:seo_description" json:"seoDescription"`
	IsFeatured             bool    `gorm:"default:false;index;column:is_featured" json:"isFeatured"`
	Language               string  `gorm:"default:'ar';index;column:language" json:"language"`

	// New fields to match DB and frontend
	CoursePrerequisites PGStringArray `gorm:"type:text[];column:course_prerequisites" json:"coursePrerequisites"`
	TargetAudience      PGStringArray `gorm:"type:text[];column:target_audience" json:"targetAudience"`
	WhatYouLearn        PGStringArray `gorm:"type:text[];column:what_you_learn" json:"whatYouLearn"`
	CompletionRate      float64       `gorm:"default:0;column:completion_rate" json:"completionRate"`
	VideoCount          int           `gorm:"default:0;column:video_count" json:"videoCount"`
	Type                string        `gorm:"default:'COURSE';column:type" json:"type"`
	LastContentUpdate   *time.Time    `gorm:"column:last_content_update" json:"lastContentUpdate"`

	// Lifecycle and enhanced fields (migration 0064 + 0109)
	Status           CourseStatus `gorm:"default:'DRAFT';index;column:status" json:"status"`
	MaxStudents      *int         `gorm:"column:max_students" json:"maxStudents"`
	Version          string       `gorm:"default:'1.0.0';column:version" json:"version"`
	IsTrending       bool         `gorm:"default:false;column:is_trending" json:"isTrending"`
	IsNew            bool         `gorm:"default:false;column:is_new" json:"isNew"`
	NewUntil         *time.Time   `gorm:"column:new_until" json:"newUntil"`
	ShortDescription *string      `gorm:"column:short_description" json:"shortDescription"`
	LongDescription  *string      `gorm:"type:text;column:long_description" json:"longDescription"`
	HasCertificate   bool         `gorm:"default:false;column:has_certificate" json:"hasCertificate"`
	AvailableFrom    *time.Time   `gorm:"column:available_from" json:"availableFrom"`
	AvailableUntil   *time.Time   `gorm:"column:available_until" json:"availableUntil"`
	Tags             []CourseTag  `gorm:"many2many:SubjectTag;" json:"tags,omitempty"`

	// Workflow metadata (Phase 1: 0109)
	SubmittedForReviewAt *time.Time `gorm:"column:submitted_for_review_at" json:"submittedForReviewAt,omitempty"`
	ReviewedAt           *time.Time `gorm:"column:reviewed_at" json:"reviewedAt,omitempty"`
	ReviewedBy           *string    `gorm:"type:uuid;column:reviewed_by" json:"reviewedBy,omitempty"`
	RejectionReason      *string    `gorm:"column:rejection_reason" json:"rejectionReason,omitempty"`
	PublishedAt          *time.Time `gorm:"column:published_at" json:"publishedAt,omitempty"`
	ArchivedAt           *time.Time `gorm:"column:archived_at" json:"archivedAt,omitempty"`
	ArchivedBy           *string    `gorm:"type:uuid;column:archived_by" json:"archivedBy,omitempty"`

	// Operational & enrollment (Phase 1)
	EnrollmentType     EnrollmentType `gorm:"default:'OPEN';column:enrollment_type" json:"enrollmentType"`
	SecondaryLanguages PGStringArray  `gorm:"type:text[];column:secondary_languages" json:"secondaryLanguages"`

	// Certificate config (Phase 1)
	CertificateTemplate             *string `gorm:"column:certificate_template" json:"certificateTemplate,omitempty"`
	CertificateIssueAfterCompletion bool    `gorm:"default:true;column:certificate_issue_after_completion" json:"certificateIssueAfterCompletion"`
	CertificateMinCompletionPct     int     `gorm:"default:100;column:certificate_min_completion_pct" json:"certificateMinCompletionPct"`

	CreatedAt time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Topics      []Topic      `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"topics,omitempty"`
	Enrollments []Enrollment `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"-"`
}

type Topic struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID   string         `gorm:"not null;index;type:uuid;constraint:OnDelete:CASCADE;column:subject_id" json:"subjectId"`
	Title       string         `gorm:"default:'';index;column:title" json:"title"`
	Description *string        `gorm:"column:description" json:"description"`
	Order       int            `gorm:"default:0;index;column:order" json:"order"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	SubTopics []SubTopic `gorm:"foreignKey:TopicID;constraint:OnDelete:CASCADE" json:"subTopics,omitempty"`
}

type SubTopicType string

const (
	SubTopicVideo      SubTopicType = "VIDEO"
	SubTopicQuiz       SubTopicType = "QUIZ"
	SubTopicArticle    SubTopicType = "ARTICLE"
	SubTopicAssignment SubTopicType = "ASSIGNMENT"
	SubTopicAudio      SubTopicType = "AUDIO"
	SubTopicLink       SubTopicType = "LINK"
	SubTopicLive       SubTopicType = "LIVE"
	SubTopicDocument   SubTopicType = "DOCUMENT"
)

type SubTopic struct {
	ID              string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	TopicID         string         `gorm:"not null;index;type:uuid;constraint:OnDelete:CASCADE;column:topic_id" json:"topicId"`
	Title           string         `gorm:"default:'';index;column:title" json:"title"`
	Description     *string        `gorm:"column:description" json:"description"`
	Content         *string        `gorm:"column:content" json:"content"`
	VideoUrl        *string        `gorm:"column:video_url" json:"videoUrl"`
	Type            SubTopicType   `gorm:"default:'VIDEO';index;column:type" json:"type"`
	ExamID          *string        `gorm:"index;type:uuid;column:exam_id" json:"examId"`
	Order           int            `gorm:"default:0;index;column:order" json:"order"`
	DurationMinutes int            `gorm:"default:0;column:duration_minutes" json:"durationMinutes"`
	IsFree          bool           `gorm:"default:false;index;column:is_free" json:"isFree"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Topic *Topic `gorm:"foreignKey:TopicID" json:"topic,omitempty"`

	// Phase 1: Advanced lesson fields (migration 0109)
	AudioUrl             *string    `gorm:"column:audio_url" json:"audioUrl,omitempty"`
	AudioDurationSeconds int        `gorm:"default:0;column:audio_duration_seconds" json:"audioDurationSeconds"`
	ExternalLinkUrl      *string    `gorm:"column:external_link_url" json:"externalLinkUrl,omitempty"`
	ExternalLinkTitle    *string    `gorm:"column:external_link_title" json:"externalLinkTitle,omitempty"`
	IsDripEnabled        bool       `gorm:"default:false;column:is_drip_enabled" json:"isDripEnabled"`
	DripReleaseDate      *time.Time `gorm:"column:drip_release_date" json:"dripReleaseDate,omitempty"`
	IsContentProtected   bool       `gorm:"default:false;column:is_content_protected" json:"isContentProtected"`
	SubtitleUrls         []byte     `gorm:"type:jsonb;column:subtitle_urls" json:"-"`
	VideoChaptersData    []byte     `gorm:"type:jsonb;column:video_chapters" json:"-"`
	// Denormalized stats
	ViewCount           int `gorm:"default:0;column:view_count" json:"viewCount"`
	CompletionCount     int `gorm:"default:0;column:completion_count" json:"completionCount"`
	AvgWatchTimeSeconds int `gorm:"default:0;column:avg_watch_time_seconds" json:"avgWatchTimeSeconds"`

	// Non-DB mapped fields
	Subtitles []LessonSubtitle `gorm:"-" json:"subtitles,omitempty"`
	Chapters  []VideoChapter   `gorm:"-" json:"chapters,omitempty"`

	// Relations
	Attachments []LessonAttachment `gorm:"foreignKey:SubTopicID;constraint:OnDelete:CASCADE" json:"attachments,omitempty"`
	Exam        *Exam              `gorm:"foreignKey:ExamID" json:"exam,omitempty"`
}

type LessonAttachment struct {
	ID         string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubTopicID string         `gorm:"not null;index;type:uuid;column:sub_topic_id;constraint:OnDelete:CASCADE" json:"subTopicId"`
	Title      string         `gorm:"not null;column:title" json:"title"`
	FileUrl    string         `gorm:"not null;column:file_url" json:"fileUrl"`
	FileType   string         `gorm:"column:file_type" json:"fileType"` // PDF, ZIP, etc.
	FileSize   int64          `gorm:"column:file_size" json:"fileSize"`
	CreatedAt  time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

type CourseReview struct {
	ID        string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID string         `gorm:"not null;index:idx_user_subject_review,unique;type:uuid;column:subject_id;constraint:OnDelete:CASCADE" json:"subjectId"`
	UserID    string         `gorm:"not null;index:idx_user_subject_review,unique;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Rating    int            `gorm:"not null;default:5;column:rating" json:"rating"`
	Comment   string         `gorm:"type:text;column:comment" json:"comment"`
	IsVisible bool           `gorm:"default:true;column:is_visible" json:"isVisible"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Subject) TableName() string {
	return "Subject"
}

func (s *Subject) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return
}

func (Topic) TableName() string {
	return "Topic"
}

func (t *Topic) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return
}

func (SubTopic) TableName() string {
	return "SubTopic"
}

func (st *SubTopic) BeforeCreate(tx *gorm.DB) (err error) {
	if st.ID == "" {
		st.ID = uuid.New().String()
	}
	return
}

func (LessonAttachment) TableName() string {
	return "LessonAttachment"
}

func (la *LessonAttachment) BeforeCreate(tx *gorm.DB) (err error) {
	if la.ID == "" {
		la.ID = uuid.New().String()
	}
	return
}

func (CourseReview) TableName() string {
	return "CourseReview"
}

func (cr *CourseReview) BeforeCreate(tx *gorm.DB) (err error) {
	if cr.ID == "" {
		cr.ID = uuid.New().String()
	}
	cr.IsVisible = true
	return
}
