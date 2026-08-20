package commands

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	db "thanawy-backend/internal/infrastructure/database"
)

// --- Command DTOs ---

type RecordLessonProgressCommand struct {
	UserID           string
	SubTopicID       string
	TimeSpentSeconds int
	Completed        bool
}

type RecordExamCompletedCommand struct {
	UserID string
	ExamID string
	Score  float64
	Passed bool
}

type RecordTaskCompletedCommand struct {
	UserID string
	TaskID string
}

type RecordStudySessionCommand struct {
	UserID          string
	DurationMinutes int
}

// --- Service ---

// ProgressCommandService handles all write operations for learner progress.
type ProgressCommandService struct{}

// NewProgressCommandService creates a new instance of ProgressCommandService.
func NewProgressCommandService() *ProgressCommandService {
	return &ProgressCommandService{}
}

// getWriteDB safely retrieves the write database connection with lazy initialization.
func (s *ProgressCommandService) getWriteDB() (*gorm.DB, error) {
	wdb := db.WriteDB()
	if wdb == nil {
		return nil, errors.New("database write connection is not initialized")
	}
	return wdb, nil
}

// --- Constants ---

const (
	StatusCompleted  = "COMPLETED"
	StatusInProgress = "IN_PROGRESS"
	StatusDone       = "DONE"

	MinValidScore = 0.0
	MaxValidScore = 100.0
)

// --- Validation Helpers ---

func validateUserID(userID string) error {
	if userID == "" {
		return errors.New("userID cannot be empty")
	}
	return nil
}

func validateTimeSpent(seconds int) error {
	if seconds < 0 {
		return errors.New("time spent cannot be negative")
	}
	return nil
}

func validateScore(score float64) error {
	if score < MinValidScore || score > MaxValidScore {
		return errors.New("score must be between 0 and 100")
	}
	return nil
}

// --- Command Handlers ---

// RecordLessonCompleted marks a sub-topic as fully completed and accumulates watch time.
func (s *ProgressCommandService) RecordLessonCompleted(ctx context.Context, cmd RecordLessonProgressCommand) error {
	if err := validateUserID(cmd.UserID); err != nil {
		return err
	}
	if cmd.SubTopicID == "" {
		return errors.New("subTopicID cannot be empty")
	}
	if err := validateTimeSpent(cmd.TimeSpentSeconds); err != nil {
		return err
	}

	wdb, err := s.getWriteDB()
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	return wdb.WithContext(ctx).Exec(`
		INSERT INTO "TopicProgress" ("id", "user_id", "sub_topic_id", "status", "completed", "time_spent_seconds", "last_watched_position", "created_at", "updated_at")
		VALUES (gen_random_uuid()::text, $1, $2, $3, true, $4, 0, $5, $5)
		ON CONFLICT ("user_id", "sub_topic_id")
		DO UPDATE SET
			"status" = $3,
			"completed" = true,
			"time_spent_seconds" = "TopicProgress"."time_spent_seconds" + EXCLUDED."time_spent_seconds",
			"updated_at" = EXCLUDED."updated_at"
	`, cmd.UserID, cmd.SubTopicID, StatusCompleted, cmd.TimeSpentSeconds, now).Error
}

// RecordLessonProgress updates watch time without necessarily marking the lesson as completed.
func (s *ProgressCommandService) RecordLessonProgress(ctx context.Context, cmd RecordLessonProgressCommand) error {
	if err := validateUserID(cmd.UserID); err != nil {
		return err
	}
	if cmd.SubTopicID == "" {
		return errors.New("subTopicID cannot be empty")
	}
	if err := validateTimeSpent(cmd.TimeSpentSeconds); err != nil {
		return err
	}

	wdb, err := s.getWriteDB()
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	return wdb.WithContext(ctx).Exec(`
		INSERT INTO "TopicProgress" ("id", "user_id", "sub_topic_id", "status", "completed", "time_spent_seconds", "last_watched_position", "created_at", "updated_at")
		VALUES (gen_random_uuid()::text, $1, $2, $3, false, $4, 0, $5, $5)
		ON CONFLICT ("user_id", "sub_topic_id")
		DO UPDATE SET
			"status" = CASE WHEN "TopicProgress"."status" = $6 THEN $6 ELSE $3 END,
			"completed" = CASE WHEN "TopicProgress"."completed" = true THEN true ELSE false END,
			"time_spent_seconds" = "TopicProgress"."time_spent_seconds" + EXCLUDED."time_spent_seconds",
			"updated_at" = EXCLUDED."updated_at"
	`, cmd.UserID, cmd.SubTopicID, StatusInProgress, cmd.TimeSpentSeconds, now, StatusCompleted).Error
}

// RecordExamCompleted records a new exam attempt.
func (s *ProgressCommandService) RecordExamCompleted(ctx context.Context, cmd RecordExamCompletedCommand) error {
	if err := validateUserID(cmd.UserID); err != nil {
		return err
	}
	if cmd.ExamID == "" {
		return errors.New("examID cannot be empty")
	}
	if err := validateScore(cmd.Score); err != nil {
		return err
	}

	wdb, err := s.getWriteDB()
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	// Note: Consider adding an idempotency key or UNIQUE constraint on (exam_id, user_id, taken_at)
	// to prevent duplicate records from network retries.
	return wdb.WithContext(ctx).Exec(`
		INSERT INTO "ExamResult" ("id", "exam_id", "user_id", "score", "passed", "taken_at", "created_at", "updated_at")
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $5, $5)
	`, cmd.ExamID, cmd.UserID, cmd.Score, cmd.Passed, now).Error
}

// RecordTaskCompleted marks a specific task as done for a user.
func (s *ProgressCommandService) RecordTaskCompleted(ctx context.Context, cmd RecordTaskCompletedCommand) error {
	if err := validateUserID(cmd.UserID); err != nil {
		return err
	}
	if cmd.TaskID == "" {
		return errors.New("taskID cannot be empty")
	}

	wdb, err := s.getWriteDB()
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	result := wdb.WithContext(ctx).Exec(`
		UPDATE "Task" SET "status" = $1, "updated_at" = $2
		WHERE "id" = $3 AND "user_id" = $4
	`, StatusDone, now, cmd.TaskID, cmd.UserID)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RecordStudySession logs a completed study session with its duration.
func (s *ProgressCommandService) RecordStudySession(ctx context.Context, cmd RecordStudySessionCommand) error {
	if err := validateUserID(cmd.UserID); err != nil {
		return err
	}
	if cmd.DurationMinutes <= 0 {
		return errors.New("duration must be greater than 0 minutes")
	}

	wdb, err := s.getWriteDB()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	endTime := now.Add(time.Duration(cmd.DurationMinutes) * time.Minute)

	return wdb.WithContext(ctx).Exec(`
		INSERT INTO "StudySession" ("id", "user_id", "duration_min", "start_time", "end_time", "created_at", "updated_at")
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $3, $3)
	`, cmd.UserID, cmd.DurationMinutes, now, endTime).Error
}
