package protected

import (
	"log"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
	"thanawy-backend/internal/infrastructure/events"
	pagination "thanawy-backend/internal/shared/utils"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func EnrollCourse(c *gin.Context) {
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

	// Check if user is already enrolled
	if isAlreadyEnrolled(userId, courseId) {
		api_response.Success(c, gin.H{"success": true, "message": "Already enrolled"})
		return
	}

	// Payment verification logic
	price, _ := subject.Price.Float64()
	if price > 0 {
		if !hasPaidForSubject(userId, courseId) {
			api_response.Success(c, gin.H{
				"error":           "Payment required for this course",
				"courseId":        courseId,
				"price":           price,
				"requiresPayment": true,
			})
			return
		}
	}

	if err := executeEnrollmentTransaction(userId, courseId); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to enroll: "+err.Error())
		return
	}

	// Fire enrollment event for gamification/analytics
	fireEnrollmentEvent(c, userId, subject.ID, string(events.EventEnrollment))

	// Invalidate cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subject.ID)
	invalidateUserListCaches(c.Request.Context(), userId)

	api_response.Success(c, gin.H{"success": true, "message": "Enrolled successfully"})
}

func isAlreadyEnrolled(userId, subjectId string) bool {
	var enrollment models.Enrollment
	err := db.DB.Where("user_id = ? AND subject_id = ?", userId, subjectId).First(&enrollment).Error
	return err == nil
}

func hasPaidForSubject(userId, subjectId string) bool {
	var payment models.Payment
	err := db.DB.Where("user_id = ? AND subject_id = ? AND status = ?", userId, subjectId, models.PaymentCompleted).First(&payment).Error
	return err == nil
}

func executeEnrollmentTransaction(userId, subjectId string) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		return enrollUserInTransaction(tx, userId, subjectId)
	})
}

func enrollUserInTransaction(tx *gorm.DB, userId, subjectId string) error {
	enrollment := models.Enrollment{
		UserID:     userId,
		SubjectID:  subjectId,
		EnrolledAt: time.Now(),
	}

	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&enrollment)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return tx.Model(&models.Subject{}).
			Where(idQuery, subjectId).
			Update("enrolled_count", gorm.Expr("enrolled_count + 1")).Error
	}
	return nil
}

// legacySubjectResponse is the original /user/subjects item shape, kept for
// backward compatibility with existing consumers (e.g. useTimeData.ts).
type legacySubjectResponse struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
}

// GetUserSubjects returns the authenticated user's enrolled subjects.
//
// Two response modes:
//   - Legacy (default): plain array of {id, subject} — the original contract,
//     unchanged for existing consumers.
//   - Cursor mode (?v=2): flat CursorPage {data, nextCursor, hasNextPage}
//     for lazy-loading / infinite-scroll frontends.
//
// Both modes are cached per-user via EnhancedCache (L1 LRU -> L2 Redis);
// the authenticated user id is part of the cache key identity, so one user
// can never be served another user's cached page.
func GetUserSubjects(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	if c.Query("v") != "2" {
		getUserSubjectsLegacy(c, userId)
		return
	}

	pg := pagination.ParseFromRequest(c)
	pg.SortField = normalizeListSort(pg.SortField, enrolledSubjectsSortWhitelist, "enrolled_at")

	cacheKey := cache.BuildListCacheKey(cache.ListQueryIdentity{
		Entity: cache.ListEntityUserSubjects,
		UserID: userId,
		Cursor: pg.Cursor,
		Limit:  pg.Limit,
		Sort:   pg.SortField + ":" + pg.SortOrder,
	})

	ec := cache.GetEnhancedCache()
	var cached pagination.CursorPage
	if err := ec.Get(c.Request.Context(), cacheKey, &cached); err == nil {
		api_response.Success(c, cached)
		return
	}

	query := db.DB.WithContext(c.Request.Context()).
		Model(&models.Enrollment{}).
		Preload("Subject").
		Where("user_id = ?", userId)

	paginatedQuery, err := pg.ApplyCursor(query)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid pagination cursor")
		return
	}

	var enrollments []models.Enrollment
	if err := paginatedQuery.Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch enrollments")
		return
	}

	hasNext := len(enrollments) > pg.Limit
	if hasNext {
		enrollments = enrollments[:pg.Limit]
	}
	var nextCursor string
	if hasNext && len(enrollments) > 0 {
		last := enrollments[len(enrollments)-1]
		nextCursor = pagination.EncodeCursor(pagination.CursorData{
			ID:        last.ID,
			Value:     enrollmentCursorValue(last, pg.SortField),
			SortField: pg.SortField,
		})
	}

	items := []legacySubjectResponse{}
	for _, e := range enrollments {
		if e.Subject.ID != "" {
			items = append(items, legacySubjectResponse{ID: e.ID, Subject: e.Subject.Name})
		}
	}

	page := pagination.NewCursorPage(items, nextCursor, hasNext)
	_ = ec.Set(c.Request.Context(), cacheKey, page, cache.TTLUserSubjectsList)

	api_response.Success(c, page)
}

