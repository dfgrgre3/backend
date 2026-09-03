package courseservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	models "thanawy-backend/internal/domain/common"

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

// Handle handles the enrollment command with proper context propagation and error handling
func (h *EnrollUserHandler) Handle(ctx context.Context, cmd EnrollUserCommand) (*models.LmsEnrollment, error) {
	// Check if already enrolled
	var existingEnrollment models.LmsEnrollment
	err := h.db.WithContext(ctx).Where("course_id = ? AND user_id = ?", cmd.CourseID, cmd.UserID).First(&existingEnrollment).Error

	// Distinguish between "already enrolled" and database errors
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check enrollment status: %w", err)
	}

	// If no error and record found, user is already enrolled
	if err == nil {
		return nil, fmt.Errorf("user already enrolled in course")
	}

	// Parse IDs with proper error handling
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

	// Create enrollment with context
	if err := h.db.WithContext(ctx).Create(enrollment).Error; err != nil {
		return nil, fmt.Errorf("failed to create enrollment: %w", err)
	}
	return enrollment, nil
}
