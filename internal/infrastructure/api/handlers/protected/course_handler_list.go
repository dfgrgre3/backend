package protected

import (
	"net/http"
	"strconv"
	"strings"
	courseservice "thanawy-backend/internal/domain/course/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// ListCourses lists courses with filters
func (h *CourseRESTHandler) ListCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offsetVal, err := strconv.Atoi(offsetStr); err == nil && limit > 0 {
			page = (offsetVal / limit) + 1
		}
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	if h.listCoursesHandler == nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list courses: listCoursesHandler not initialized")
		return
	}

	var status *string
	if statusStr := c.Query("status"); statusStr != "" {
		status = &statusStr
	}

	var level *string
	if levelStr := c.Query("level"); levelStr != "" {
		level = &levelStr
	}

	query := courseservice.ListCoursesQuery{
		Status:       status,
		Level:        level,
		Language:     optionalStringPtr(c.Query("language")),
		CategoryID:   optionalStringPtr(c.Query("categoryId")),
		InstructorID: optionalStringPtr(c.Query("instructorId")),
		IsFeatured:   boolPtr(c.Query("isFeatured")),
		IsTrending:   boolPtr(c.Query("isTrending")),
		IsNew:        boolPtr(c.Query("isNew")),
		SearchQuery:  optionalStringPtr(c.Query("search")),
		Page:         page,
		Limit:        limit,
	}

	courses, total, err := h.listCoursesHandler.Handle(c.Request.Context(), query)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to list courses", err)
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

// SearchCourses performs an advanced search for courses
func (h *CourseRESTHandler) SearchCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	if h.searchCoursesHandler == nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to search courses: searchCoursesHandler not initialized")
		return
	}

	var categoryIDs []string
	if catStr := c.Query("categoryIds"); catStr != "" {
		categoryIDs = strings.Split(catStr, ",")
	}

	var instructorIDs []string
	if instStr := c.Query("instructorIds"); instStr != "" {
		instructorIDs = strings.Split(instStr, ",")
	}

	var tags []string
	if tagStr := c.Query("tags"); tagStr != "" {
		tags = strings.Split(tagStr, ",")
	}

	var minPrice, maxPrice *float64
	if minStr := c.Query("minPrice"); minStr != "" {
		if val, err := strconv.ParseFloat(minStr, 64); err == nil {
			minPrice = &val
		}
	}
	if maxStr := c.Query("maxPrice"); maxStr != "" {
		if val, err := strconv.ParseFloat(maxStr, 64); err == nil {
			maxPrice = &val
		}
	}

	query := courseservice.SearchCoursesQuery{
		ListCoursesQuery: courseservice.ListCoursesQuery{
			Status:          optionalStringPtr(c.Query("status")),
			Level:           optionalStringPtr(c.Query("level")),
			Language:        optionalStringPtr(c.Query("language")),
			InstructorID:    optionalStringPtr(c.Query("instructorId")),
			IsFeatured:      boolPtr(c.Query("isFeatured")),
			IsTrending:      boolPtr(c.Query("isTrending")),
			IsNew:           boolPtr(c.Query("isNew")),
			HasCertificate:  boolPtr(c.Query("hasCertificate")),
			PublishedAfter:  optionalStringPtr(c.Query("publishedAfter")),
			PublishedBefore: optionalStringPtr(c.Query("publishedBefore")),
			Tags:            tags,
			PriceType:       optionalStringPtr(c.Query("priceType")),
			MinPrice:        minPrice,
			MaxPrice:        maxPrice,
			SortBy:          c.DefaultQuery("sortBy", "created_at"),
			SortOrder:       c.DefaultQuery("sortOrder", "desc"),
		},
		Query:           c.Query("q"),
		CategoryIDs:     categoryIDs,
		InstructorIDs:   instructorIDs,
		ExcludeCourseID: optionalStringPtr(c.Query("excludeCourseId")),
		Page:            page,
		Limit:           limit,
	}

	courses, total, err := h.searchCoursesHandler.Handle(c.Request.Context(), query)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to search courses", err)
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
