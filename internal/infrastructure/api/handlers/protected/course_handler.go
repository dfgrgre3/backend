package protected

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	models "thanawy-backend/internal/domain/common"
	courseservice "thanawy-backend/internal/domain/course/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CourseRESTHandler handles REST API requests for courses
type CourseRESTHandler struct {
	createCourseHandler   *courseservice.CreateCourseHandler
	updateCourseHandler   *courseservice.UpdateCourseHandler
	enrollUserHandler     *courseservice.EnrollUserHandler
	updateProgressHandler *courseservice.UpdateProgressHandler
	getCourseHandler      *courseservice.GetCourseHandler
	listCoursesHandler    *courseservice.ListCoursesHandler
	searchCoursesHandler  *courseservice.SearchCoursesHandler
	getEnrollmentHandler  *courseservice.GetEnrollmentHandler
	courseService         *courseservice.LmsService
	db                    *gorm.DB
}

// NewCourseRESTHandler creates a new REST course handler
func NewCourseRESTHandler(
	courseService *courseservice.LmsService,
	createCourseHandler *courseservice.CreateCourseHandler,
	updateCourseHandler *courseservice.UpdateCourseHandler,
	enrollUserHandler *courseservice.EnrollUserHandler,
	updateProgressHandler *courseservice.UpdateProgressHandler,
	getCourseHandler *courseservice.GetCourseHandler,
	listCoursesHandler *courseservice.ListCoursesHandler,
	searchCoursesHandler *courseservice.SearchCoursesHandler,
	getEnrollmentHandler *courseservice.GetEnrollmentHandler,
	db *gorm.DB,
) *CourseRESTHandler {
	return &CourseRESTHandler{
		courseService:         courseService,
		createCourseHandler:   createCourseHandler,
		updateCourseHandler:   updateCourseHandler,
		enrollUserHandler:     enrollUserHandler,
		updateProgressHandler: updateProgressHandler,
		getCourseHandler:      getCourseHandler,
		listCoursesHandler:    listCoursesHandler,
		searchCoursesHandler:  searchCoursesHandler,
		getEnrollmentHandler:  getEnrollmentHandler,
		db:                    db,
	}
}

// CreateCourse creates a new course
func (h *CourseRESTHandler) CreateCourse(c *gin.Context) {
	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if req.Title == "" {
		if req.NameAr != "" {
			req.Title = req.NameAr
		} else if req.Name != "" {
			req.Title = req.Name
		} else {
			req.Title = "كورس جديد"
		}
	}
	if req.PrimaryInstructorID == "" {
		if req.InstructorID != "" {
			req.PrimaryInstructorID = req.InstructorID
		} else if userIdVal, exists := c.Get("userId"); exists {
			if uid, ok := userIdVal.(string); ok && uid != "" {
				req.PrimaryInstructorID = uid
			}
		}
		if req.PrimaryInstructorID == "" {
			req.PrimaryInstructorID = "00000000-0000-0000-0000-000000000000"
		}
	}

	cmd := courseservice.CreateCourseCommand{
		Title:                 req.Title,
		Slug:                  req.Slug,
		ShortDescription:      req.ShortDescription,
		LongDescription:       req.LongDescription,
		CoverImageURL:         req.CoverImageURL,
		PromoVideoURL:         req.PromoVideoURL,
		Level:                 req.Level,
		Language:              req.Language,
		EstimatedDurationMins: req.EstimatedDurationMins,
		HasCertificate:        req.HasCertificate,
		CertificateTemplate:   req.CertificateTemplate,
		MaxStudents:           req.MaxStudents,
		SEOTitle:              req.SEOTitle,
		SEODescription:        req.SEODescription,
		SEOKeywords:           req.SEOKeywords,
		PrerequisitesText:     req.PrerequisitesText,
		TargetAudience:        req.TargetAudience,
		LearningOutcomes:      req.LearningOutcomes,
		PrimaryInstructorID:   req.PrimaryInstructorID,
		CategoryIDs:           req.CategoryIDs,
	}

	courseEntity, err := h.createCourseHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create course: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"course": courseEntity})
}

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

// GetReviews retrieves reviews for a course
func (h *CourseRESTHandler) GetReviews(c *gin.Context) {
	id := c.Param("id")

	reviews, err := h.courseService.ListReviews(uuid.MustParse(id), "")
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to get course reviews: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"reviews": reviews})
}

