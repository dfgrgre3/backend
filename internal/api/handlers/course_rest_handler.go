package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	api_response "thanawy-backend/internal/api/response"
	command "thanawy-backend/internal/app/command/course"
	querypkg "thanawy-backend/internal/app/query/course"
	"thanawy-backend/internal/domain/course"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CourseRESTHandler handles REST API requests for courses
type CourseRESTHandler struct {
	// Command handlers
	createCourseHandler   *command.CreateCourseHandler
	updateCourseHandler   *command.UpdateCourseHandler
	enrollUserHandler     *command.EnrollUserHandler
	updateProgressHandler *command.UpdateProgressHandler

	// Query handlers
	getCourseHandler   *querypkg.GetCourseHandler
	listCoursesHandler *querypkg.ListCoursesHandler

	// Enrollment query handler
	getEnrollmentHandler *querypkg.GetEnrollmentHandler

	// Service
	courseService *course.CourseService

	// DB for direct operations (for audit logging, etc.)
	db *gorm.DB
}

// NewCourseRESTHandler creates a new REST course handler
func NewCourseRESTHandler(
	courseService *course.CourseService,
	createCourseHandler *command.CreateCourseHandler,
	updateCourseHandler *command.UpdateCourseHandler,
	enrollUserHandler *command.EnrollUserHandler,
	updateProgressHandler *command.UpdateProgressHandler,
	getCourseHandler *querypkg.GetCourseHandler,
	listCoursesHandler *querypkg.ListCoursesHandler,
	getEnrollmentHandler *querypkg.GetEnrollmentHandler,
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
		getEnrollmentHandler:  getEnrollmentHandler,
		db:                    db,
	}
}

// CreateCourseRequest represents the REST request body for creating a course
type CreateCourseRequest struct {
	Title                 string   `json:"title" binding:"required"`
	Slug                  string   `json:"slug" binding:"required"`
	ShortDescription      *string  `json:"shortDescription"`
	LongDescription       *string  `json:"longDescription"`
	CoverImageURL         *string  `json:"coverImageUrl"`
	PromoVideoURL         *string  `json:"promoVideoUrl"`
	Level                 string   `json:"level" binding:"required"`
	Language              string   `json:"language" binding:"required"`
	EstimatedDurationMins int      `json:"estimatedDurationMins"`
	HasCertificate        bool     `json:"hasCertificate"`
	CertificateTemplate   *string  `json:"certificateTemplate"`
	MaxStudents           *int     `json:"maxStudents"`
	SEOTitle              *string  `json:"seoTitle"`
	SEODescription        *string  `json:"seoDescription"`
	SEOKeywords           []string `json:"seoKeywords"`
	PrerequisitesText     *string  `json:"prerequisitesText"`
	TargetAudience        *string  `json:"targetAudience"`
	LearningOutcomes      []string `json:"learningOutcomes"`
	PrimaryInstructorID   string   `json:"primaryInstructorId" binding:"required"`
	CategoryIDs           []string `json:"categoryIds"`
}

// UpdateCourseRequest represents the REST request body for updating a course
type UpdateCourseRequest struct {
	Title                 *string  `json:"title"`
	Slug                  *string  `json:"slug"`
	ShortDescription      *string  `json:"shortDescription"`
	LongDescription       *string  `json:"longDescription"`
	CoverImageURL         *string  `json:"coverImageUrl"`
	PromoVideoURL         *string  `json:"promoVideoUrl"`
	Level                 *string  `json:"level"`
	Language              *string  `json:"language"`
	EstimatedDurationMins *int     `json:"estimatedDurationMins"`
	HasCertificate        *bool    `json:"hasCertificate"`
	CertificateTemplate   *string  `json:"certificateTemplate"`
	MaxStudents           *int     `json:"maxStudents"`
	IsFeatured            *bool    `json:"isFeatured"`
	IsTrending            *bool    `json:"isTrending"`
	IsNew                 *bool    `json:"isNew"`
	SEOTitle              *string  `json:"seoTitle"`
	SEODescription        *string  `json:"seoDescription"`
	SEOKeywords           []string `json:"seoKeywords"`
	PrerequisitesText     *string  `json:"prerequisitesText"`
	TargetAudience        *string  `json:"targetAudience"`
	LearningOutcomes      []string `json:"learningOutcomes"`
	PrimaryInstructorID   *string  `json:"primaryInstructorId"`
	CategoryIDs           []string `json:"categoryIds"`
}

