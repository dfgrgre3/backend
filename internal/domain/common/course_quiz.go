package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CourseQuiz is the persisted quiz contract used by the education frontend.
// Questions are kept as JSON because the supported question shapes are
// intentionally extensible; attempts are stored separately and never trusted
// for grading metadata.
type CourseQuiz struct {
	ID                     string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID               string          `gorm:"not null;index;type:uuid;column:course_id" json:"courseId"`
	LessonID               *string         `gorm:"index;type:uuid;column:lesson_id" json:"lessonId,omitempty"`
	Title                  string          `gorm:"not null;column:title" json:"title"`
	Description            string          `gorm:"type:text;column:description" json:"description,omitempty"`
	Instructions           string          `gorm:"type:text;column:instructions" json:"instructions,omitempty"`
	TimeLimitMinutes       *int            `gorm:"column:time_limit_minutes" json:"timeLimitMinutes,omitempty"`
	PassingScore           float64         `gorm:"not null;default:60;column:passing_score" json:"passingScore"`
	MaxAttempts            int             `gorm:"not null;default:1;column:max_attempts" json:"maxAttempts"`
	Required               bool            `gorm:"not null;default:true;column:required" json:"required"`
	ShuffleQuestions       bool            `gorm:"not null;default:false;column:shuffle_questions" json:"shuffleQuestions"`
	ShuffleOptions         bool            `gorm:"not null;default:false;column:shuffle_options" json:"shuffleOptions"`
	ShowResultsImmediately bool            `gorm:"not null;default:true;column:show_results_immediately" json:"showResultsImmediately"`
	ShowCorrectAnswers     bool            `gorm:"not null;default:false;column:show_correct_answers" json:"showCorrectAnswers"`
	AllowReview            bool            `gorm:"not null;default:true;column:allow_review" json:"allowReview"`
	Status                 string          `gorm:"not null;default:'draft';column:status" json:"status"`
	Questions              json.RawMessage `gorm:"type:jsonb;not null;column:questions" json:"questions"`
	CreatedAt              time.Time       `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              time.Time       `gorm:"column:updated_at" json:"updatedAt"`
	CreatedBy              string          `gorm:"not null;type:uuid;column:created_by" json:"createdBy"`
}

func (CourseQuiz) TableName() string { return "CourseQuiz" }

func (q *CourseQuiz) BeforeCreate(tx *gorm.DB) error {
	if q.ID == "" {
		q.ID = uuid.NewString()
	}
	return nil
}

type CourseQuizAttempt struct {
	ID               string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	QuizID           string          `gorm:"not null;index;type:uuid;column:quiz_id" json:"quizId"`
	CourseID         string          `gorm:"not null;index;type:uuid;column:course_id" json:"courseId"`
	UserID           string          `gorm:"not null;index;type:uuid;column:user_id" json:"userId"`
	Answers          json.RawMessage `gorm:"type:jsonb;not null;column:answers" json:"answers"`
	Score            float64         `gorm:"column:score" json:"score"`
	MaxScore         float64         `gorm:"column:max_score" json:"maxScore"`
	Percentage       float64         `gorm:"column:percentage" json:"percentage"`
	Passed           bool            `gorm:"column:passed" json:"passed"`
	Status           string          `gorm:"not null;default:'in_progress';column:status" json:"status"`
	TimeSpentSeconds int             `gorm:"column:time_spent_seconds" json:"timeSpentSeconds"`
	StartedAt        time.Time       `gorm:"column:started_at" json:"startedAt"`
	SubmittedAt      time.Time       `gorm:"column:submitted_at" json:"submittedAt"`
	GradedAt         time.Time       `gorm:"column:graded_at" json:"gradedAt"`
}

func (CourseQuizAttempt) TableName() string { return "CourseQuizAttempt" }

func (a *CourseQuizAttempt) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}
