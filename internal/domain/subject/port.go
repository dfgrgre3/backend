package subject

import (
	"context"
	"time"
)

type Repository interface {
	// Subject CRUD
	Create(ctx context.Context, subject *Subject) error
	FindByID(ctx context.Context, id string) (*Subject, error)
	FindBySlug(ctx context.Context, slug string) (*Subject, error)
	Update(ctx context.Context, subject *Subject) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ListSubjectsFilter) (ListSubjectsResult, error)

	// Curriculum
	UpdateCurriculum(ctx context.Context, subjectID string, curriculum CurriculumInput) error
	GetCurriculum(ctx context.Context, subjectID string) ([]Topic, error)
	UpdateVideoCount(ctx context.Context, subjectID string, count int) error

	// Stats / Counts
	CountByCategory(ctx context.Context, categoryID string) (int64, error)
	CountTotal(ctx context.Context) (int64, error)
	CountCompletedLessons(ctx context.Context, userID string, subjectID string) (int, error)

	// Enrollment
	GetEnrollment(ctx context.Context, userID string, subjectID string) (*Enrollment, error)
	CreateEnrollment(ctx context.Context, userID string, subjectID string) error
	DeleteEnrollment(ctx context.Context, userID string, subjectID string) error
	UpdateEnrollmentProgress(ctx context.Context, userID string, subjectID string, progress float64) error
	GetUserEnrollments(ctx context.Context, userID string) ([]EnrollmentWithSubject, error)
	ListStudents(ctx context.Context, subjectID string, filter StudentsFilter) (StudentsResult, error)

	// Lesson Progress
	IsUserEnrolledInLessonCourse(ctx context.Context, userID string, lessonID string) (bool, error)
	GetSubjectIDByLesson(ctx context.Context, lessonID string) (string, error)
	UpsertLessonProgress(ctx context.Context, userID string, lessonID string, data LessonProgressData) error
	GetLessonProgressForSubject(ctx context.Context, userID string, subjectID string) ([]LessonProgressInfo, error)
	GetLessonProgressCounts(ctx context.Context, userID string, subjectID string) (total int64, completed int64, err error)

	// Payment Checks
	HasUserPaid(ctx context.Context, userID string, subjectID string) (bool, error)

	// Reviews
	CreateReview(ctx context.Context, review *Review) error
	GetReviews(ctx context.Context, subjectID string) ([]Review, error)
	GetReviewStats(ctx context.Context, subjectID string) (*ReviewStats, error)
	GetUserReview(ctx context.Context, userID string, subjectID string) (*Review, error)
	UpdateSubjectRating(ctx context.Context, subjectID string) error

	// Batch Operations
	BatchUpdate(ctx context.Context, ids []string, updates map[string]interface{}) error
	BatchDelete(ctx context.Context, ids []string) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event SubjectEvent) error
}

type SubjectEvent struct {
	Type      string
	SubjectID string
	Timestamp time.Time
	Data      map[string]interface{}
}

// EnrollmentWithSubject is used when fetching enrollments along with subject data
type EnrollmentWithSubject struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	SubjectID  string    `json:"subjectId"`
	Progress   float64   `json:"progress"`
	EnrolledAt time.Time `json:"enrolledAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	SubjectName    string   `json:"subjectName"`
	SubjectNameAr  *string  `json:"subjectNameAr"`
	SubjectCode    *string  `json:"subjectCode"`
	ThumbnailUrl   *string  `json:"thumbnailUrl"`
	Rating         float64  `json:"rating"`
	Level          *string  `json:"level"`
	SubjectActive  bool     `json:"subjectActive"`
	SubjectPublished bool   `json:"subjectPublished"`
}