// SetPricingRequest represents the REST request body for course pricing.
type SetPricingRequest struct {
	Type                     string   `json:"type" binding:"required"`
	Amount                   float64  `json:"amount"`
	CurrencyCode             string   `json:"currencyCode" binding:"required"`
	SubscriptionDurationDays *int     `json:"subscriptionDurationDays"`
	DiscountPrice            *float64 `json:"discountPrice"`
	DiscountStartAt          *int64   `json:"discountStartAt"`
	DiscountEndAt            *int64   `json:"discountEndAt"`
	SubscriptionPlanID       *string  `json:"subscriptionPlanId"`
}

// =============================================================
// Course CRUD Endpoints
// =============================================================

// CreateCourse creates a new course
func (h *CourseRESTHandler) CreateCourse(c *gin.Context) {
	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	cmd := command.CreateCourseCommand{
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

// GetCourse retrieves a course by ID or slug
func (h *CourseRESTHandler) GetCourse(c *gin.Context) {
	id := c.Param("id")
	slug := c.Query("slug")

	query := querypkg.GetCourseQuery{
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

func (h *CourseRESTHandler) GetReviews(c *gin.Context) {
	id := c.Param("id")

	reviews, err := h.courseService.GetReviews(c.Request.Context(), id)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to get course reviews: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"reviews": reviews})
}

// UpdateCourse updates an existing course
func (h *CourseRESTHandler) UpdateCourse(c *gin.Context) {
	id := c.Param("id")

	var req UpdateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	cmd := command.UpdateCourseCommand{
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

	err := h.courseService.DeleteCourse(c.Request.Context(), id)
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

	query := querypkg.ListCoursesQuery{
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

// GetPricing returns the pricing configuration for a course.
func (h *CourseRESTHandler) GetPricing(c *gin.Context) {
	courseID := c.Param("id")
	if _, err := uuid.Parse(courseID); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	pricing, err := h.courseService.GetPricing(c.Request.Context(), courseID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Pricing not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch pricing")
		return
	}

	api_response.Success(c, gin.H{"pricing": pricing})
}

// SetPricing creates or updates the pricing configuration for a course.
func (h *CourseRESTHandler) SetPricing(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	var req SetPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}
	if req.Amount < 0 || (req.DiscountPrice != nil && *req.DiscountPrice < 0) ||
		(req.SubscriptionDurationDays != nil && *req.SubscriptionDurationDays < 0) {
		api_response.Error(c, http.StatusBadRequest, "Pricing values cannot be negative")
		return
	}

	pricing := &course.Pricing{
		CourseID:                 parsedCourseID,
		Type:                     course.PricingType(req.Type),
		Amount:                   req.Amount,
		CurrencyCode:             req.CurrencyCode,
		SubscriptionDurationDays: req.SubscriptionDurationDays,
		DiscountPrice:            req.DiscountPrice,
		SubscriptionPlanID:       req.SubscriptionPlanID,
		IsActive:                 true,
	}
	if req.DiscountStartAt != nil {
		discountStartAt := time.Unix(*req.DiscountStartAt, 0)
		pricing.DiscountStartAt = &discountStartAt
	}
	if req.DiscountEndAt != nil {
		discountEndAt := time.Unix(*req.DiscountEndAt, 0)
		pricing.DiscountEndAt = &discountEndAt
	}

	result, err := h.courseService.SetPricing(c.Request.Context(), courseID, pricing)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to set pricing")
		return
	}

	api_response.Success(c, gin.H{"pricing": result})
}

// =============================================================
// Course Workflow Endpoints
// =============================================================

// SubmitForReview submits a course for review
func (h *CourseRESTHandler) SubmitForReview(c *gin.Context) {
	id := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	err := h.courseService.SubmitForReview(c.Request.Context(), id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to submit for review: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "submit_for_review", "userId": uid})

	api_response.Success(c, gin.H{
		"message": "Course submitted for review",
		"status":  string(course.CourseStatusUnderReview),
	})
}

// ApproveCourse approves a course
func (h *CourseRESTHandler) ApproveCourse(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ReviewerID string `json:"reviewerId" binding:"required"`
		Notes      string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	err := h.courseService.ApproveCourse(c.Request.Context(), id, req.ReviewerID, req.Notes)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to approve course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "approve", "reviewerId": req.ReviewerID})

	api_response.Success(c, gin.H{
		"message": "Course approved",
		"status":  string(course.CourseStatusPublished),
	})
}

