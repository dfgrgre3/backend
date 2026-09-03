package courseservice

import (
	"context"
	"fmt"
	"time"

	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UpdateProgressHandler handles progress update commands
type UpdateProgressHandler struct {
	db *gorm.DB
}

// NewUpdateProgressHandler creates a new UpdateProgressHandler
func NewUpdateProgressHandler(db *gorm.DB) *UpdateProgressHandler {
	return &UpdateProgressHandler{db: db}
}

// Handle handles the update progress command with proper context propagation
func (h *UpdateProgressHandler) Handle(ctx context.Context, cmd UpdateProgressCommand) error {
	// Validate IDs
	courseID, err := uuid.Parse(cmd.CourseID)
	if err != nil {
		return fmt.Errorf("invalid course ID: %w", err)
	}
	userID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Validate progress value
	if cmd.Progress < 0 || cmd.Progress > 100 {
		return fmt.Errorf("progress must be between 0 and 100")
	}

	// Build update map
	updates := map[string]interface{}{
		"progress":   cmd.Progress,
		"updated_at": time.Now(),
	}

	// Auto-set completion time when progress reaches 100%
	if cmd.Progress >= 100 {
		now := time.Now()
		updates["completed_at"] = &now
	}

	// Execute update with context propagation
	result := h.db.WithContext(ctx).Model(&models.LmsEnrollment{}).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update progress: %w", result.Error)
	}

	// Check if any rows were actually affected
	if result.RowsAffected == 0 {
		return fmt.Errorf("enrollment not found for course_id=%s and user_id=%s", courseID, userID)
	}

	return nil
}
