package query

import (
	"context"

	"gorm.io/gorm"
)

// QueryOptimizer provides optimized query patterns for common operations
type QueryOptimizer struct {
	db *gorm.DB
}

func NewQueryOptimizer(db *gorm.DB) *QueryOptimizer {
	return &QueryOptimizer{db: db}
}

// OptimizedSubjectListQuery builds an optimized query for subject listing.
// Uses covering indexes and avoids N+1 queries.
func (qo *QueryOptimizer) OptimizedSubjectListQuery(ctx context.Context, filters SubjectFilters, page, limit int) ([]SubjectListItem, int64, error) {
	var items []SubjectListItem
	var total int64

	// Build base query with only needed columns
	query := qo.db.WithContext(ctx).
		Table("Subject").
		Select(`
			id, name, name_ar, code, description, icon, color,
			is_active, is_published, price, level, instructor_name,
			instructor_id, category_id, thumbnail_url, trailer_url,
			trailer_duration_minutes, duration_hours, requirements,
			learning_objectives, seo_title, seo_description, slug,
			rating, enrolled_count, created_at, updated_at
		`)

	// Apply filters
	query = qo.applySubjectFilters(query, filters)

	// Count total with same filters — single query via subquery approach
	countQuery := qo.db.WithContext(ctx).
		Table("Subject").
		Select("count(*)")
	countQuery = qo.applySubjectFilters(countQuery, filters)
	if err := countQuery.Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, 0, err
	}

	// Fetch topic counts in a single query (avoid N+1)
	if len(items) > 0 {
		subjectIDs := make([]string, len(items))
		for i, item := range items {
			subjectIDs[i] = item.ID
		}
		topicCounts := qo.fetchTopicCounts(ctx, subjectIDs)
		for i := range items {
			if count, ok := topicCounts[items[i].ID]; ok {
				items[i].TopicCount = count
			}
		}
	}

	return items, total, nil
}

// applySubjectFilters applies the given filters to the query.
func (qo *QueryOptimizer) applySubjectFilters(query *gorm.DB, filters SubjectFilters) *gorm.DB {
	if filters.CategoryID != "" {
		query = query.Where("category_id = ?", filters.CategoryID)
	}
	if filters.Search != "" {
		// Use trigram search when the GIN index is available; fall back to ILIKE
		// for compatibility on unindexed columns.
		searchTerm := "%" + filters.Search + "%"
		query = query.Where("name ILIKE ? OR name_ar ILIKE ?", searchTerm, searchTerm)
	}
	if filters.Level != "" {
		query = query.Where("level = ?", filters.Level)
	}
	if filters.IsPublished != nil {
		query = query.Where("is_published = ?", *filters.IsPublished)
	}
	if filters.IsActive != nil {
		query = query.Where("is_active = ?", *filters.IsActive)
	}
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.IsFeatured {
		query = query.Where("is_featured = ?", true)
	}
	if filters.IsTrending {
		query = query.Where("is_trending = ?", true)
	}
	if filters.IsNew {
		query = query.Where("is_new = ?", true)
	}
	return query
}

// fetchTopicCounts returns a map of subjectID → topic count for the given IDs.
// Uses a single IN-clause query that hits idx_topic_subject_order.
func (qo *QueryOptimizer) fetchTopicCounts(ctx context.Context, subjectIDs []string) map[string]int64 {
	if len(subjectIDs) == 0 {
		return map[string]int64{}
	}

	type countResult struct {
		SubjectID string
		Count     int64
	}
	var topicCounts []countResult

	// Single query with IN clause — hits idx_topic_subject_order
	qo.db.WithContext(ctx).Table("Topic").
		Select("subject_id, count(*) as count").
		Where("subject_id IN ?", subjectIDs).
		Group("subject_id").
		Scan(&topicCounts)

	result := make(map[string]int64, len(subjectIDs))
	for _, c := range topicCounts {
		result[c.SubjectID] = c.Count
	}
	return result
}