// RejectCourse rejects a course
func (h *CourseRESTHandler) RejectCourse(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ReviewerID string `json:"reviewerId" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	err := h.courseService.RejectCourse(c.Request.Context(), id, req.ReviewerID, req.Reason)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to reject course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "reject", "reviewerId": req.ReviewerID, "reason": req.Reason})

	api_response.Success(c, gin.H{
		"message": "Course rejected",
		"status":  string(course.CourseStatusRejected),
	})
}

// ArchiveCourse archives a course
func (h *CourseRESTHandler) ArchiveCourse(c *gin.Context) {
	id := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	err := h.courseService.ArchiveCourse(c.Request.Context(), id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to archive course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "archive", "userId": uid})

	api_response.Success(c, gin.H{
		"message": "Course archived",
		"status":  string(course.CourseStatusArchived),
	})
}

// UnarchiveCourse unarchives a course
func (h *CourseRESTHandler) UnarchiveCourse(c *gin.Context) {
	id := c.Param("id")

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	err := h.courseService.UnarchiveCourse(c.Request.Context(), id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to unarchive course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "unarchive", "userId": uid})

	api_response.Success(c, gin.H{
		"message": "Course unarchived",
		"status":  string(course.CourseStatusDraft),
	})
}

// GetCoursesPendingReview returns courses awaiting admin review
func (h *CourseRESTHandler) GetCoursesPendingReview(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Use the query handler to get courses with UNDER_REVIEW status
	statusStr := "UNDER_REVIEW"
	filter := querypkg.ListCoursesQuery{
		Status: &statusStr,
		Page:   page,
		Limit:  limit,
	}

	courses, total, err := h.listCoursesHandler.Handle(c.Request.Context(), filter)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch pending courses: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{
		"courses": courses,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (int(total) + limit - 1) / limit,
		},
	})
}

// BulkStatusChange performs batch status changes on multiple courses
func (h *CourseRESTHandler) BulkStatusChange(c *gin.Context) {
	var req struct {
		IDs    []string `json:"ids" binding:"required"`
		Action string   `json:"action" binding:"required"`
		Reason string   `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	results := make([]gin.H, 0, len(req.IDs))
	successCount := 0
	failureCount := 0

	for _, courseID := range req.IDs {
		var err error
		var status string

		switch req.Action {
		case "approve":
			err = h.courseService.ApproveCourse(c.Request.Context(), courseID, uid, req.Reason)
			status = string(course.CourseStatusPublished)
		case "reject":
			err = h.courseService.RejectCourse(c.Request.Context(), courseID, uid, req.Reason)
			status = string(course.CourseStatusRejected)
		case "archive":
			err = h.courseService.ArchiveCourse(c.Request.Context(), courseID)
			status = string(course.CourseStatusArchived)
		case "unarchive":
			err = h.courseService.UnarchiveCourse(c.Request.Context(), courseID)
			status = string(course.CourseStatusDraft)
		default:
			err = fmt.Errorf("invalid action: %s", req.Action)
		}

		if err != nil {
			failureCount++
			results = append(results, gin.H{
				"courseId": courseID,
				"success":  false,
				"error":    err.Error(),
			})
		} else {
			successCount++
			// Log audit for each successful operation
			h.logAudit(c, "BULK_STATUS_CHANGE", "course", courseID, gin.H{
				"action": req.Action,
				"status": status,
				"reason": req.Reason,
				"userId": uid,
			})
			results = append(results, gin.H{
				"courseId": courseID,
				"success":  true,
				"status":   status,
			})
		}
	}

	api_response.Success(c, gin.H{
		"message": fmt.Sprintf("Bulk operation completed: %d succeeded, %d failed", successCount, failureCount),
		"results": results,
		"summary": gin.H{
			"total":   len(req.IDs),
			"success": successCount,
			"failed":  failureCount,
		},
	})
}

// =============================================================
// Enrollment Endpoints
// =============================================================

// EnrollUser enrolls a user in a course
func (h *CourseRESTHandler) EnrollUser(c *gin.Context) {
	var req struct {
		CourseID string `json:"courseId" binding:"required"`
		UserID   string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	cmd := command.EnrollUserCommand{
		CourseID: req.CourseID,
		UserID:   req.UserID,
	}

	enrollment, err := h.enrollUserHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to enroll user: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"enrollment": enrollment})
}

