package courseservice

import (
	"context"
	"errors"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetCourseHandler handles course queries
type GetCourseHandler struct {
	db *gorm.DB
}

// NewGetCourseHandler creates a new GetCourseHandler
func NewGetCourseHandler(db *gorm.DB) *GetCourseHandler {
	return &GetCourseHandler{db: db}
}

// Handle handles the get course query
func (h *GetCourseHandler) Handle(ctx context.Context, query GetCourseQuery) (interface{}, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("handler not initialized")
	}

	var c models.LmsCourse
	q := h.db.WithContext(ctx)

	if query.ID != "" {
		idUUID, err := uuid.Parse(query.ID)
		if err == nil {
			if err := q.First(&c, "id = ?", idUUID).Error; err == nil {
				return &c, nil
			}
		}
	}

	if query.Slug != "" {
		if err := q.First(&c, "slug = ?", query.Slug).Error; err == nil {
			return &c, nil
		}
	}

	return nil, gorm.ErrRecordNotFound
}
