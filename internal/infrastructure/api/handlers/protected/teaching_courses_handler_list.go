package protected

import (
	"fmt"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TeachingListCourses returns all courses owned by the authenticated instructor.
func TeachingListCourses(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	page, limit := parsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	statusFilter := strings.TrimSpace(c.Query("status"))

	// Build base query filtered by instructor
	baseQuery := database.Model(&models.Subject{}).
		Where("instructor_id = ?", userID)

	if search != "" {
		pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
		baseQuery = baseQuery.Where(
			"LOWER(COALESCE(name, '')) LIKE ? OR LOWER(COALESCE(name_ar, '')) LIKE ?",
			pattern, pattern,
		)
	}

	if statusFilter != "" && statusFilter != "all" {
		upperStatus := strings.ToUpper(statusFilter)
		baseQuery = baseQuery.Where("status = ?", upperStatus)
	}

	var total int64
	baseQuery.Session(&gorm.Session{}).Count(&total)

	offset := (page - 1) * limit
	var subjects []models.Subject
	if err := baseQuery.
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Preload("Topics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC").Preload("SubTopics", func(db *gorm.DB) *gorm.DB {
				return db.Order("\"order\" ASC")
			})
		}).
		Find(&subjects).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	// Transform to frontend format
	type LessonItem struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Duration  string `json:"duration"`
		Type      string `json:"type"`
		IsPreview bool   `json:"isPreview"`
	}
	type ChapterItem struct {
		ID      string       `json:"id"`
		Title   string       `json:"title"`
		Lessons []LessonItem `json:"lessons"`
	}
	type CourseItem struct {
		ID            string        `json:"id"`
		Title         string        `json:"title"`
		Description   string        `json:"description"`
		Thumbnail     string        `json:"thumbnail"`
		Status        string        `json:"status"`
		StudentsCount int           `json:"studentsCount"`
		LessonsCount  int           `json:"lessonsCount"`
		Rating        float64       `json:"rating"`
		Price         float64       `json:"price"`
		Duration      string        `json:"duration"`
		Category      string        `json:"category"`
		CreatedDate   string        `json:"createdDate"`
		Chapters      []ChapterItem `json:"chapters"`
	}

	courses := make([]CourseItem, 0, len(subjects))
	for _, s := range subjects {
		price, _ := s.Price.Float64()
		rating, _ := s.Rating.Float64()

		// Count lessons
		lessonsCount := 0
		var chapters []ChapterItem
		for _, t := range s.Topics {
			ch := ChapterItem{
				ID:      t.ID,
				Title:   t.Title,
				Lessons: make([]LessonItem, 0, len(t.SubTopics)),
			}
			for _, st := range t.SubTopics {
				lessonsCount++
				ch.Lessons = append(ch.Lessons, LessonItem{
					ID:        st.ID,
					Title:     st.Title,
					Duration:  fmt.Sprintf("%d دقيقة", st.DurationMinutes),
					Type:      strings.ToLower(string(st.Type)),
					IsPreview: st.IsFree,
				})
			}
			chapters = append(chapters, ch)
		}

		// Map status to frontend format
		status := "draft"
		switch s.Status {
		case models.CourseStatusPublished:
			status = "published"
		case models.CourseStatusArchived:
			status = "archived"
		case models.CourseStatusDraft:
			status = "draft"
		}

		courses = append(courses, CourseItem{
			ID:            s.ID,
			Title:         s.Name,
			Description:   stringPtrToString(s.Description),
			Thumbnail:     stringPtrToString(s.ThumbnailUrl),
			Status:        status,
			StudentsCount: s.EnrolledCount,
			LessonsCount:  lessonsCount,
			Rating:        rating,
			Price:         price,
			Duration:      fmt.Sprintf("%d ساعة", s.DurationHours),
			Category:      "", // CategoryId not expanded
			CreatedDate:   s.CreatedAt.Format("2006-01-02"),
			Chapters:      chapters,
		})
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	api_response.Success(c, gin.H{
		"courses": courses,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}