// GetEnrollment retrieves user enrollment
func (h *CourseRESTHandler) GetEnrollment(c *gin.Context) {
	courseID := c.Param("courseId")
	userID := c.Param("userId")

	query := querypkg.GetEnrollmentQuery{
		CourseID: courseID,
		UserID:   userID,
	}

	enrollment, err := h.getEnrollmentHandler.Handle(c.Request.Context(), query)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Enrollment not found")
		return
	}

	api_response.Success(c, gin.H{"enrollment": enrollment})
}

// UpdateProgress updates enrollment progress
func (h *CourseRESTHandler) UpdateProgress(c *gin.Context) {
	courseID := c.Param("courseId")
	userID := c.Param("userId")

	var req struct {
		Progress float64 `json:"progress" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	cmd := command.UpdateProgressCommand{
		CourseID: courseID,
		UserID:   userID,
		Progress: req.Progress,
	}

	err := h.updateProgressHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update progress: "+err.Error())
		return
	}

	// Fetch updated enrollment
	query := querypkg.GetEnrollmentQuery{
		CourseID: courseID,
		UserID:   userID,
	}
	enrollment, _ := h.getEnrollmentHandler.Handle(c.Request.Context(), query)

	api_response.Success(c, gin.H{"enrollment": enrollment})
}

// =============================================================
// Section Endpoints
// =============================================================

// CreateSectionRequest represents the REST request body for creating a section
type CreateSectionRequest struct {
	CourseID      string `json:"courseId" binding:"required"`
	Title         string `json:"title" binding:"required"`
	OrderIndex    int    `json:"orderIndex"`
	AvailableFrom *int64 `json:"availableFrom"`
	DripDelayDays *int   `json:"dripDelayDays"`
}

// UpdateSectionRequest represents the REST request body for updating a section
type UpdateSectionRequest struct {
	Title         *string `json:"title"`
	OrderIndex    *int    `json:"orderIndex"`
	AvailableFrom *int64  `json:"availableFrom"`
	DripDelayDays *int    `json:"dripDelayDays"`
}

// CreateSection creates a new section
func (h *CourseRESTHandler) CreateSection(c *gin.Context) {
	var req CreateSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	section := &course.Section{
		CourseID:   uuid.MustParse(req.CourseID),
		Title:      req.Title,
		OrderIndex: req.OrderIndex,
	}

	if req.AvailableFrom != nil {
		t := time.Unix(*req.AvailableFrom, 0)
		section.AvailableFrom = &t
	}
	section.DripDelayDays = req.DripDelayDays

	createdSection, err := h.courseService.CreateSection(c.Request.Context(), req.CourseID, section)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create section: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"section": createdSection})
}

// UpdateSection updates a section
func (h *CourseRESTHandler) UpdateSection(c *gin.Context) {
	id := c.Param("sectionId")

	var req UpdateSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if _, err := h.courseService.GetCourse(c.Request.Context(), id); err != nil {
		api_response.Error(c, http.StatusNotFound, "Section not found")
		return
	}

	// This is a simplified approach - in production you'd have a proper GetSection method
	// For now, we'll return an error
	api_response.Error(c, http.StatusNotImplemented, "UpdateSection requires GetSection in service")
}

// DeleteSection deletes a section
func (h *CourseRESTHandler) DeleteSection(c *gin.Context) {
	id := c.Param("id")

	err := h.courseService.DeleteSection(c.Request.Context(), id)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete section: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Section deleted successfully"})
}

// ListSections lists sections for a course
func (h *CourseRESTHandler) ListSections(c *gin.Context) {
	// This would need a ListSections method in the service
	// For now, return not implemented
	api_response.Error(c, http.StatusNotImplemented, "ListSections requires service implementation")
}

// ReorderSections reorders sections in a course
func (h *CourseRESTHandler) ReorderSections(c *gin.Context) {
	courseID := c.Param("courseId")

	var req struct {
		SectionIDs []string `json:"sectionIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	err := h.courseService.ReorderSections(c.Request.Context(), courseID, req.SectionIDs)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reorder sections: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Sections reordered successfully"})
}

// =============================================================
// Lesson Endpoints
// =============================================================

// CreateLessonRequest represents the REST request body for creating a lesson
type CreateLessonRequest struct {
	SectionID        string  `json:"sectionId" binding:"required"`
	Title            string  `json:"title" binding:"required"`
	Type             string  `json:"type" binding:"required"`
	Content          *string `json:"content"`
	MediaURL         *string `json:"mediaUrl"`
	DurationSeconds  int     `json:"durationSeconds"`
	IsFreePreview    bool    `json:"isFreePreview"`
	OrderIndex       int     `json:"orderIndex"`
	AvailabilityType string  `json:"availabilityType"`
	AvailableFrom    *int64  `json:"availableFrom"`
	DripDelayDays    *int    `json:"dripDelayDays"`
}

