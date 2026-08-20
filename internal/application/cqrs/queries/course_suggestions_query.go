package queries

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
)

// CourseSuggestionReadModel represents a course proposed to a learner, including the reason it was matched.
type CourseSuggestionReadModel struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	Subject       string  `json:"subject"`
	Rating        float64 `json:"rating"`
	StudentsCount int     `json:"studentsCount"`
	Duration      string  `json:"duration"`
	Level         string  `json:"level"`
	Image         string  `json:"image"`
	MatchReason   string  `json:"matchReason"`
	MatchScore    int     `json:"matchScore"`
}

// SearchHistoryReadModel represents a past search performed by the learner.
type SearchHistoryReadModel struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
}

// CourseSuggestionsPage represents one page of course suggestions along with the learner's search history.
type CourseSuggestionsPage struct {
	Recommendations []CourseSuggestionReadModel `json:"recommendations"`
	SearchHistory   []SearchHistoryReadModel    `json:"searchHistory"`
	Page            int                         `json:"page"`
	TotalPages      int                         `json:"totalPages"`
	Total           int64                       `json:"total"`
}

// CourseSuggestionsQueryService recommends courses the learner is not enrolled in,
// ranked by how well they match the learner's category history.
type CourseSuggestionsQueryService struct{}

// NewCourseSuggestionsQueryService creates a new instance of CourseSuggestionsQueryService.
func NewCourseSuggestionsQueryService() *CourseSuggestionsQueryService {
	return &CourseSuggestionsQueryService{}
}

// readDB returns the read-only database connection.
func (s *CourseSuggestionsQueryService) readDB() *gorm.DB {
	return db.ReadDB()
}

// courseSuggestionRow represents the raw database row structure for the suggestions query.
// Defined at the package level to avoid re-allocation on every function call.
type courseSuggestionRow struct {
	ID            string  `gorm:"column:id"`
	Title         string  `gorm:"column:title"`
	Description   *string `gorm:"column:description"`
	CategoryName  *string `gorm:"column:category_name"`
	CategoryID    *string `gorm:"column:category_id"`
	Rating        float64 `gorm:"column:rating"`
	EnrolledCount int     `gorm:"column:enrolled_count"`
	DurationHours int     `gorm:"column:duration_hours"`
	Level         string  `gorm:"column:level"`
	ThumbnailURL  *string `gorm:"column:thumbnail_url"`
}

// GetSuggestions returns published courses the learner has not enrolled in,
// prioritizing categories the learner already studies.
func (s *CourseSuggestionsQueryService) GetSuggestions(userID string, page, limit int) (*CourseSuggestionsPage, error) {
	rdb := s.readDB()
	if rdb == nil {
		return &CourseSuggestionsPage{
			Recommendations: []CourseSuggestionReadModel{},
			SearchHistory:   []SearchHistoryReadModel{},
			Page:            page,
			TotalPages:      0,
			Total:           0,
		}, nil
	}

	preferredCategories, err := s.preferredCategories(rdb, userID)
	if err != nil {
		return nil, err
	}

	// Base query to filter published, active subjects the user is not enrolled in.
	// Note: Session(&gorm.Session{}) gives this chain an isolated statement so that
	// GORM does not leak leftover JOIN / WHERE clauses from other query chains that
	// are built on the same root *gorm.DB (rdb). Without it, reusing rdb across
	// multiple chains produces SQL like 'FROM "Subject" s JOIN "Subject" s ...'
	// (SQLSTATE 42712: table name "s" specified more than once).
	base := rdb.Session(&gorm.Session{}).Table(`"Subject" s`).
		Where(`s.deleted_at IS NULL AND s.is_published = true AND s.is_active = true`).
		Where(`NOT EXISTS (
			SELECT 1 FROM "SubjectEnrollment" se
			WHERE se.subject_id = s.id AND se.user_id = ? AND se.deleted_at IS NULL
		)`, userID)

	var total int64
	// Use a new session for counting to avoid mutating the base query's SELECT clause.
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []courseSuggestionRow

	// Formatted SQL select for better readability.
	query := base.Session(&gorm.Session{}).
		Joins(`LEFT JOIN "Category" c ON c.id = s.category_id AND c.deleted_at IS NULL`).
		Select(`
			s.id AS id,
			COALESCE(NULLIF(s.name_ar, ''), s.name) AS title,
			s.description AS description,
			c.name AS category_name,
			s.category_id AS category_id,
			s.rating AS rating,
			s.enrolled_count AS enrolled_count,
			s.duration_hours AS duration_hours,
			s.level AS level,
			s.thumbnail_url AS thumbnail_url
		`)

	// Courses in categories the learner already studies rank first, followed by secondary sorting.
	if len(preferredCategories) > 0 {
		query = query.Order(clauseCategoryPriority(preferredCategories)).Order("s.rating DESC").Order("s.enrolled_count DESC")
	} else {
		query = query.Order("s.rating DESC, s.enrolled_count DESC")
	}

	if err := query.
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	// Map preferred categories for O(1) lookups during suggestion building.
	preferred := make(map[string]bool, len(preferredCategories))
	for _, id := range preferredCategories {
		preferred[id] = true
	}

	// Pre-allocate slice capacity for better memory management.
	suggestions := make([]CourseSuggestionReadModel, 0, len(rows))
	for _, r := range rows {
		suggestions = append(suggestions, buildSuggestion(
			r.ID, r.Title, r.Description, r.CategoryName, r.CategoryID, r.ThumbnailURL,
			r.Level, r.Rating, r.EnrolledCount, r.DurationHours, preferred,
		))
	}

	history, err := s.searchHistory(rdb, userID)
	if err != nil {
		return nil, err
	}

	// Calculate total pages using ceiling division.
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if total == 0 {
		totalPages = 0
	}

	return &CourseSuggestionsPage{
		Recommendations: suggestions,
		SearchHistory:   history,
		Page:            page,
		TotalPages:      totalPages,
		Total:           total,
	}, nil
}

