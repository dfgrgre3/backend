package courseservice

import (
	"context"
	"fmt"
	"strings"
	models "thanawy-backend/internal/domain/common"

	"gorm.io/gorm"
)

// SearchCoursesHandler handles advanced course search queries
type SearchCoursesHandler struct {
	db *gorm.DB
}

// NewSearchCoursesHandler creates a new SearchCoursesHandler
func NewSearchCoursesHandler(db *gorm.DB) *SearchCoursesHandler {
	return &SearchCoursesHandler{db: db}
}

// Handle handles the search courses query
func (h *SearchCoursesHandler) Handle(ctx context.Context, query SearchCoursesQuery) (interface{}, int64, error) {
	if h == nil || h.db == nil {
		return []models.LmsCourse{}, 0, nil
	}

	var courses []models.LmsCourse
	var total int64

	q := h.db.WithContext(ctx).Model(&models.LmsCourse{})

	// Base filters from ListCoursesQuery
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

	// Full-text search
	searchQuery := query.Query
	if searchQuery == "" && query.Search != nil {
		searchQuery = *query.Search
	}
	if searchQuery == "" && query.SearchQuery != nil {
		searchQuery = *query.SearchQuery
	}
	if searchQuery != "" {
		// Use ILIKE for case-insensitive search across multiple fields
		searchTerm := "%" + searchQuery + "%"
		q = q.Where("title ILIKE ? OR slug ILIKE ? OR short_description ILIKE ? OR long_description ILIKE ?",
			searchTerm, searchTerm, searchTerm, searchTerm)
	}

	// Multiple categories (OR logic)
	if len(query.CategoryIDs) > 0 {
		q = q.Joins("JOIN \"LmsCourseCategory\" lcc ON lcc.course_id = \"LmsCourse\".id").
			Where("lcc.category_id IN ?", query.CategoryIDs).
			Group("\"LmsCourse\".id")
	} else if query.CategoryID != nil && *query.CategoryID != "" {
		q = q.Joins("JOIN \"LmsCourseCategory\" lcc ON lcc.course_id = \"LmsCourse\".id").
			Where("lcc.category_id = ?", *query.CategoryID)
	}

	// Multiple instructors (OR logic)
	if len(query.InstructorIDs) > 0 {
		q = q.Where("primary_instructor_id IN ?", query.InstructorIDs)
	}

	// Exclude specific course
	if query.ExcludeCourseID != nil && *query.ExcludeCourseID != "" {
		q = q.Where("id != ?", *query.ExcludeCourseID)
	}

	// Tags filter (ALL tags must match)
	if len(query.Tags) > 0 {
		q = q.Joins("JOIN \"LmsCourseTag\" lct ON lct.course_id = \"LmsCourse\".id").
			Joins("JOIN \"LmsTag\" lt ON lt.id = lct.tag_id").
			Where("lt.name IN ?", query.Tags).
			Group("\"LmsCourse\".id").
			Having("COUNT(DISTINCT lt.name) = ?", len(query.Tags))
	}

	// Price filter
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

	// Count total
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	page := query.Page
	if page < 1 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 20
	}

	// Sorting
	sortBy := query.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	// Validate sortBy to prevent SQL injection
	allowedSortFields := map[string]bool{
		"title":                   true,
		"created_at":              true,
		"updated_at":              true,
		"estimated_duration_mins": true,
		"level":                   true,
		"status":                  true,
	}
	if !allowedSortFields[sortBy] {
		sortBy = "created_at"
	}

	sortOrder := strings.ToUpper(query.SortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
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
