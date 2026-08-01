package course

import (
	"context"
	"thanawy-backend/internal/models"

	"gorm.io/gorm"
)

// ListCoursesHandler handles course listing queries
type ListCoursesHandler struct {
	db *gorm.DB
}

// NewListCoursesHandler creates a new ListCoursesHandler
func NewListCoursesHandler(db *gorm.DB) *ListCoursesHandler {
	return &ListCoursesHandler{db: db}
}

// Handle handles the list courses query
func (h *ListCoursesHandler) Handle(ctx context.Context, query ListCoursesQuery) (interface{}, int64, error) {
	if h == nil || h.db == nil {
		return []models.LmsCourse{}, 0, nil
	}

	var courses []models.LmsCourse
	var total int64

	q := h.db.WithContext(ctx).Model(&models.LmsCourse{})

	if query.Status != nil && *query.Status != "" {
		q = q.Where("status = ?", *query.Status)
	}
	if query.Level != nil && *query.Level != "" {
		q = q.Where("level = ?", *query.Level)
	}
	if query.Language != nil && *query.Language != "" {
		q = q.Where("language = ?", *query.Language)
	}
	if query.InstructorID != nil && *query.InstructorID != "" {
		q = q.Where("primary_instructor_id = ?", *query.InstructorID)
	}
	if query.IsFeatured != nil {
		q = q.Where("is_featured = ?", *query.IsFeatured)
	}
	if query.IsTrending != nil {
		q = q.Where("is_trending = ?", *query.IsTrending)
	}
	if query.IsNew != nil {
		q = q.Where("is_new = ?", *query.IsNew)
	}

	search := query.Search
	if search == nil {
		search = query.SearchQuery
	}
	if search != nil && *search != "" {
		q = q.Where("title ILIKE ? OR slug ILIKE ?", "%"+*search+"%", "%"+*search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = query.PageSize
	}
	if limit < 1 {
		limit = 20
	}

	offset := (page - 1) * limit
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&courses).Error; err != nil {
		return nil, 0, err
	}

	if courses == nil {
		courses = []models.LmsCourse{}
	}

	return courses, total, nil
}
