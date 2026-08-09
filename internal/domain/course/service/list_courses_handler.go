package courseservice

import (
	"context"
	"fmt"
	models "thanawy-backend/internal/domain/common"

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
	if query.HasCertificate != nil {
		q = q.Where("has_certificate = ?", *query.HasCertificate)
	}
	if query.PublishedAfter != nil && *query.PublishedAfter != "" {
		q = q.Where("created_at >= ?", *query.PublishedAfter)
	}
	if query.PublishedBefore != nil && *query.PublishedBefore != "" {
		q = q.Where("created_at <= ?", *query.PublishedBefore)
	}

	search := query.Search
	if search == nil {
		search = query.SearchQuery
	}
	if search != nil && *search != "" {
		q = q.Where("title ILIKE ? OR slug ILIKE ? OR short_description ILIKE ? OR long_description ILIKE ?",
			"%"+*search+"%", "%"+*search+"%", "%"+*search+"%", "%"+*search+"%")
	}

	// Category filter - join with LmsCourseCategory
	if query.CategoryID != nil && *query.CategoryID != "" {
		q = q.Joins("JOIN \"LmsCourseCategory\" lcc ON lcc.course_id = \"LmsCourse\".id").
			Where("lcc.category_id = ?", *query.CategoryID)
	}

	// Price filter - join with LmsPricing
	if query.MinPrice != nil || query.MaxPrice != nil {
		q = q.Joins("JOIN \"LmsPricing\" lp ON lp.course_id = \"LmsCourse\".id AND lp.deleted_at IS NULL")
		if query.MinPrice != nil {
			q = q.Where("lp.amount >= ?", *query.MinPrice)
		}
		if query.MaxPrice != nil {
			q = q.Where("lp.amount <= ?", *query.MaxPrice)
		}
		if query.PriceType != nil && *query.PriceType != "" {
			q = q.Where("lp.type = ?", *query.PriceType)
		}
	}

	// Tags filter - join with LmsCourseTag and LmsTag
	if len(query.Tags) > 0 {
		q = q.Joins("JOIN \"LmsCourseTag\" lct ON lct.course_id = \"LmsCourse\".id").
			Joins("JOIN \"LmsTag\" lt ON lt.id = lct.tag_id").
			Where("lt.name IN ?", query.Tags).
			Group("\"LmsCourse\".id").
			Having("COUNT(DISTINCT lt.name) = ?", len(query.Tags))
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

	// Sorting
	sortBy := query.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := query.SortOrder
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	orderClause := fmt.Sprintf("%s %s", sortBy, sortOrder)

	offset := (page - 1) * limit
	if err := q.Order(orderClause).Limit(limit).Offset(offset).Find(&courses).Error; err != nil {
		return nil, 0, err
	}

	if courses == nil {
		courses = []models.LmsCourse{}
	}

	return courses, total, nil
}
