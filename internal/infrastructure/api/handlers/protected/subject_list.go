package protected

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"

	"time"

	"github.com/gin-gonic/gin"
)

// Public handlers
func GetSubjects(c *gin.Context) {
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

	categoryId := c.Query("categoryId")
	search := c.Query("search")
	level := c.Query("level")
	isPublished := c.Query("isPublished")
	isActive := c.Query("isActive")

	cacheKey := fmt.Sprintf("subject:list:page=%d:limit=%d:offset=%d:cat=%s:search=%s:level=%s:pub=%s:act=%s",
		page, limit, offset, categoryId, search, level, isPublished, isActive)

	if cache.Redis != nil {
		cached, err := cache.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var cachedResponse gin.H
			if json.Unmarshal([]byte(cached), &cachedResponse) == nil {
				api_response.Success(c, cachedResponse)
				return
			}
		}
	}

	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}
	var subjects []models.Subject
	query := readDB.Model(&models.Subject{})

	// Apply filters once and reuse
	query = buildSubjectFilters(query, c)

	// Count with same filters
	var total int64
	countQuery := readDB.Model(&models.Subject{})
	countQuery = buildSubjectFilters(countQuery, c)
	countQuery.Count(&total)

	if err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&subjects).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch subjects")
		return
	}

	// Keep list pages light: do not preload full curriculum; only fetch topic counts.
	// Use same readDB for topic counts
	subjectIDs := make([]string, len(subjects))
	for i, s := range subjects {
		subjectIDs[i] = s.ID
	}

	topicCountMap := fetchTopicCounts(c.Request.Context(), subjectIDs)

	// Format response for frontend
	items := buildSubjectListResponse(subjects, topicCountMap)

	responsePayload := gin.H{
		"items": items,
		"pagination": api_response.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
		},
		"subjects": items,
		"courses":  items,
		"offset":   offset,
	}

	if cache.Redis != nil {
		if data, err := json.Marshal(responsePayload); err == nil {
			cache.Redis.Set(c.Request.Context(), cacheKey, data, 15*time.Minute)
		}
	}

	api_response.Success(c, responsePayload)
}

func GetSubject(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	id := c.Param("id")
	var subject models.Subject

	// Support both ID (UUID) and Slug
	query := database.Preload("Topics.SubTopics.Attachments").Preload("Topics.SubTopics.Exam")

	// Check if it's a UUID or Slug
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "fetching subject")
		return
	}

	// Check enrollment if userId is provided in query or context
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
		var e models.Enrollment
		if err := database.Where("user_id = ? AND subject_id = ?", userID, subject.ID).First(&e).Error; err == nil {
			enrollment = &e
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
