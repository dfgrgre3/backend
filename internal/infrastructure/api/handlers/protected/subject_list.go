package protected

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	authdto "thanawy-backend/internal/application/dto"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"

	"time"

	"github.com/gin-gonic/gin"
)

var _ = authdto.CourseListResponse{}

// Public handlers
// @Summary List courses
// @Tags courses
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Page size"
// @Param offset query int false "Pagination offset"
// @Param search query string false "Search term"
// @Param level query string false "Course level"
// @Success 200 {object} authdto.CourseListResponse
// @Router /api/v1/courses [get]
func GetSubjects(c *gin.Context) {
	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
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

	// The cache key must cover every filter applied by buildSubjectFilters,
	// otherwise different filter combinations collide on the same entry.
	cacheKey := fmt.Sprintf("subject:list:page=%d:limit=%d:offset=%d:%s",
		page, limit, offset, subjectFilterCacheFragment(c))

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
	query := readDB.Model(&models.Subject{}).Preload("Tags")

	// Apply filters once and reuse
	query = buildSubjectFilters(query, c)

	// Count with same filters
	var total int64
	countQuery := readDB.Model(&models.Subject{})
	countQuery = buildSubjectFilters(countQuery, c)
	countQuery.Count(&total)

	if err := query.Order(subjectSortClause(c.Query("sort"))).Offset(offset).Limit(limit).Find(&subjects).Error; err != nil {
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

	// The admin course editor writes the newer LMS model (LmsCourse), while
	// older public catalog entries still live in Subject. Include published LMS
	// courses here so courses created from /admin/courses are visible in the
	// same public catalog. Draft and under-review courses must remain private.
	var lmsCourses []models.LmsCourse
	lmsQuery := readDB.Model(&models.LmsCourse{}).
		Where("status = ?", models.CourseStatusPublished)
	if search := sanitizeSearchTerm(c.Query("search")); search != "" {
		lmsQuery = lmsQuery.Where("title ILIKE ? OR short_description ILIKE ? OR long_description ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if level := c.Query("level"); isValidLevel(level) {
		lmsQuery = lmsQuery.Where("level = ?", level)
	}
	if c.Query("isPublished") == "false" || c.Query("isActive") == "false" ||
		(c.Query("status") != "" && c.Query("status") != string(models.CourseStatusPublished)) {
		lmsCourses = nil
	} else {
		// Keep the existing Subject pagination stable and add LMS courses to the
		// response. The next catalog request will still receive the full LMS set.
		if err := lmsQuery.Order("created_at DESC").Limit(limit).Find(&lmsCourses).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to fetch LMS courses")
			return
		}
		for _, course := range lmsCourses {
			items = append(items, gin.H{
				"id": course.ID.String(), "name": course.Title, "nameAr": course.Title,
				"description": course.LongDescription, "shortDescription": course.ShortDescription,
				"type": "COURSE", "isActive": true, "isPublished": true,
				"price": 0, "level": course.Level, "language": course.Language,
				"thumbnailUrl": course.CoverImageURL, "slug": course.Slug,
				"instructorId": course.PrimaryInstructorID.String(),
				"isFeatured":   course.IsFeatured, "isTrending": course.IsTrending, "isNew": course.IsNew,
				"createdAt": course.CreatedAt, "updatedAt": course.UpdatedAt,
				"_count": gin.H{"enrollments": 0, "topics": 0, "reviews": 0, "teachers": 0},
			})
		}
	}
	total += int64(len(lmsCourses))

	responsePayload := gin.H{
		"items": items,
		"pagination": api_response.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
		},
		"offset": offset,
	}

	if cache.Redis != nil {
		if data, err := json.Marshal(responsePayload); err == nil {
			cache.Redis.Set(c.Request.Context(), cacheKey, data, 15*time.Minute)
		}
	}

	api_response.Success(c, responsePayload)
}

// @Summary Get course
// @Tags courses
// @Produce json
// @Param id path string true "Course ID or slug"
// @Success 200 {object} authdto.CourseDetailResponse
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/courses/{id} [get]
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

	// Resolve the caller exclusively from the authenticated session (set by
	// middleware.OptionalAuth on the public route). The previously accepted
	// ?userId= override is dropped: this endpoint unredacts paid-lesson
	// content for enrolled users, so trusting a client-supplied id let any
	// anonymous visitor read paid content by guessing another user's id
	// (IDOR/BOLA).
	userID := ""
	if uid, exists := c.Get("userId"); exists {
		if s, ok := uid.(string); ok {
			userID = s
		}
	}

	var enrollment *models.Enrollment
	isEnrolled := false
	if userID != "" {
		var e models.Enrollment
		if err := database.Where("user_id = ? AND subject_id = ?", userID, subject.ID).First(&e).Error; err == nil {
			enrollment = &e
			isEnrolled = true
		}
	}

	// SECURITY: this endpoint is public and unauthenticated. Serializing
	// `subject` directly would include every SubTopic's VideoUrl, AudioUrl,
	// Content, ExternalLinkUrl and Attachments regardless of IsFree — i.e.
	// full paid-lesson content (video links, notes, downloadable files)
	// exposed to any visitor. Redact non-free lessons unless the caller is
	// enrolled, matching the rule GetAvailableLessons already applies
	// elsewhere for this same content.
	if !isEnrolled {
		redactLockedLessonContent(&subject)
	}
	// Duration is a server-owned aggregate of the curriculum, not an
	// independently editable value that can drift from lesson durations.
	totalLessonMinutes := 0
	for _, topic := range subject.Topics {
		for _, lesson := range topic.SubTopics {
			totalLessonMinutes += lesson.DurationMinutes
		}
	}
	if totalLessonMinutes > 0 {
		subject.DurationHours = (totalLessonMinutes + 59) / 60
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

// redactLockedLessonContent strips playable/downloadable content (video,
// audio, external links, attachments, notes) from every non-free lesson in
// subject, in place. Lesson titles, order, duration, and other metadata are
// left intact so the course structure still previews correctly.
func redactLockedLessonContent(subject *models.Subject) {
	for ti := range subject.Topics {
		subTopics := subject.Topics[ti].SubTopics
		for si := range subTopics {
			if subTopics[si].IsFree {
				continue
			}
			subTopics[si].VideoUrl = nil
			subTopics[si].AudioUrl = nil
			subTopics[si].Content = nil
			subTopics[si].ExternalLinkUrl = nil
			subTopics[si].ExternalLinkTitle = nil
			subTopics[si].Attachments = nil
		}
	}
}
