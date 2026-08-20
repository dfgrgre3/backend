package protected

import (
	"context"
	"log"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func sanitizeSearchTerm(term string) string {
	// Remove potential SQL injection patterns
	term = strings.ReplaceAll(term, "--", "")
	term = strings.ReplaceAll(term, "/*", "")
	term = strings.ReplaceAll(term, "*/", "")
	term = strings.ReplaceAll(term, ";", "")
	term = strings.TrimSpace(term)

	// Limit length to prevent abuse
	if len(term) > 100 {
		term = term[:100]
	}

	return term
}

func isValidLevel(level string) bool {
	switch level {
	case "BEGINNER", "INTERMEDIATE", "ADVANCED":
		return true
	default:
		return false
	}
}

func isValidBoolean(value string) bool {
	return value == "true" || value == "false"
}

func isValidCourseStatus(status string) bool {
	switch status {
	case "DRAFT", "UNDER_REVIEW", "PUBLISHED", "ARCHIVED", "REJECTED":
		return true
	default:
		return false
	}
}

// fetchTopicCounts retrieves topic counts for multiple subjects in a single query
func fetchTopicCounts(ctx context.Context, subjectIDs []string) map[string]int64 {
	if len(subjectIDs) == 0 {
		return map[string]int64{}
	}

	type countResult struct {
		SubjectID string
		Count     int64
	}
	var topicCounts []countResult
	db.ReadDB(ctx).Table("Topic").
		Select("subject_id, count(*) as count").
		Where("subject_id IN ?", subjectIDs).
		Group("subject_id").
		Scan(&topicCounts)

	topicCountMap := make(map[string]int64)
	for _, c := range topicCounts {
		topicCountMap[c.SubjectID] = c.Count
	}
	return topicCountMap
}

// buildSubjectListResponse creates a standardized response for subject lists
func buildSubjectListResponse(subjects []models.Subject, topicCountMap map[string]int64) []gin.H {
	items := make([]gin.H, 0, len(subjects))
	for _, subject := range subjects {
		items = append(items, gin.H{
			"id":                     subject.ID,
			"name":                   subject.Name,
			"nameAr":                 subject.NameAr,
			"code":                   subject.Code,
			"description":            subject.Description,
			"icon":                   subject.Icon,
			"color":                  subject.Color,
			"type":                   "COURSE",
			"isActive":               subject.IsActive,
			"isPublished":            subject.IsPublished,
			"price":                  subject.Price,
			"level":                  subject.Level,
			"instructorName":         subject.InstructorName,
			"instructorId":           subject.InstructorId,
			"categoryId":             subject.CategoryId,
			"thumbnailUrl":           subject.ThumbnailUrl,
			"trailerUrl":             subject.TrailerUrl,
			"trailerDurationMinutes": subject.TrailerDurationMinutes,
			"durationHours":          subject.DurationHours,
			"requirements":           subject.Requirements,
			"learningObjectives":     subject.LearningObjectives,
			"seoTitle":               subject.SeoTitle,
			"seoDescription":         subject.SeoDescription,
			"slug":                   subject.Slug,
			"rating":                 subject.Rating,
			"enrolledCount":          subject.EnrolledCount,
			"createdAt":              subject.CreatedAt,
			"updatedAt":              subject.UpdatedAt,
			"_count": gin.H{
				"enrollments": subject.EnrolledCount,
				"topics":      topicCountMap[subject.ID],
				"reviews":     0,
				"teachers":    0,
			},
		})
	}
	return items
}

// buildSubjectFilters applies common filter conditions to a query and returns it.
func buildSubjectFilters(query *gorm.DB, c *gin.Context) *gorm.DB {
	if catID := c.Query("categoryId"); catID != "" {
		if _, err := uuid.Parse(catID); err == nil {
			query = query.Where("category_id = ?", catID)
		}
	}

	search := c.Query("search")
	if search != "" {
		search = sanitizeSearchTerm(search)
		if search != "" {
			query = query.Where("name ILIKE ? OR name_ar ILIKE ?", "%"+search+"%", "%"+search+"%")
		}
	}

	if level := c.Query("level"); level != "" {
		if isValidLevel(level) {
			query = query.Where("level = ?", level)
		}
	}
	if isPublished := c.Query("isPublished"); isPublished != "" {
		if isValidBoolean(isPublished) {
			query = query.Where("is_published = ?", isPublished == "true")
		}
	}
	if isActive := c.Query("isActive"); isActive != "" {
		if isValidBoolean(isActive) {
			query = query.Where(isActiveQuery, isActive == "true")
		}
	}
	if status := c.Query("status"); status != "" {
		if isValidCourseStatus(status) {
			query = query.Where("status = ?", status)
		}
	}
	if isFeatured := c.Query("isFeatured"); isValidBoolean(isFeatured) && isFeatured == "true" {
		query = query.Where("is_featured = ?", true)
	}
	if isTrending := c.Query("isTrending"); isValidBoolean(isTrending) && isTrending == "true" {
		query = query.Where("is_trending = ?", true)
	}
	if isNew := c.Query("isNew"); isValidBoolean(isNew) && isNew == "true" {
		query = query.Where("is_new = ?", true)
	}
	return query
}

// applyIDOrSlugQuery applies where clause based on whether id is a UUID or a slug
func applyIDOrSlugQuery(query *gorm.DB, id string) *gorm.DB {
	id = strings.TrimSpace(id)
	if id == "" || strings.EqualFold(id, "undefined") || strings.EqualFold(id, "null") {
		return query.Where("1 = 0")
	}
	if len(id) == 36 && strings.Contains(id, "-") {
		return query.Where(idQuery, id)
	}
	return query.Where("slug = ? OR id = ?", id, id)
}

func handleSubjectError(c *gin.Context, id string, err error, contextMsg string) {
	if err == gorm.ErrRecordNotFound {
		api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
	} else {
		log.Printf("Error %s %q: %v", contextMsg, id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to "+contextMsg)
	}
}
