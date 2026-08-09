package gamificationservice

import (
	"fmt"
	db "thanawy-backend/internal/infrastructure/database"
	userrepo "thanawy-backend/internal/infrastructure/persistence/repositories"

	"gorm.io/gorm"
)

type AwardXPCommand struct {
	UserID   string
	XPType   string
	XPAmount int
	Source   string
	SourceID string
}

type BatchFlushCommand struct {
	UserID string
}

type GamificationCommandService struct {
	writeDB *gorm.DB
}

func NewGamificationCommandService() *GamificationCommandService {
	return &GamificationCommandService{writeDB: db.WriteDB()}
}

func (s *GamificationCommandService) AwardXP(cmd AwardXPCommand) error {
	if cmd.XPAmount <= 0 {
		return nil
	}

	var updateColumn string
	switch cmd.XPType {
	case "study":
		updateColumn = "study_xp"
	case "task":
		updateColumn = "task_xp"
	case "exam":
		updateColumn = "exam_xp"
	case "challenge":
		updateColumn = "challenge_xp"
	case "quest":
		updateColumn = "quest_xp"
	case "season":
		updateColumn = "season_xp"
	default:
		updateColumn = "total_xp"
	}

	result := s.writeDB.Exec(fmt.Sprintf(`
		UPDATE "User" SET
			total_xp = total_xp + $1,
			%s = %s + $1,
			level = FLOOR(((total_xp + $1) / 100)) + 1,
			updated_at = NOW()
		WHERE id = $2
	`, updateColumn, updateColumn), cmd.XPAmount, cmd.UserID)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	// Invalidate the user cache so that updated XP/level is fetched on next /auth/me or profile query
	userrepo.NewUserRepository(s.writeDB).InvalidateCache(cmd.UserID)

	return nil
}

func (s *GamificationCommandService) FlushUserProgress(cmd BatchFlushCommand) error {
	err := s.writeDB.Exec(`
		UPDATE "User" SET
			total_study_time = COALESCE((
				SELECT SUM(time_spent_seconds) / 60
				FROM "TopicProgress"
				WHERE user_id = $1 AND deleted_at IS NULL
			), 0) + COALESCE((
				SELECT SUM(duration_min)
				FROM "StudySession"
				WHERE user_id = $1 AND deleted_at IS NULL
			), 0),
			tasks_completed = (
				SELECT COUNT(*) FROM "Task"
				WHERE user_id = $1 AND (status = 'DONE' OR status = 'COMPLETED') AND deleted_at IS NULL
			),
			exams_passed = (
				SELECT COUNT(*) FROM "ExamResult"
				WHERE user_id = $1 AND passed = true AND deleted_at IS NULL
			),
			updated_at = NOW()
		WHERE id = $1
	`, cmd.UserID).Error

	if err == nil {
		// Invalidate the user cache
		userrepo.NewUserRepository(s.writeDB).InvalidateCache(cmd.UserID)
	}

	return err
}
