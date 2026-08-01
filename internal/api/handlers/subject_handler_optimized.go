package handlers

import (
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/events"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	queryOptimizer  *db.QueryOptimizer
	optimizerOnce   sync.Once
	enhancedCache   *cache.EnhancedCache
	cacheOnce       sync.Once
)

func getQueryOptimizer() *db.QueryOptimizer {
	optimizerOnce.Do(func() {
		queryOptimizer = db.NewQueryOptimizer(db.DB)
	})
	return queryOptimizer
}

func getEnhancedCache() *cache.EnhancedCache {
	cacheOnce.Do(func() {
		enhancedCache = cache.GetEnhancedCache()
	})
	return enhancedCache
}

// OptimizedGetSubjects uses QueryOptimizer and EnhancedCache for better performance
func OptimizedGetSubjects(c *gin.Context) {
	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, offsetErr := strconv.Atoi(c.Query("offset"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	if offsetErr != nil {
		offset = (page - 1) * limit
	} else {
		page = (offset / limit) + 1
	}

	// Build filters
	filters := db.SubjectFilters{
		CategoryID:  c.Query("categoryId"),
		Search:      sanitizeSearchTerm(c.Query("search")),
		Level:       c.Query("level"),
		IsFeatured:  c.Query("isFeatured") == "true",
		IsTrending:  c.Query("isTrending") == "true",
		IsNew:       c.Query("isNew") == "true",
	}

	if isPublished := c.Query("isPublished"); isPublished != "" {
		if isValidBoolean(isPublished) {
			val := isPublished == "true"
			filters.IsPublished = &val
		}
	}
	if isActive := c.Query("isActive"); isActive != "" {
		if isValidBoolean(isActive) {
			val := isActive == "true"
			filters.IsActive = &val
		}
	}
	if status := c.Query("status"); status != "" {
		if isValidCourseStatus(status) {
			filters.Status = status
		}
	}

	// Try cache first
	cacheKey := cache.SubjectListKey(page, limit, map[string]string{
		"categoryId":  filters.CategoryID,
		"search":      filters.Search,
		"level":       filters.Level,
		"isPublished": c.Query("isPublished"),
		"isActive":    c.Query("isActive"),
		"status":      filters.Status,
		"isFeatured":  c.Query("isFeatured"),
		"isTrending":  c.Query("isTrending"),
		"isNew":       c.Query("isNew"),
	})

	ctx := c.Request.Context()
	ec := getEnhancedCache()

	var responsePayload gin.H
	err := ec.GetOrSet(ctx, cacheKey, &responsePayload, cache.TTLSubjectList, func() (interface{}, error) {
		// Use QueryOptimizer for efficient database query
		qo := getQueryOptimizer()
		items, total, err := qo.OptimizedSubjectListQuery(ctx, filters, page, limit)
		if err != nil {
			return nil, err
		}

		// Format response
		formattedItems := make([]gin.H, 0, len(items))
		for _, item := range items {
			formattedItems = append(formattedItems, gin.H{
				"id":                     item.ID,
				"name":                   item.Name,
				"nameAr":                 item.NameAr,
				"code":                   item.Code,
				"description":            item.Description,
				"icon":                   item.Icon,
				"color":                  item.Color,
				"type":                   "COURSE",
				"isActive":               item.IsActive,
				"isPublished":            item.IsPublished,
				"price":                  item.Price,
				"level":                  item.Level,
				"instructorName":         item.InstructorName,
				"instructorId":           item.InstructorId,
				"categoryId":             item.CategoryId,
				"thumbnailUrl":           item.ThumbnailUrl,
				"trailerUrl":             item.TrailerUrl,
				"trailerDurationMinutes": item.TrailerDurationMinutes,
				"durationHours":          item.DurationHours,
				"requirements":           item.Requirements,
				"learningObjectives":     item.LearningObjectives,
				"seoTitle":               item.SeoTitle,
				"seoDescription":         item.SeoDescription,
				"slug":                   item.Slug,
				"rating":                 item.Rating,
				"enrolledCount":          item.EnrolledCount,
				"createdAt":              item.CreatedAt,
				"updatedAt":              item.UpdatedAt,
				"_count": gin.H{
					"enrollments": item.EnrolledCount,
					"topics":      item.TopicCount,
					"reviews":     0,
					"teachers":    0,
				},
			})
		}

		return gin.H{
			"items": formattedItems,
			"pagination": api_response.Pagination{
				Page:       page,
				Limit:      limit,
				Total:      total,
				TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
			},
			"subjects": formattedItems,
			"courses":  formattedItems,
			"offset":   offset,
		}, nil
	})

	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch subjects: "+err.Error())
		return
	}

	api_response.Success(c, responsePayload)
}

// OptimizedGetSubject fetches a single subject with curriculum using optimized queries
func OptimizedGetSubject(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	// Try cache first
	cacheKey := cache.SubjectDetailKey(id)
	ec := getEnhancedCache()

	var subject *db.SubjectDetail
	err := ec.GetOrSet(ctx, cacheKey, &subject, cache.TTLSubjectDetail, func() (interface{}, error) {
		qo := getQueryOptimizer()
		return qo.OptimizedSubjectDetailQuery(ctx, id)
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
		} else {
			log.Printf("Error fetching subject %q: %v", id, err)
			api_response.Error(c, http.StatusInternalServerError, "Failed to fetch subject")
		}
		return
	}

	// Check enrollment if userId is provided
	userID := c.Query("userId")
	if userID == "" {
		if uid, exists := c.Get("userId"); exists {
			if s, ok := uid.(string); ok {
				userID = s
			}
		}
	}

	var enrollment *models.Enrollment
	if userID != "" {
		qo := getQueryOptimizer()
		enrolled, err := qo.OptimizedEnrollmentCheck(ctx, userID, subject.ID)
		if err == nil && enrolled {
			// Fetch full enrollment details
			var e models.Enrollment
			if err := db.ReadDB(ctx).Where("user_id = ? AND subject_id = ?", userID, subject.ID).First(&e).Error; err == nil {
				enrollment = &e
			}
		}
	}

	// Wrap for frontend
	response := gin.H{
		"subject": subject,
		"data": gin.H{
			"subject": subject,
			"course":  subject,
		},
	}
	if enrollment != nil {
		response["enrollment"] = enrollment
	}

	api_response.Success(c, response)
}

// OptimizedGetCourseLessons fetches course lessons efficiently
func OptimizedGetCourseLessons(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	cacheKey := cache.SubjectCurriculumKey(id)
	ec := getEnhancedCache()

	var subject *db.SubjectDetail
	err := ec.GetOrSet(ctx, cacheKey, &subject, cache.TTLSubjectCurriculum, func() (interface{}, error) {
		qo := getQueryOptimizer()
		return qo.OptimizedSubjectDetailQuery(ctx, id)
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
		} else {
			log.Printf("Error fetching course lessons %q: %v", id, err)
			api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course lessons")
		}
		return
	}

// Transform curriculum to lessons format
		var lessons []Lesson
		for _, topic := range subject.Curriculum {
			for _, st := range topic.SubTopics {
				audioDuration := 0
				if st.AudioDurationSeconds != nil {
					audioDuration = *st.AudioDurationSeconds
				}
				l := Lesson{
					ID:                 st.ID,
					Title:              st.Title,
					Description:        stringOrEmpty(st.Description),
					VideoUrl:           stringOrEmpty(st.VideoUrl),
					AudioUrl:           stringOrEmpty(st.AudioUrl),
					AudioDuration:      audioDuration,
					ExternalLinkUrl:    stringOrEmpty(st.ExternalLinkUrl),
					ExternalLinkTitle:  stringOrEmpty(st.ExternalLinkTitle),
					Type:               string(st.Type),
					IsFree:             st.IsFree,
					Order:              st.Order,
					DurationMinutes:    st.DurationMinutes,
					ExamID:             stringOrEmpty(st.ExamID),
					IsDripEnabled:      st.IsDripEnabled,
					IsContentProtected: st.IsContentProtected,
					ViewCount:          st.ViewCount,
					CompletionCount:    st.CompletionCount,
				}
				if st.DripReleaseDate != nil {
					l.DripReleaseDate = st.DripReleaseDate.Format(time.RFC3339)
				}
				if len(st.SubtitleUrls) > 0 {
					l.HasSubtitles = true
				}
				if len(st.VideoChaptersData) > 0 {
					l.HasChapters = true
				}
				lessons = append(lessons, l)
			}
		}

	api_response.Success(c, gin.H{
		"lessons": lessons,
		"course": gin.H{
			"id":        subject.ID,
			"title":     subject.Name,
			"titleAr":   subject.NameAr,
			"status":    subject.Status,
"type":      string(subject.Type),
			"thumbnail": subject.ThumbnailUrl,
		},
	})
}

// OptimizedEnrollCourse enrolls a user in a course with optimized checks
func OptimizedEnrollCourse(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	courseId := c.Param("id")

	// Ensure the authenticated account exists
	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		log.Printf("[Enrollment] authenticated user was not found in database: %q", userId)
		api_response.Error(c, http.StatusUnauthorized, "User account was not found. Please sign in again or complete registration.")
		return
	}

	// Resolve and verify subject
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "verifying subject for enrollment")
		return
	}

	// Check if user is already enrolled (using batch check for efficiency)
	qo := getQueryOptimizer()
	ctx := c.Request.Context()
	enrolled, err := qo.OptimizedEnrollmentCheck(ctx, userId, subject.ID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to check enrollment status")
		return
	}
	if enrolled {
		api_response.Success(c, gin.H{"success": true, "message": "Already enrolled"})
		return
	}

	// Payment verification logic
	price, _ := subject.Price.Float64()
	if price > 0 {
		paid, err := qo.OptimizedPaymentCheck(ctx, userId, subject.ID)
		if err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to check payment status")
			return
		}
		if !paid {
			api_response.Success(c, gin.H{
				"error":           "Payment required for this course",
				"courseId":        courseId,
				"price":           price,
				"requiresPayment": true,
			})
			return
		}
	}

	if err := executeEnrollmentTransaction(userId, subject.ID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to enroll: "+err.Error())
		return
	}

	// Fire enrollment event for gamification/analytics
	fireEnrollmentEvent(c, userId, subject.ID, string(events.EventEnrollment))

	// Invalidate cache
	cache.NewCacheInvalidator().InvalidateSubject(ctx, subject.ID)

	api_response.Success(c, gin.H{"success": true, "message": "Enrolled successfully"})
}

