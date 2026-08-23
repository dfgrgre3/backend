package courseservice

import (
	"context"
	"fmt"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetEnrollmentHandler handles enrollment queries
type GetEnrollmentHandler struct {
	db *gorm.DB
}

// NewGetEnrollmentHandler creates a new GetEnrollmentHandler
func NewGetEnrollmentHandler(db *gorm.DB) *GetEnrollmentHandler {
	return &GetEnrollmentHandler{db: db}
}

// Handle handles the get enrollment query
func (h *GetEnrollmentHandler) Handle(ctx context.Context, query GetEnrollmentQuery) (*models.LmsEnrollment, error) {
	courseID, err := uuid.Parse(query.CourseID)
	if err != nil {
		return nil, fmt.Errorf("invalid course ID: %w", err)
	}
	userID, err := uuid.Parse(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	var enrollment models.LmsEnrollment
	if err := h.db.WithContext(ctx).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		First(&enrollment).Error; err != nil {
		return nil, err
	}
	return &enrollment, nil
}
