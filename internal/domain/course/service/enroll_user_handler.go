package courseservice

import (
	"context"
	models "thanawy-backend/internal/domain/common"
	"time"

	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// EnrollUserHandler handles user enrollment in courses
type EnrollUserHandler struct {
	db *gorm.DB
}

// NewEnrollUserHandler creates a new EnrollUserHandler
func NewEnrollUserHandler(db *gorm.DB) *EnrollUserHandler {
	return &EnrollUserHandler{db: db}
}

// Handle handles the enrollment command
func (h *EnrollUserHandler) Handle(ctx context.Context, cmd EnrollUserCommand) (*models.LmsEnrollment, error) {
	// Check if already enrolled
	var existingEnrollment models.LmsEnrollment
	if err := h.db.Where("course_id = ? AND user_id = ?", cmd.CourseID, cmd.UserID).First(&existingEnrollment).Error; err == nil {
		return nil, fmt.Errorf("user already enrolled in course")
	}

	courseID, err := uuid.Parse(cmd.CourseID)
	if err != nil {
		return nil, fmt.Errorf("invalid course ID: %w", err)
	}
	userID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	enrollment := &models.LmsEnrollment{
		CourseID:   courseID,
		UserID:     userID,
		Progress:   decimal.NewFromInt(0),
		EnrolledAt: time.Now(),
	}

	if err := h.db.Create(enrollment).Error; err != nil {
		return nil, err
	}
	return enrollment, nil
}