// UpdateCourse updates an existing course
func (h *CourseRESTHandler) UpdateCourse(c *gin.Context) {
	id := c.Param("id")

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if id == "" {
		var idBody struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(bodyBytes, &idBody); err == nil && idBody.ID != "" {
			id = idBody.ID
		}
	}

	var req UpdateCourseRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if req.Title == nil {
		if req.NameAr != nil && *req.NameAr != "" {
			req.Title = req.NameAr
		} else if req.Name != nil && *req.Name != "" {
			req.Title = req.Name
		}
	}
	if req.PrimaryInstructorID == nil && req.InstructorID != nil && *req.InstructorID != "" {
		req.PrimaryInstructorID = req.InstructorID
	}

	var courseID uuid.UUID
	if parsed, err := uuid.Parse(id); err == nil {
		courseID = parsed
	}

	cmd := courseservice.UpdateCourseCommand{
		ID:                    courseID,
		CourseID:              id,
		Title:                 req.Title,
		Slug:                  req.Slug,
		ShortDescription:      req.ShortDescription,
		LongDescription:       req.LongDescription,
		CoverImageURL:         req.CoverImageURL,
		PromoVideoURL:         req.PromoVideoURL,
		Level:                 req.Level,
		Language:              req.Language,
		EstimatedDurationMins: req.EstimatedDurationMins,
		HasCertificate:        req.HasCertificate,
		CertificateTemplate:   req.CertificateTemplate,
		MaxStudents:           req.MaxStudents,
		IsFeatured:            req.IsFeatured,
		IsTrending:            req.IsTrending,
		IsNew:                 req.IsNew,
		SEOTitle:              req.SEOTitle,
		SEODescription:        req.SEODescription,
		SEOKeywords:           req.SEOKeywords,
		PrerequisitesText:     req.PrerequisitesText,
		TargetAudience:        req.TargetAudience,
		LearningOutcomes:      req.LearningOutcomes,
		PrimaryInstructorID:   req.PrimaryInstructorID,
		CategoryIDs:           req.CategoryIDs,
	}

	courseEntity, err := h.updateCourseHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update course: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"course": courseEntity})
}

// DeleteCourse deletes a course
func (h *CourseRESTHandler) DeleteCourse(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		var idBody struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&idBody); err == nil && idBody.ID != "" {
			id = idBody.ID
		}
	}
	if id == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	err := h.db.WithContext(c.Request.Context()).Delete(&models.LmsCourse{}, "id = ?", id).Error
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete course: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Course deleted successfully"})
}

// ListCourses lists courses with filters
func (h *CourseRESTHandler) ListCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
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
		api_response.Error(c, http.StatusInternalServerError, "Failed to list courses: "+err.Error())
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
		api_response.Error(c, http.StatusInternalServerError, "Failed to search courses: "+err.Error())
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

// BulkStatusChange changes the status of multiple courses
func (h *CourseRESTHandler) BulkStatusChange(c *gin.Context) {
	var req struct {
		CourseIDs []string `json:"courseIds" binding:"required,min=1"`
		Status    string   `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"draft":          true,
		"pending_review": true,
		"published":      true,
		"archived":       true,
	}
	if !validStatuses[req.Status] {
		api_response.Error(c, http.StatusBadRequest, "Invalid status: "+req.Status)
		return
	}

	// Update each course
	updated := []string{}
	failed := []string{}

	for _, courseIDStr := range req.CourseIDs {
		courseUUID, err := uuid.Parse(courseIDStr)
		if err != nil {
			failed = append(failed, courseIDStr)
			continue
		}

		// Get the course first
		query := courseservice.GetCourseQuery{ID: courseIDStr}
		_, err = h.getCourseHandler.Handle(c.Request.Context(), query)
		if err != nil {
			failed = append(failed, courseIDStr)
			continue
		}

		// Update status
		if err := h.db.Model(&models.LmsCourse{}).Where("id = ?", courseUUID).Update("status", req.Status).Error; err != nil {
			failed = append(failed, courseIDStr)
			continue
		}

		updated = append(updated, courseIDStr)
	}

	api_response.Success(c, gin.H{
		"message": "Bulk status change completed",
		"updated": updated,
		"failed":  failed,
		"total":   len(updated),
	})
}
