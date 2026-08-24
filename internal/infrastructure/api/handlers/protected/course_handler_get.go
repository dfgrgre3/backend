package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	courseservice "thanawy-backend/internal/domain/course/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// GetCourseMeta returns metadata for the course admin panel (counts, filter options).
func (h *CourseRESTHandler) GetCourseMeta(c *gin.Context) {
	var (
		totalCount     int64
		draftCount     int64
		reviewCount    int64
		publishedCount int64
		archivedCount  int64
	)

	db := h.db.Model(&models.LmsCourse{})
	db.Count(&totalCount)
	db.Where("status = ?", "draft").Count(&draftCount)
	db.Where("status = ?", "pending_review").Count(&reviewCount)
	db.Where("status = ?", "published").Count(&publishedCount)
	db.Where("status = ?", "archived").Count(&archivedCount)

	api_response.Success(c, gin.H{
		"total":          totalCount,
		"draft":          draftCount,
		"pending_review": reviewCount,
		"published":      publishedCount,
		"archived":       archivedCount,
		"levels":         []string{"BEGINNER", "INTERMEDIATE", "ADVANCED"},
		"statuses":       []string{"draft", "pending_review", "published", "archived"},
	})
}

// GetCourse retrieves a course by ID or slug
func (h *CourseRESTHandler) GetCourse(c *gin.Context) {
	id := c.Param("id")
	slug := c.Query("slug")

	query := courseservice.GetCourseQuery{
		ID:   id,
		Slug: slug,
	}

	courseEntity, err := h.getCourseHandler.Handle(c.Request.Context(), query)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	api_response.Success(c, gin.H{"course": courseEntity})
}

// CheckSlug checks if a slug is already in use
func (h *CourseRESTHandler) CheckSlug(c *gin.Context) {
	slug := c.Query("slug")
	courseID := c.Query("courseId")
	if slug == "" {
		api_response.Error(c, http.StatusBadRequest, "Slug query parameter is required")
		return
	}

	courseEntity, err := h.courseService.GetCourseBySlug(slug)
	if err != nil {
		api_response.Success(c, gin.H{"unique": true})
		return
	}

	if courseEntity != nil && courseID != "" && courseEntity.ID.String() == courseID {
		api_response.Success(c, gin.H{"unique": true})
		return
	}

	api_response.Success(c, gin.H{"unique": false})
}

// GetCoursesPendingReview returns courses pending review
func (h *CourseRESTHandler) GetCoursesPendingReview(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	query := courseservice.ListCoursesQuery{
		Status: optionalStringPtr("pending_review"),
		Page:   page,
		Limit:  limit,
	}

	courses, total, err := h.listCoursesHandler.Handle(c.Request.Context(), query)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch courses: "+err.Error())
		return
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (int(total) + limit - 1) / limit
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
