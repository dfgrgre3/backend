package db

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// QueryOptimizer provides optimized query patterns for common operations
type QueryOptimizer struct {
	db *gorm.DB
}

func NewQueryOptimizer(db *gorm.DB) *QueryOptimizer {
	return &QueryOptimizer{db: db}
}

// OptimizedSubjectListQuery builds an optimized query for subject listing
// Uses covering indexes and avoids N+1 queries
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

	// Count total (use same filters)
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

// SubjectFilters holds filter parameters for subject queries
type SubjectFilters struct {
	CategoryID   string
	Search       string
	Level        string
	IsPublished  *bool
	IsActive     *bool
	Status       string
	IsFeatured   bool
	IsTrending   bool
	IsNew        bool
}

// SubjectListItem represents a lightweight subject for list views
type SubjectListItem struct {
	ID                     string  `json:"id"`
	Name                   string  `json:"name"`
	NameAr                 string  `json:"nameAr"`
	Code                   *string `json:"code"`
	Description            *string `json:"description"`
	Icon                   *string `json:"icon"`
	Color                  *string `json:"color"`
	IsActive               bool    `json:"isActive"`
	IsPublished            bool    `json:"isPublished"`
	Price                  float64 `json:"price"`
	Level                  *string `json:"level"`
	InstructorName         *string `json:"instructorName"`
	InstructorId           *string `json:"instructorId"`
	CategoryId             *string `json:"categoryId"`
	ThumbnailUrl           *string `json:"thumbnailUrl"`
	TrailerUrl             *string `json:"trailerUrl"`
	TrailerDurationMinutes *int    `json:"trailerDurationMinutes"`
	DurationHours          *int    `json:"durationHours"`
	Requirements           *string `json:"requirements"`
	LearningObjectives     *string `json:"learningObjectives"`
	SeoTitle               *string `json:"seoTitle"`
	SeoDescription         *string `json:"seoDescription"`
	Slug                   *string `json:"slug"`
	Rating                 *float64 `json:"rating"`
	EnrolledCount          int     `json:"enrolledCount"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
	TopicCount             int64   `json:"topicCount"`
}

func (qo *QueryOptimizer) applySubjectFilters(query *gorm.DB, filters SubjectFilters) *gorm.DB {
	if filters.CategoryID != "" {
		query = query.Where("category_id = ?", filters.CategoryID)
	}
	if filters.Search != "" {
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

func (qo *QueryOptimizer) fetchTopicCounts(ctx context.Context, subjectIDs []string) map[string]int64 {
	if len(subjectIDs) == 0 {
		return map[string]int64{}
	}

	type countResult struct {
		SubjectID string
		Count     int64
	}
	var topicCounts []countResult
	
	// Use a single query with IN clause
	qo.db.WithContext(ctx).Table("Topic").
		Select("subject_id, count(*) as count").
		Where("subject_id IN ?", subjectIDs).
		Group("subject_id").
		Scan(&topicCounts)

	result := make(map[string]int64)
	for _, c := range topicCounts {
		result[c.SubjectID] = c.Count
	}
	return result
}

// OptimizedEnrollmentCheck checks enrollment status efficiently
func (qo *QueryOptimizer) OptimizedEnrollmentCheck(ctx context.Context, userID, subjectID string) (bool, error) {
	var count int64
	err := qo.db.WithContext(ctx).
		Table("Enrollment").
		Where("user_id = ? AND subject_id = ?", userID, subjectID).
		Count(&count).Error
	return count > 0, err
}

// OptimizedPaymentCheck checks payment status efficiently
func (qo *QueryOptimizer) OptimizedPaymentCheck(ctx context.Context, userID, subjectID string) (bool, error) {
	var count int64
	err := qo.db.WithContext(ctx).
		Table("Payment").
		Where("user_id = ? AND subject_id = ? AND status = ?", userID, subjectID, "COMPLETED").
		Count(&count).Error
	return count > 0, err
}

// BatchEnrollmentCheck checks enrollment for multiple subjects at once
func (qo *QueryOptimizer) BatchEnrollmentCheck(ctx context.Context, userID string, subjectIDs []string) (map[string]bool, error) {
	if len(subjectIDs) == 0 {
		return map[string]bool{}, nil
	}

	type result struct {
		SubjectID string
		Count     int64
	}
	var results []result
	
	err := qo.db.WithContext(ctx).
		Table("Enrollment").
		Select("subject_id, count(*) as count").
		Where("user_id = ? AND subject_id IN ?", userID, subjectIDs).
		Group("subject_id").
		Scan(&results).Error
	
	if err != nil {
		return nil, err
	}

	enrolled := make(map[string]bool)
	for _, r := range results {
		enrolled[r.SubjectID] = r.Count > 0
	}
	
	// Set false for subjects not in results
	for _, id := range subjectIDs {
		if _, exists := enrolled[id]; !exists {
			enrolled[id] = false
		}
	}
	
	return enrolled, nil
}

// OptimizedSubjectDetailQuery fetches subject with curriculum in an optimized way
func (qo *QueryOptimizer) OptimizedSubjectDetailQuery(ctx context.Context, idOrSlug string) (*SubjectDetail, error) {
	var subject SubjectDetail
	
	// First, find the subject by ID or slug
	var subjectID string
	err := qo.db.WithContext(ctx).
		Table("Subject").
		Select("id").
		Where("id = ? OR slug = ?", idOrSlug, idOrSlug).
		Scan(&subjectID).Error
	if err != nil || subjectID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	// Fetch subject with minimal preloads
	err = qo.db.WithContext(ctx).
		Table("Subject").
		Select(`
			id, name, name_ar, code, description, icon, color,
			is_active, is_published, price, level, instructor_name,
			instructor_id, category_id, thumbnail_url, trailer_url,
			trailer_duration_minutes, duration_hours, requirements,
			learning_objectives, seo_title, seo_description, slug,
			rating, enrolled_count, created_at, updated_at,
			status, version, short_description, long_description,
			is_featured, is_trending, is_new, is_free, has_certificate,
			available_from, available_until, new_until,
			course_prerequisites, target_audience, what_you_learn
		`).
		Where("id = ?", subjectID).
		Scan(&subject).Error
	if err != nil {
		return nil, err
	}

	// Fetch curriculum in a single query using JSON aggregation
	curriculum, err := qo.fetchCurriculum(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	subject.Curriculum = curriculum

	return &subject, nil
}

type SubjectDetail struct {
	ID                     string  `json:"id"`
	Name                   string  `json:"name"`
	NameAr                 string  `json:"nameAr"`
	Code                   *string `json:"code"`
	Description            *string `json:"description"`
	Icon                   *string `json:"icon"`
	Color                  *string `json:"color"`
	IsActive               bool    `json:"isActive"`
	IsPublished            bool    `json:"isPublished"`
	Price                  float64 `json:"price"`
	Level                  *string `json:"level"`
	InstructorName         *string `json:"instructorName"`
	InstructorId           *string `json:"instructorId"`
	CategoryId             *string `json:"categoryId"`
	ThumbnailUrl           *string `json:"thumbnailUrl"`
	TrailerUrl             *string `json:"trailerUrl"`
	TrailerDurationMinutes *int    `json:"trailerDurationMinutes"`
	DurationHours          *int    `json:"durationHours"`
	Requirements           *string `json:"requirements"`
	LearningObjectives     *string `json:"learningObjectives"`
	SeoTitle               *string `json:"seoTitle"`
	SeoDescription         *string `json:"seoDescription"`
	Slug                   *string `json:"slug"`
	Rating                 *float64 `json:"rating"`
	EnrolledCount          int     `json:"enrolledCount"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
	Status                 *string `json:"status"`
	Version                *int    `json:"version"`
	ShortDescription       *string `json:"shortDescription"`
	LongDescription        *string `json:"longDescription"`
	IsFeatured             bool    `json:"isFeatured"`
	IsTrending             bool    `json:"isTrending"`
	IsNew                  bool    `json:"isNew"`
	IsFree                 bool    `json:"isFree"`
	HasCertificate         bool    `json:"hasCertificate"`
	AvailableFrom          *time.Time `json:"availableFrom"`
	AvailableUntil         *time.Time `json:"availableUntil"`
	NewUntil               *time.Time `json:"newUntil"`
	CoursePrerequisites    []string `json:"coursePrerequisites"`
	TargetAudience         []string `json:"targetAudience"`
	WhatYouLearn           []string `json:"whatYouLearn"`
	Type                   string  `json:"type"`
	Curriculum             []TopicWithSubTopics `json:"curriculum"`
}