// OptimizedUpdateLessonProgress updates lesson progress with batch operations
func OptimizedUpdateLessonProgress(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	lessonId := c.Param("id")

	var input struct {
		Completed           bool    `json:"completed"`
		LastWatchedPosition float64 `json:"lastWatchedPosition"`
		TimeSpentSeconds    int     `json:"timeSpentSeconds"`
		Status              string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	progressStatus := models.ProgressStatus(input.Status)
	if progressStatus == "" {
		if input.Completed {
			progressStatus = models.ProgressStatusCompleted
		} else {
			progressStatus = models.ProgressStatusInProgress
		}
	}

	progress := models.LessonProgress{
		UserID:              userId,
		LessonID:            lessonId,
		Completed:           input.Completed,
		LastWatchedPosition: int(input.LastWatchedPosition),
		TimeSpentSeconds:    input.TimeSpentSeconds,
		Status:              progressStatus,
	}

	// Write to database using WriteDB for CQRS write path
	if err := db.WriteDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "sub_topic_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"completed":             input.Completed,
			"last_watched_position": input.LastWatchedPosition,
			"time_spent_seconds":    gorm.Expr("time_spent_seconds + ?", input.TimeSpentSeconds),
			"status":                progressStatus,
			"updated_at":            time.Now(),
		}),
	}).Create(&progress).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save lesson progress: "+err.Error())
		return
	}

	// Invalidate progress cache
	ec := getEnhancedCache()
	ctx := c.Request.Context()
	// We need to find the subject ID for this lesson to invalidate properly
	// For now, invalidate user progress cache pattern
	ec.InvalidatePattern(ctx, "prog:subject:"+userId+":*")

	api_response.Success(c, nil)
}


