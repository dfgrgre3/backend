package http

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"thanawy-backend/internal/domain/certificate"
	"thanawy-backend/internal/domain/subject"

	"github.com/gin-gonic/gin"
)

type SubjectHandler struct {
	service           *subject.Service
	certificateService *certificate.Service
}

func NewSubjectHandler(service *subject.Service, certService *certificate.Service) *SubjectHandler {
	return &SubjectHandler{
		service:            service,
		certificateService: certService,
	}
}

// ============================================================================
// Cache Control Helpers
// ============================================================================

const (
	cachePublic  = "public, max-age=%d, s-maxage=%d"
	cachePrivate = "private, no-cache, must-revalidate"
)

func setPublicCache(c *gin.Context, maxAge time.Duration) {
	c.Header("Cache-Control", fmt.Sprintf(cachePublic, int(maxAge.Seconds()), int(maxAge.Seconds())))
}

func setPrivateCache(c *gin.Context) {
	c.Header("Cache-Control", cachePrivate)
}

// ============================================================================
// Subject CRUD
// ============================================================================

func (h *SubjectHandler) CreateSubject(c *gin.Context) {
	var input subject.CreateSubjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	s, err := h.service.CreateSubject(c.Request.Context(), input)
	if err != nil {
		switch err {
		case subject.ErrInvalidInput:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case subject.ErrSubjectExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subject"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"course": s})
}

func (h *SubjectHandler) GetSubject(c *gin.Context) {
	ifNoneMatch := c.GetHeader("If-None-Match")
	idOrSlug := c.Param("id")

	userID := c.Query("userId")
	if userID == "" {
		if uid, exists := c.Get("userId"); exists {
			if s, ok := uid.(string); ok {
				userID = s
			}
		}
	}

	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}

	detail, err := h.service.GetSubjectWithDetails(c.Request.Context(), idOrSlug, userIDPtr)
	if err != nil {
		if err == subject.ErrSubjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subject"})
		return
	}

	// Cache public subject data for 5 minutes if user not enrolled
	if !detail.IsEnrolled {
		setPublicCache(c, 5*time.Minute)
	} else {
		setPrivateCache(c)
	}

	// ETag support for caching
	etag := fmt.Sprintf(`"%s-%d"`, detail.Subject.ID, detail.Subject.UpdatedAt.Unix())
	c.Header("ETag", etag)
	if ifNoneMatch == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subject":    detail,
		"enrollment": gin.H{"isEnrolled": detail.IsEnrolled, "progress": detail.Progress},
	})
}

func (h *SubjectHandler) UpdateSubject(c *gin.Context) {
	id := c.Param("id")

	var input subject.UpdateSubjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}
	input.ID = id

	s, err := h.service.UpdateSubject(c.Request.Context(), input)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subject"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"course": s})
}

func (h *SubjectHandler) DeleteSubject(c *gin.Context) {
	id := c.Param("id")

	err := h.service.DeleteSubject(c.Request.Context(), id)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subject"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Subject deleted successfully"})
}

func (h *SubjectHandler) ListSubjects(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filter := subject.ListSubjectsFilter{
		Page:  page,
		Limit: limit,
	}

	if categoryID := c.Query("categoryId"); categoryID != "" {
		filter.CategoryID = &categoryID
	}
	if level := c.Query("level"); level != "" {
		filter.Level = &level
	}
	if c.Query("isPublished") != "" {
		filter.IsPublished = boolPtr(c.Query("isPublished") == "true")
	}
	if c.Query("isActive") != "" {
		filter.IsActive = boolPtr(c.Query("isActive") == "true")
	}
	if c.Query("isFeatured") != "" {
		filter.IsFeatured = boolPtr(c.Query("isFeatured") == "true")
	}
	if search := c.Query("search"); search != "" {
		filter.Search = &search
	}
	if includeUnpub := c.Query("includeUnpublished"); includeUnpub == "true" {
		filter.IncludeUnpublished = true
	}
	if sortBy := c.Query("sortBy"); sortBy != "" {
		filter.SortBy = sortBy
	}
	if sortOrder := c.Query("sortOrder"); sortOrder != "" {
		filter.SortOrder = sortOrder
	}

	result, err := h.service.ListSubjects(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list subjects"})
		return
	}

	// Cache public listing for 2 minutes
	setPublicCache(c, 2*time.Minute)

	c.JSON(http.StatusOK, gin.H{
		"items": result.Subjects,
		"pagination": gin.H{
			"page":       result.Page,
			"limit":      result.Limit,
			"total":      result.Total,
			"totalPages": result.TotalPages,
		},
	})
}