// UpdateLessonRequest represents the REST request body for updating a lesson
type UpdateLessonRequest struct {
	Title            *string `json:"title"`
	Type             *string `json:"type"`
	Content          *string `json:"content"`
	MediaURL         *string `json:"mediaUrl"`
	DurationSeconds  *int    `json:"durationSeconds"`
	IsFreePreview    *bool   `json:"isFreePreview"`
	OrderIndex       *int    `json:"orderIndex"`
	AvailabilityType *string `json:"availabilityType"`
	AvailableFrom    *int64  `json:"availableFrom"`
	DripDelayDays    *int    `json:"dripDelayDays"`
}

// CreateLesson creates a new lesson
func (h *CourseRESTHandler) CreateLesson(c *gin.Context) {
	var req CreateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	lesson := &course.Lesson{
		SectionID:        uuid.MustParse(req.SectionID),
		Title:            req.Title,
		Type:             course.LessonType(req.Type),
		Content:          req.Content,
		MediaURL:         req.MediaURL,
		DurationSeconds:  req.DurationSeconds,
		IsFreePreview:    req.IsFreePreview,
		OrderIndex:       req.OrderIndex,
		AvailabilityType: course.AvailabilityType(req.AvailabilityType),
	}

	if req.AvailableFrom != nil {
		t := time.Unix(*req.AvailableFrom, 0)
		lesson.AvailableFrom = &t
	}
	lesson.DripDelayDays = req.DripDelayDays

	createdLesson, err := h.courseService.CreateLesson(c.Request.Context(), req.SectionID, lesson)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create lesson: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"lesson": createdLesson})
}

// UpdateLesson updates a lesson
func (h *CourseRESTHandler) UpdateLesson(c *gin.Context) {
	var req UpdateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// This would need a GetLesson method in the service
	api_response.Error(c, http.StatusNotImplemented, "UpdateLesson requires GetLesson in service")
}

// DeleteLesson deletes a lesson
func (h *CourseRESTHandler) DeleteLesson(c *gin.Context) {
	id := c.Param("id")

	err := h.courseService.DeleteLesson(c.Request.Context(), id)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete lesson: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Lesson deleted successfully"})
}

// ListLessons lists lessons for a section
func (h *CourseRESTHandler) ListLessons(c *gin.Context) {
	// This would need a ListLessons method in the service
	api_response.Error(c, http.StatusNotImplemented, "ListLessons requires service implementation")
}

// ReorderLessons reorders lessons in a section
func (h *CourseRESTHandler) ReorderLessons(c *gin.Context) {
	sectionID := c.Param("sectionId")

	var req struct {
		LessonIDs []string `json:"lessonIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	err := h.courseService.ReorderLessons(c.Request.Context(), sectionID, req.LessonIDs)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reorder lessons: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Lessons reordered successfully"})
}

// =============================================================
// Helper functions
// =============================================================

func optionalStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func boolPtr(s string) *bool {
	if s == "" {
		return nil
	}
	b := s == "true"
	return &b
}

// logAudit logs audit events to the database
func (h *CourseRESTHandler) logAudit(c *gin.Context, action, resourceType, resourceID string, details gin.H) {
	if h.db == nil {
		return
	}

	// Get user ID from context
	userID, _ := c.Get("userId")
	var userIDStr *string
	if uid, ok := userID.(string); ok && uid != "" {
		userIDStr = &uid
	}

	// Get IP address
	ip := c.ClientIP()

	// Get user agent
	userAgent := c.GetHeader("User-Agent")

	// Convert details to JSON
	changesJSON := ""
	if len(details) > 0 {
		if bytes, err := json.Marshal(details); err == nil {
			changesJSON = string(bytes)
		}
	}

	auditLog := models.AuditLog{
		UserID:     userIDStr,
		EventType:  action,
		Action:     action,
		Resource:   resourceType,
		ResourceID: resourceID,
		Changes:    changesJSON,
		Metadata:   changesJSON,
		IP:         ip,
		UserAgent:  userAgent,
	}

	if err := h.db.Create(&auditLog).Error; err != nil {
		// Log error but don't fail the request
		// In production, you might want to use a proper logger
	}
}