// getUserSubjectsLegacy preserves the original non-paginated contract:
// a plain JSON array of {id, subject} entries (limit/offset supported).
// The only change vs. the original implementation is the per-user page cache;
// the SQL and response shape are untouched.
func getUserSubjectsLegacy(c *gin.Context, userId string) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	ec := cache.GetEnhancedCache()
	cacheKey := cache.BuildListCacheKey(cache.ListQueryIdentity{
		Entity:  cache.ListEntityUserSubjects,
		UserID:  userId,
		Limit:   limit,
		Filters: map[string]string{"offset": strconv.Itoa(offset)},
	})

	var cached []legacySubjectResponse
	if err := ec.Get(c.Request.Context(), cacheKey, &cached); err == nil && cached != nil {
		api_response.Success(c, cached)
		return
	}

	var enrollments []models.Enrollment
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", userId).
		Limit(limit).
		Offset(offset).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch enrollments")
		return
	}

	response := []legacySubjectResponse{}
	for _, e := range enrollments {
		if e.Subject.ID != "" {
			response = append(response, legacySubjectResponse{ID: e.ID, Subject: e.Subject.Name})
		}
	}

	_ = ec.Set(c.Request.Context(), cacheKey, &response, cache.TTLUserSubjectsList)

	api_response.Success(c, response)
}

// GetMyCourses returns the authenticated user's enrolled course cards.
//
// Two response modes:
//   - Legacy (default): {courses: [...], data: {courses: [...]}} — the
//     original contract, unchanged for existing consumers.
//   - Cursor mode (?v=2): flat CursorPage {data, nextCursor, hasNextPage}.
//
// Cached per-user via EnhancedCache; user id is part of the key identity.
func GetMyCourses(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	if c.Query("v") != "2" {
		getMyCoursesLegacy(c, userId)
		return
	}

	pg := pagination.ParseFromRequest(c)
	pg.SortField = normalizeListSort(pg.SortField, myCoursesSortWhitelist, "updated_at")

	cacheKey := cache.BuildListCacheKey(cache.ListQueryIdentity{
		Entity: cache.ListEntityMyCourses,
		UserID: userId,
		Cursor: pg.Cursor,
		Limit:  pg.Limit,
		Sort:   pg.SortField + ":" + pg.SortOrder,
	})

	ec := cache.GetEnhancedCache()
	var cached pagination.CursorPage
	if err := ec.Get(c.Request.Context(), cacheKey, &cached); err == nil {
		api_response.Success(c, cached)
		return
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	query := readDB.WithContext(c.Request.Context()).
		Model(&models.Enrollment{}).
		Preload("Subject").
		Where("user_id = ?", userId)

	paginatedQuery, err := pg.ApplyCursor(query)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid pagination cursor")
		return
	}

	var enrollments []models.Enrollment
	if err := paginatedQuery.Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	hasNext := len(enrollments) > pg.Limit
	if hasNext {
		enrollments = enrollments[:pg.Limit]
	}
	var nextCursor string
	if hasNext && len(enrollments) > 0 {
		last := enrollments[len(enrollments)-1]
		nextCursor = pagination.EncodeCursor(pagination.CursorData{
			ID:        last.ID,
			Value:     enrollmentCursorValue(last, pg.SortField),
			SortField: pg.SortField,
		})
	}

	courses := buildMyCourseCards(enrollments)

	page := pagination.NewCursorPage(courses, nextCursor, hasNext)
	_ = ec.Set(c.Request.Context(), cacheKey, page, cache.TTLMyCoursesList)

	api_response.Success(c, page)
}

// getMyCoursesLegacy preserves the original GetMyCourses contract exactly
// ({courses, data:{courses}}); only the per-user page cache is added.
func getMyCoursesLegacy(c *gin.Context, userId string) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	ec := cache.GetEnhancedCache()
	cacheKey := cache.BuildListCacheKey(cache.ListQueryIdentity{
		Entity: cache.ListEntityMyCourses,
		UserID: userId,
		Limit:  limit,
		Sort:   "updated_at:desc",
	})

	var cached struct {
		Courses []gin.H `json:"courses"`
		Data    struct {
			Courses []gin.H `json:"courses"`
		} `json:"data"`
	}
	if err := ec.Get(c.Request.Context(), cacheKey, &cached); err == nil && cached.Courses != nil {
		api_response.Success(c, gin.H{
			"courses": cached.Courses,
			"data": gin.H{
				"courses": cached.Data.Courses,
			},
		})
		return
	}

	var enrollments []models.Enrollment
	if err := readDB.
		Preload("Subject").
		Where("user_id = ?", userId).
		Order("updated_at DESC").
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	courses := buildMyCourseCards(enrollments)

	payload := struct {
		Courses []gin.H `json:"courses"`
		Data    struct {
			Courses []gin.H `json:"courses"`
		} `json:"data"`
	}{}
	payload.Courses = courses
	payload.Data.Courses = courses

	_ = ec.Set(c.Request.Context(), cacheKey, &payload, cache.TTLMyCoursesList)

	api_response.Success(c, gin.H{
		"courses": courses,
		"data": gin.H{
			"courses": courses,
		},
	})
}

// buildMyCourseCards maps enrollments into the course-card shape consumed by
// the frontend (shared by legacy and cursor modes so both stay consistent).
func buildMyCourseCards(enrollments []models.Enrollment) []gin.H {
	courses := make([]gin.H, 0, len(enrollments))
	for _, enrollment := range enrollments {
		subject := enrollment.Subject
		if subject.ID == "" {
			continue
		}

		title := subject.Name
		if subject.NameAr != nil && *subject.NameAr != "" {
			title = *subject.NameAr
		}

		courses = append(courses, gin.H{
			"id":             subject.ID,
			"enrollmentId":   enrollment.ID,
			"title":          title,
			"name":           subject.Name,
			"nameAr":         subject.NameAr,
			"description":    subject.Description,
			"thumbnailUrl":   subject.ThumbnailUrl,
			"progress":       enrollment.Progress,
			"enrolled":       true,
			"lastAccessedAt": enrollment.UpdatedAt,
			"enrolledAt":     enrollment.EnrolledAt,
			"subject":        subject.Code,
			"rating":         subject.Rating,
			"level":          subject.Level,
		})
	}
	return courses
}
