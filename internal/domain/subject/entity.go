package subject

import (
	"time"
)

// ============================================================================
// Core Entity
// ============================================================================

type Subject struct {
	ID                     string
	Name                   string
	NameAr                 *string
	Code                   *string
	Description            *string
	Icon                   *string
	Color                  *string
	Type                   string
	Level                  *string
	Slug                   *string
	ThumbnailUrl           *string
	TrailerUrl             *string
	SeoTitle               *string
	SeoDescription         *string
	InstructorName         *string
	InstructorId           *string
	CategoryId             *string
	Price                  float64
	IsFree                 bool
	IsPublished            bool
	IsActive               bool
	IsFeatured             bool
	Rating                 float64
	EnrolledCount          int
	DurationHours          *float64
	TrailerDurationMinutes *int
	Language               *string
	Requirements           []string
	LearningObjectives     []string
	CoursePrerequisites    []string
	TargetAudience         []string
	WhatYouLearn           []string
	VideoCount             int
	CompletionRate         float64
	Topics                 []Topic
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type Topic struct {
	ID        string
	SubjectID string
	Title     string
	Order     int
	SubTopics []SubTopic
	CreatedAt time.Time
}

type SubTopic struct {
	ID          string
	TopicID     string
	Title       string
	Type        string
	Order       int
	IsFree      bool
	VideoUrl    *string
	Duration    int
	DurationMin int
	Description *string
	CreatedAt   time.Time
}

// ============================================================================
// Input Types
// ============================================================================

type CreateSubjectInput struct {
	Name                   string
	NameAr                 *string
	Code                   *string
	Description            *string
	Icon                   *string
	Color                  *string
	Type                   string
	Level                  *string
	Slug                   *string
	ThumbnailUrl           *string
	TrailerUrl             *string
	SeoTitle               *string
	SeoDescription         *string
	InstructorName         *string
	InstructorId           *string
	CategoryId             *string
	Price                  float64
	IsFree                 bool
	IsPublished            bool
	IsActive               bool
	IsFeatured             bool
	Language               *string
	DurationHours          *float64
	TrailerDurationMinutes *int
	Requirements           []string
	LearningObjectives     []string
	CoursePrerequisites    []string
	TargetAudience         []string
	WhatYouLearn           []string
}

type UpdateSubjectInput struct {
	ID                     string
	Name                   *string
	NameAr                 *string
	Code                   *string
	Description            *string
	Icon                   *string
	Color                  *string
	Type                   *string
	Level                  *string
	Slug                   *string
	ThumbnailUrl           *string
	TrailerUrl             *string
	SeoTitle               *string
	SeoDescription         *string
	InstructorName         *string
	InstructorId           *string
	CategoryId             *string
	Price                  *float64
	IsFree                 *bool
	IsPublished            *bool
	IsActive               *bool
	IsFeatured             *bool
	Language               *string
	DurationHours          *float64
	TrailerDurationMinutes *int
	VideoCount             *int
	Requirements           []string
	LearningObjectives     []string
	CoursePrerequisites    []string
	TargetAudience         []string
	WhatYouLearn           []string
}

type ListSubjectsFilter struct {
	CategoryID        *string
	Level             *string
	IsPublished       *bool
	IsActive          *bool
	IsFeatured        *bool
	Search            *string
	IncludeUnpublished bool
	SortBy            string
	SortOrder         string
	Page              int
	Limit             int
}

type ListSubjectsResult struct {
	Subjects   []Subject
	Total      int64
	Page       int
	Limit      int
	TotalPages int64
}

// ============================================================================
// Curriculum Types
// ============================================================================

type CurriculumInput struct {
	Topics []TopicInput
}

type TopicInput struct {
	ID        string
	Title     string
	Order     int
	SubTopics []SubTopicInput
}

type SubTopicInput struct {
	ID          string
	Title       string
	Type        string
	Order       int
	IsFree      bool
	VideoUrl    *string
	Duration    int
	DurationMin int
	Description *string
}

type Curriculum struct {
	Topics           []Topic
	ChaptersCount    int
	LessonsCount     int
	FreeLessonsCount int
	TotalDuration    int
}

// ============================================================================
// Detail / Response Types
// ============================================================================

type SubjectDetail struct {
	Subject Subject   `json:"subject"`
	IsEnrolled       bool       `json:"isEnrolled"`
	Progress         float64    `json:"progress"`
	EnrolledAt       *time.Time `json:"enrolledAt"`
	TotalLessons     int        `json:"totalLessons"`
	CompletedLessons int        `json:"completedLessons"`
}

type Enrollment struct {
	ID         string
	UserID     string
	SubjectID  string
	Progress   float64
	EnrolledAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type EnrollmentStatus struct {
	IsEnrolled      bool       `json:"isEnrolled"`
	Status          string     `json:"status"`
	Progress        float64    `json:"progress"`
	EnrolledAt      *time.Time `json:"enrolledAt"`
	TotalLessons    int64      `json:"totalLessons"`
	CompletedLessons int64     `json:"completedLessons"`
	PaymentRequired bool       `json:"paymentRequired"`
	Price           float64    `json:"price"`
}

type UserCourse struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	SubjectID    string    `json:"subjectId"`
	Progress     float64   `json:"progress"`
	EnrolledAt   time.Time `json:"enrolledAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	SubjectName   string   `json:"subjectName"`
	SubjectNameAr *string  `json:"subjectNameAr"`
	ThumbnailUrl  *string  `json:"thumbnailUrl"`
	Rating        float64  `json:"rating"`
	Level         *string  `json:"level"`
}

type StudentsFilter struct {
	Page     int
	Limit    int
	Progress string // "completed", "in_progress", "not_started"
}

type StudentsResult struct {
	Students []StudentInfo
	Total    int64
	Page     int
	Limit    int
	TotalPages int64
}

type StudentInfo struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Avatar     *string   `json:"avatar"`
	Progress   float64   `json:"progress"`
	EnrolledAt time.Time `json:"enrolledAt"`
	Completed  bool      `json:"completed"`
}

// ============================================================================
// Lesson Progress Types
// ============================================================================

type LessonProgressData struct {
	Completed           bool
	Status              string
	TimeSpentSeconds    int
	LastWatchedPosition int
}

type ProgressInput struct {
	Completed           bool    `json:"completed"`
	LastWatchedPosition float64 `json:"lastWatchedPosition"`
	TimeSpentSeconds    int     `json:"timeSpentSeconds"`
	Status              string  `json:"status"`
}

type LessonProgressInfo struct {
	LessonID            string  `json:"lessonId"`
	Completed           bool    `json:"completed"`
	Status              string  `json:"status"`
	TimeSpentSeconds    int     `json:"timeSpentSeconds"`
	LastWatchedPosition int     `json:"lastWatchedPosition"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// ============================================================================
// Review Types
// ============================================================================

type Review struct {
	ID        string    `json:"id"`
	SubjectID string    `json:"subjectId"`
	UserID    string    `json:"userId"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	IsVisible bool      `json:"isVisible"`
	CreatedAt time.Time `json:"createdAt"`
	UserName  string    `json:"userName"`
	UserAvatar *string  `json:"userAvatar"`
}

type CreateReviewInput struct {
	Rating  int    `json:"rating" binding:"required"`
	Comment string `json:"comment"`
}

type ReviewStats struct {
	TotalReviews int64            `json:"totalReviews"`
	AvgRating    float64          `json:"avgRating"`
	Distribution map[int]int64    `json:"distribution"`
}

// ============================================================================
// Event Types
// ============================================================================

const (
	EventSubjectCreated     = "subject.created"
	EventSubjectUpdated     = "subject.updated"
	EventSubjectDeleted     = "subject.deleted"
	EventSubjectDuplicated  = "subject.duplicated"
	EventCurriculumUpdated  = "subject.curriculum_updated"
	EventUserEnrolled       = "subject.user_enrolled"
	EventUserUnenrolled     = "subject.user_unenrolled"
	EventCourseCompleted    = "subject.course_completed"
	EventReviewCreated = "subject.review_created"
)