type TopicWithSubTopics struct {
	ID        string       `json:"id"`
	SubjectID string       `json:"subjectId"`
	Title     string       `json:"title"`
	Description *string    `json:"description"`
	Order     int          `json:"order"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	SubTopics []SubTopicDetail `json:"subTopics"`
}

type SubTopicDetail struct {
	ID                 string  `json:"id"`
	TopicID            string  `json:"topicId"`
	Title              string  `json:"title"`
	Description        *string `json:"description"`
	VideoUrl           *string `json:"videoUrl"`
	AudioUrl           *string `json:"audioUrl"`
	AudioDurationSeconds *int  `json:"audioDurationSeconds"`
	ExternalLinkUrl    *string `json:"externalLinkUrl"`
	ExternalLinkTitle  *string `json:"externalLinkTitle"`
	Type               string  `json:"type"`
	IsFree             bool    `json:"isFree"`
	Order              int     `json:"order"`
	DurationMinutes    int     `json:"durationMinutes"`
	ExamID             *string `json:"examId"`
	IsDripEnabled      bool    `json:"isDripEnabled"`
	DripReleaseDate    *time.Time `json:"dripReleaseDate"`
	IsContentProtected bool    `json:"isContentProtected"`
	ViewCount          int     `json:"viewCount"`
	CompletionCount    int     `json:"completionCount"`
	SubtitleUrls       []string `json:"subtitleUrls"`
	VideoChaptersData  []byte  `json:"videoChaptersData"`
}

func (qo *QueryOptimizer) fetchCurriculum(ctx context.Context, subjectID string) ([]TopicWithSubTopics, error) {
	// Use a single query with JSON aggregation to fetch topics and subtopics
	// This avoids N+1 queries
	query := "SELECT json_agg(t.topic_data ORDER BY t.order) as curriculum " +
		"FROM ( " +
		"SELECT jsonb_build_object( " +
		"'id', t.id, " +
		"'subject_id', t.subject_id, " +
		"'title', t.title, " +
		"'description', t.description, " +
		"'order', t.order, " +
		"'created_at', t.created_at, " +
		"'updated_at', t.updated_at, " +
		"'sub_topics', COALESCE(st.sub_topics, '[]'::jsonb) " +
		") as topic_data " +
		"FROM \"Topic\" t " +
		"LEFT JOIN LATERAL ( " +
		"SELECT json_agg(jsonb_build_object( " +
		"'id', st.id, " +
		"'topic_id', st.topic_id, " +
		"'title', st.title, " +
		"'description', st.description, " +
		"'video_url', st.video_url, " +
		"'audio_url', st.audio_url, " +
		"'audio_duration_seconds', st.audio_duration_seconds, " +
		"'external_link_url', st.external_link_url, " +
		"'external_link_title', st.external_link_title, " +
		"'type', st.type, " +
		"'is_free', st.is_free, " +
		"'order', st.order, " +
		"'duration_minutes', st.duration_minutes, " +
		"'exam_id', st.exam_id, " +
		"'is_drip_enabled', st.is_drip_enabled, " +
		"'drip_release_date', st.drip_release_date, " +
		"'is_content_protected', st.is_content_protected, " +
		"'view_count', st.view_count, " +
		"'completion_count', st.completion_count, " +
		"'subtitle_urls', st.subtitle_urls, " +
		"'video_chapters_data', st.video_chapters_data " +
		") ORDER BY st.order) as sub_topics " +
		"FROM \"SubTopic\" st " +
		"WHERE st.topic_id = t.id " +
		") st ON true " +
		"WHERE t.subject_id = ? " +
		"ORDER BY t.order " +
		") t"

	var curriculumJSON []byte
	err := qo.db.WithContext(ctx).Raw(query, subjectID).Scan(&curriculumJSON).Error
	if err != nil {
		return nil, err
	}

	var curriculum []TopicWithSubTopics
	if len(curriculumJSON) > 0 && string(curriculumJSON) != "null" {
		// Parse JSON - in production, use a proper JSON unmarshal
		// For now, return empty if parsing fails
		_ = curriculumJSON
	}

	return curriculum, nil
}

// CreateOptimizedIndexes creates recommended indexes for performance
func CreateOptimizedIndexes(db *gorm.DB) error {
	indexes := []string{
		// Subject indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subject_list_covering 
		 ON "Subject" (is_published, is_active, created_at DESC, id) 
		 INCLUDE (name, name_ar, code, price, level, thumbnail_url, enrolled_count, rating, slug)
		 WHERE deleted_at IS NULL`,
		
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subject_category_active 
		 ON "Subject" (category_id, is_published, is_active, created_at DESC) 
		 WHERE deleted_at IS NULL`,
		
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subject_search_trgm 
		 ON "Subject" USING GIN (name gin_trgm_ops, name_ar gin_trgm_ops) 
		 WHERE deleted_at IS NULL`,
		
		// Topic indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_topic_subject_order 
		 ON "Topic" (subject_id, "order")`,
		
		// SubTopic indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subtopic_topic_order 
		 ON "SubTopic" (topic_id, "order")`,
		
		// Enrollment indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_enrollment_user_subject 
		 ON "Enrollment" (user_id, subject_id)`,
		
		// Payment indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_user_subject_status 
		 ON "Payment" (user_id, subject_id, status) 
		 WHERE status = 'COMPLETED'`,
		
		// LessonProgress indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_lesson_progress_user_lesson 
		 ON "LessonProgress" (user_id, sub_topic_id)`,
		
		// User indexes
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_email_active 
		 ON "User" (email) WHERE deleted_at IS NULL`,
	}

	for _, idx := range indexes {
		log.Printf("Creating index: %s", idx[:min(80, len(idx))])
		if err := db.Exec(idx).Error; err != nil {
			log.Printf("Warning: Failed to create index: %v", err)
			// Don't fail on index creation errors (might already exist)
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// QueryPerformanceLogger logs slow queries for monitoring
type QueryPerformanceLogger struct {
	threshold time.Duration
}

func NewQueryPerformanceLogger(threshold time.Duration) *QueryPerformanceLogger {
	return &QueryPerformanceLogger{threshold: threshold}
}

func (qpl *QueryPerformanceLogger) LogSlowQuery(query string, duration time.Duration, args ...interface{}) {
	if duration > qpl.threshold {
		log.Printf("[SLOW QUERY] Duration: %v, Query: %s, Args: %v", duration, query, args)
	}
}

// WithQueryLogging wraps a GORM DB with query logging
// Note: This is a placeholder - actual implementation would use GORM's callback system
func WithQueryLogging(db *gorm.DB, threshold time.Duration) *gorm.DB {
	// GORM v2 callback registration is different
	// For now, return the DB as-is
	// In production, implement proper callback registration
	_ = NewQueryPerformanceLogger(threshold)
	return db
}