// preferredCategories lists the distinct categories of courses the learner is currently enrolled in.
func (s *CourseSuggestionsQueryService) preferredCategories(rdb *gorm.DB, userID string) ([]string, error) {
	var categories []string
	err := rdb.Session(&gorm.Session{}).Table(`"SubjectEnrollment" se`).
		Joins(`JOIN "Subject" s ON s.id = se.subject_id`).
		Where("se.user_id = ? AND se.deleted_at IS NULL AND s.category_id IS NOT NULL", userID).
		Distinct("s.category_id").
		Pluck("s.category_id", &categories).Error
	return categories, err
}

// searchHistory returns the learner's most recent distinct searches.
func (s *CourseSuggestionsQueryService) searchHistory(rdb *gorm.DB, userID string) ([]SearchHistoryReadModel, error) {
	// Query AnalyticsEvent table where search events are tracked
	var events []models.AnalyticsEvent
	if err := rdb.Session(&gorm.Session{}).
		Where("event_type = ? AND user_id = ?", "search_query", userID).
		Order("received_at DESC").
		Limit(10).
		Find(&events).Error; err == nil && len(events) > 0 {
		history := make([]SearchHistoryReadModel, 0, len(events))
		for _, evt := range events {
			if q, ok := evt.Payload["query"].(string); ok && q != "" {
				history = append(history, SearchHistoryReadModel{
					Query:     q,
					Timestamp: evt.ReceivedAt,
				})
			}
		}
		return history, nil
	}

	// Fallback to legacy search_history table if it exists
	if rdb.Migrator().HasTable("search_history") {
		var records []models.SearchHistory
		if err := rdb.Session(&gorm.Session{}).
			Where("user_id = ?", userID).
			Order("created_at DESC").
			Limit(10).
			Find(&records).Error; err == nil && len(records) > 0 {
			history := make([]SearchHistoryReadModel, 0, len(records))
			for _, record := range records {
				history = append(history, SearchHistoryReadModel{
					Query:     record.Query,
					Timestamp: record.CreatedAt,
				})
			}
			return history, nil
		}
	}

	return []SearchHistoryReadModel{}, nil
}

// clauseCategoryPriority generates a SQL ORDER BY clause that sorts courses
// in the learner's known categories first (0) before others (1).
func clauseCategoryPriority(categories []string) clause.OrderByColumn {
	quoted := make([]string, 0, len(categories))
	for _, c := range categories {
		quoted = append(quoted, fmt.Sprintf("'%s'", strings.ReplaceAll(c, "'", "''")))
	}
	rawSQL := fmt.Sprintf("CASE WHEN s.category_id IN (%s) THEN 0 ELSE 1 END", strings.Join(quoted, ","))
	return clause.OrderByColumn{Column: clause.Column{Name: rawSQL, Raw: true}}
}

// buildSuggestion assembles one course suggestion card, deriving the match score
// and reason from the actual data present on the course.
func buildSuggestion(
	id, title string,
	description, categoryName, categoryID, thumbnail *string,
	level string,
	rating float64,
	enrolledCount, durationHours int,
	preferred map[string]bool,
) CourseSuggestionReadModel {

	isPreferred := categoryID != nil && preferred[*categoryID]

	return CourseSuggestionReadModel{
		ID:            id,
		Title:         title,
		Description:   derefOr(description, ""),
		Category:      derefOr(categoryName, "عام"),
		Subject:       title,
		Rating:        rating,
		StudentsCount: enrolledCount,
		Duration:      formatDuration(durationHours),
		Level:         localizeLevel(level),
		Image:         derefOr(thumbnail, ""),
		MatchReason:   matchReason(isPreferred, rating, enrolledCount),
		MatchScore:    matchScore(isPreferred, rating, enrolledCount),
	}
}

// matchScore calculates a weighted score based on category affinity, rating, and popularity.
func matchScore(isPreferred bool, rating float64, enrolledCount int) int {
	score := 40.0
	if isPreferred {
		score += 30
	}
	score += (rating / 5.0) * 20

	switch {
	case enrolledCount >= 500:
		score += 10
	case enrolledCount >= 100:
		score += 6
	case enrolledCount >= 10:
		score += 3
	}

	if score > 100 {
		score = 100
	}

	// Round to nearest integer
	return int(score + 0.5)
}

// matchReason generates a localized explanation for why a course was suggested.
func matchReason(isPreferred bool, rating float64, enrolledCount int) string {
	if isPreferred {
		return "من نفس مجال الكورسات المسجل بها"
	}
	if rating >= 4.5 {
		return fmt.Sprintf("تقييم مرتفع %.1f من 5", rating)
	}
	if enrolledCount >= 100 {
		return fmt.Sprintf("مقبل عليه %d طالب", enrolledCount)
	}
	return "كورس منشور متاح للتسجيل"
}

// formatDuration converts hours into a readable Arabic string.
func formatDuration(hours int) string {
	if hours <= 0 {
		return "غير محدد"
	}
	return fmt.Sprintf("%d ساعة", hours)
}

// localizeLevel maps internal level constants to Arabic display strings.
func localizeLevel(level string) string {
	switch models.Level(level) {
	case models.LevelBeginner:
		return "مبتدئ"
	case models.LevelAdvanced:
		return "متقدم"
	default:
		return "متوسط"
	}
}

// derefOr safely dereferences a string pointer, returning a fallback value if nil or empty.
func derefOr(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}