// ============================================================================
// Curriculum
// ============================================================================

func (h *SubjectHandler) UpdateCurriculum(c *gin.Context) {
	subjectID := c.Param("id")

	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	var topicsInput []subject.TopicInput
	if curriculum, ok := raw["curriculum"]; ok {
		if topics, ok := curriculum.([]interface{}); ok {
			topicsInput = parseTopics(topics)
		}
	} else if topics, ok := raw["topics"]; ok {
		if topicsArr, ok := topics.([]interface{}); ok {
			topicsInput = parseTopics(topicsArr)
		}
	}

	if topicsInput == nil {
		parseTopicsFromRaw(raw, &topicsInput)
	}

	input := subject.CurriculumInput{Topics: topicsInput}

	err := h.service.UpdateCurriculum(c.Request.Context(), subjectID, input)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update curriculum"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Curriculum updated successfully"})
}

func (h *SubjectHandler) GetCurriculum(c *gin.Context) {
	subjectID := c.Param("id")

	curriculum, err := h.service.GetCurriculum(c.Request.Context(), subjectID)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch curriculum"})
		}
		return
	}

	// Curriculum changes infrequently - cache for 10 minutes
	setPublicCache(c, 10*time.Minute)

	c.JSON(http.StatusOK, gin.H{
		"topics": curriculum.Topics,
		"stats": gin.H{
			"chaptersCount":        curriculum.ChaptersCount,
			"lessonsCount":         curriculum.LessonsCount,
			"freeLessonsCount":     curriculum.FreeLessonsCount,
			"totalDurationMinutes": curriculum.TotalDuration,
		},
	})
}

// ============================================================================
// Enrollment
// ============================================================================

func (h *SubjectHandler) EnrollCourse(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courseID := c.Param("id")

	err := h.service.EnrollUser(c.Request.Context(), userID, courseID)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case subject.ErrAlreadyEnrolled:
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Already enrolled"})
		case subject.ErrPaymentRequired:
			c.JSON(http.StatusOK, gin.H{"requiresPayment": true, "message": "Payment required"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enroll: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Enrolled successfully"})
}

func (h *SubjectHandler) UnenrollCourse(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courseID := c.Param("id")

	err := h.service.UnenrollUser(c.Request.Context(), userID, courseID)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case subject.ErrNotEnrolled:
			c.JSON(http.StatusNotFound, gin.H{"error": "You are not enrolled in this course"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unenroll"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Unenrolled successfully"})
}

func (h *SubjectHandler) GetEnrollmentStatus(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courseID := c.Param("id")

	status, err := h.service.GetEnrollmentStatus(c.Request.Context(), userID, courseID)
	if err != nil {
		if err == subject.ErrSubjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch enrollment status"})
		return
	}

	setPrivateCache(c)
	c.JSON(http.StatusOK, status)
}

func (h *SubjectHandler) CompleteCourse(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courseID := c.Param("id")

	err := h.service.CompleteCourse(c.Request.Context(), userID, courseID)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case subject.ErrNotEnrolled:
			c.JSON(http.StatusBadRequest, gin.H{"error": "You must be enrolled to complete this course"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Course completed successfully!"})
}

func (h *SubjectHandler) GetMyCourses(c *gin.Context) {
	setPrivateCache(c)

	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courses, err := h.service.GetUserCourses(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch courses"})
		return
	}

	formatted := make([]gin.H, 0, len(courses))
	for _, course := range courses {
		title := course.SubjectName
		if course.SubjectNameAr != nil && *course.SubjectNameAr != "" {
			title = *course.SubjectNameAr
		}

		formatted = append(formatted, gin.H{
			"id":             course.SubjectID,
			"enrollmentId":   course.ID,
			"title":          title,
			"name":           course.SubjectName,
			"nameAr":         course.SubjectNameAr,
			"thumbnailUrl":   course.ThumbnailUrl,
			"progress":       course.Progress,
			"enrolled":       true,
			"lastAccessedAt": course.UpdatedAt,
			"enrolledAt":     course.EnrolledAt,
			"rating":         course.Rating,
			"level":          course.Level,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"courses": formatted,
		"data":    gin.H{"courses": formatted},
	})
}

func (h *SubjectHandler) GetCourseStudents(c *gin.Context) {
	setPrivateCache(c)

	courseID := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filter := subject.StudentsFilter{
		Page:     page,
		Limit:    limit,
		Progress: c.Query("progress"),
	}

	result, err := h.service.GetCourseStudents(c.Request.Context(), courseID, filter)
	if err != nil {
		if err == subject.ErrSubjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch students"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"students": result.Students,
		"pagination": gin.H{
			"page":       result.Page,
			"limit":      result.Limit,
			"total":      result.Total,
			"totalPages": result.TotalPages,
		},
	})
}

// ============================================================================
// Lesson Progress
// ============================================================================

func (h *SubjectHandler) UpdateLessonProgress(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	lessonID := c.Param("id")

	var input subject.ProgressInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if err := h.service.UpdateLessonProgress(c.Request.Context(), userID, lessonID, input); err != nil {
		switch err {
		case subject.ErrNotEnrolled:
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not enrolled in this course"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SubjectHandler) GetLessonProgress(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	subjectID := c.Param("id")

	progress, err := h.service.GetLessonProgress(c.Request.Context(), userID, subjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch progress"})
		return
	}

	setPrivateCache(c)
	c.JSON(http.StatusOK, gin.H{"progress": progress})
}

// ============================================================================
// Courses (Lessons List)
// ============================================================================

func (h *SubjectHandler) GetCourseLessons(c *gin.Context) {
	setPublicCache(c, 5*time.Minute)

	courseID := c.Param("id")

	curriculum, err := h.service.GetCurriculum(c.Request.Context(), courseID)
	if err != nil {
		if err == subject.ErrSubjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch lessons"})
		return
	}

	type LessonResponse struct {
		ID              string `json:"id"`
		Title           string `json:"title"`
		Description     string `json:"description"`
		VideoUrl        string `json:"videoUrl"`
		IsFree          bool   `json:"isFree"`
		Order           int    `json:"order"`
		DurationMinutes int    `json:"durationMinutes"`
		Type            string `json:"type"`
	}

	lessons := make([]LessonResponse, 0)
	for _, topic := range curriculum.Topics {
		for _, st := range topic.SubTopics {
			dur := st.Duration
			if dur == 0 {
				dur = st.DurationMin
			}
			desc := ""
			if st.Description != nil {
				desc = *st.Description
			}
			videoURL := ""
			if st.VideoUrl != nil {
				videoURL = *st.VideoUrl
			}

			lessons = append(lessons, LessonResponse{
				ID:              st.ID,
				Title:           st.Title,
				Description:     desc,
				VideoUrl:        videoURL,
				IsFree:          st.IsFree,
				Order:           st.Order,
				DurationMinutes: dur,
				Type:            st.Type,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"lessons": lessons,
	})
}

// ============================================================================
// Reviews
// ============================================================================

func (h *SubjectHandler) CreateReview(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courseID := c.Param("id")

	var input subject.CreateReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if input.Rating < 1 || input.Rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rating must be between 1 and 5"})
		return
	}

	review, err := h.service.CreateReview(c.Request.Context(), userID, courseID, input)
	if err != nil {
		switch err {
		case subject.ErrSubjectNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case subject.ErrNotEnrolled:
			c.JSON(http.StatusForbidden, gin.H{"error": "You must be enrolled to review this course"})
		case subject.ErrReviewExists:
			c.JSON(http.StatusConflict, gin.H{"error": "You have already reviewed this course"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create review"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"review": review,
		},
	})
}

func (h *SubjectHandler) GetReviews(c *gin.Context) {
	setPublicCache(c, 3*time.Minute)

	courseID := c.Param("id")

	reviews, err := h.service.GetReviews(c.Request.Context(), courseID)
	if err != nil {
		if err == subject.ErrSubjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reviews"})
		return
	}

	stats, _ := h.service.GetReviewStats(c.Request.Context(), courseID)

	c.JSON(http.StatusOK, gin.H{
		"reviews": reviews,
		"stats":   stats,
	})
}

// ============================================================================
// Admin Operations
// ============================================================================

func (h *SubjectHandler) DuplicateCourse(c *gin.Context) {
	var input struct {
		CourseID string `json:"courseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Course ID is required"})
		return
	}

	duplicate, err := h.service.DuplicateSubject(c.Request.Context(), input.CourseID)
	if err != nil {
		if err == subject.ErrSubjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Course not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to duplicate course"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Course duplicated successfully",
		"course":  duplicate,
	})
}

func (h *SubjectHandler) BatchCourseAction(c *gin.Context) {
	var input struct {
		IDs    []string `json:"ids" binding:"required"`
		Action string   `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IDs and Action are required"})
		return
	}

	if err := h.service.BatchAction(c.Request.Context(), input.IDs, input.Action); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute batch action: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Batch action executed successfully",
	})
}

func (h *SubjectHandler) GetDashboardStats(c *gin.Context) {
	setPrivateCache(c)

	stats, err := h.service.GetDashboardStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dashboard stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ============================================================================
// Helpers
// ============================================================================

func boolPtr(b bool) *bool {
	return &b
}

func parseTopics(raw []interface{}) []subject.TopicInput {
	topics := make([]subject.TopicInput, 0, len(raw))
	for i, topicRaw := range raw {
		if topicMap, ok := topicRaw.(map[string]interface{}); ok {
			topic := subject.TopicInput{
				Title: getStringField(topicMap, "name", "title"),
				Order: i,
			}
			if id, ok := topicMap["id"].(string); ok {
				topic.ID = id
			}
			if order, ok := topicMap["order"].(float64); ok {
				topic.Order = int(order)
			}

			if subTopicsRaw, ok := topicMap["subTopics"]; ok {
				if subTopicsArr, ok := subTopicsRaw.([]interface{}); ok {
					topic.SubTopics = parseSubTopics(subTopicsArr)
				}
			}

			topics = append(topics, topic)
		}
	}
	return topics
}

func parseSubTopics(raw []interface{}) []subject.SubTopicInput {
	subTopics := make([]subject.SubTopicInput, 0, len(raw))
	for i, stRaw := range raw {
		if stMap, ok := stRaw.(map[string]interface{}); ok {
			st := subject.SubTopicInput{
				Title: getStringField(stMap, "name", "title"),
				Order: i,
				Type:  "VIDEO",
			}
			if id, ok := stMap["id"].(string); ok {
				st.ID = id
			}
			if order, ok := stMap["order"].(float64); ok {
				st.Order = int(order)
			}
			if t, ok := stMap["type"].(string); ok {
				st.Type = t
			}
			if isFree, ok := stMap["isFree"].(bool); ok {
				st.IsFree = isFree
			}
			if videoUrl, ok := stMap["videoUrl"].(string); ok {
				st.VideoUrl = &videoUrl
			}
			if duration, ok := stMap["duration"].(float64); ok {
				st.Duration = int(duration)
			}
			if durationMin, ok := stMap["durationMinutes"].(float64); ok {
				st.DurationMin = int(durationMin)
			}
			if desc, ok := stMap["description"].(string); ok {
				st.Description = &desc
			}

			subTopics = append(subTopics, st)
		}
	}
	return subTopics
}

func parseTopicsFromRaw(raw map[string]interface{}, topics *[]subject.TopicInput) {
	for _, key := range []string{"topics", "chapters", "curriculum"} {
		if val, ok := raw[key]; ok {
			if arr, ok := val.([]interface{}); ok {
				*topics = parseTopics(arr)
				return
			}
		}
	}
}

func getStringField(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

func (h *SubjectHandler) GetUserSubjects(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courses, err := h.service.GetUserCourses(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subjects"})
		return
	}

	type subjectResponse struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
	}

	response := make([]subjectResponse, 0)
	for _, c := range courses {
		response = append(response, subjectResponse{
			ID:      c.ID,
			Subject: c.SubjectName,
		})
	}

	c.JSON(http.StatusOK, response)
}