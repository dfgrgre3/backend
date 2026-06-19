package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/cqrs/commands"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/events"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"
	"thanawy-backend/internal/services"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	msgSubjectNotFound      = "Subject not found"
	preloadTopicsSubTopics  = "Topics.SubTopics"
	msgUserNotAuthenticated = "User not authenticated"
	msgInvalidInput         = "Invalid input"
	subjectIDQuery          = "subject_id = ?"
	subjectIDQuotedQuery    = "subject_id = ?"
)

var (
	subjectRepo     *repository.SubjectRepository
	subjectRepoOnce sync.Once
)

func getSubjectRepo() *repository.SubjectRepository {
	subjectRepoOnce.Do(func() {
		subjectRepo = repository.NewSubjectRepository(db.DB)
	})
	return subjectRepo
}

// buildSubjectFilters applies common filter conditions to a query and returns it.
func buildSubjectFilters(query *gorm.DB, c *gin.Context) *gorm.DB {
	if catID := c.Query("categoryId"); catID != "" {
		query = query.Where("category_id = ?", catID)
	}

	search := c.Query("search")
	if search != "" {
		query = query.Where("name ILIKE ? OR name_ar ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if level := c.Query("level"); level != "" {
		query = query.Where("level = ?", level)
	}
	if isPublished := c.Query("isPublished"); isPublished != "" {
		query = query.Where("is_published = ?", isPublished == "true")
	}
	if isActive := c.Query("isActive"); isActive != "" {
		query = query.Where(isActiveQuery, isActive == "true")
	}
	return query
}

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

	if db.Redis != nil {
		cached, err := db.Redis.Get(c.Request.Context(), cacheKey).Result()
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

	type countResult struct {
		SubjectID string
		Count     int64
	}
	var topicCounts []countResult
	if len(subjectIDs) > 0 {
		db.ReadDB(c.Request.Context()).Table("Topic").
			Select("subject_id, count(*) as count").
			Where("subject_id IN ?", subjectIDs).
			Group("subject_id").
			Scan(&topicCounts)
	}

	topicCountMap := make(map[string]int64)
	for _, c := range topicCounts {
		topicCountMap[c.SubjectID] = c.Count
	}

	// Format response for frontend
	items := make([]gin.H, 0, len(subjects))
	for _, subject := range subjects {
		items = append(items, gin.H{
			"id":                     subject.ID,
			"name":                   subject.Name,
			"nameAr":                 subject.NameAr,
			"code":                   subject.Code,
			"description":            subject.Description,
			"icon":                   subject.Icon,
			"color":                  subject.Color,
			"type":                   "COURSE",
			"isActive":               subject.IsActive,
			"isPublished":            subject.IsPublished,
			"price":                  subject.Price,
			"level":                  subject.Level,
			"instructorName":         subject.InstructorName,
			"instructorId":           subject.InstructorId,
			"categoryId":             subject.CategoryId,
			"thumbnailUrl":           subject.ThumbnailUrl,
			"trailerUrl":             subject.TrailerUrl,
			"trailerDurationMinutes": subject.TrailerDurationMinutes,
			"durationHours":          subject.DurationHours,
			"requirements":           subject.Requirements,
			"learningObjectives":     subject.LearningObjectives,
			"seoTitle":               subject.SeoTitle,
			"seoDescription":         subject.SeoDescription,
			"slug":                   subject.Slug,
			"rating":                 subject.Rating,
			"enrolledCount":          subject.EnrolledCount,
			"createdAt":              subject.CreatedAt,
			"updatedAt":              subject.UpdatedAt,
			"_count": gin.H{
				"enrollments": subject.EnrolledCount,
				"topics":      topicCountMap[subject.ID],
				"reviews":     0,
				"teachers":    0,
			},
		})
	}

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

	if db.Redis != nil {
		if data, err := json.Marshal(responsePayload); err == nil {
			db.Redis.Set(c.Request.Context(), cacheKey, data, 15*time.Minute)
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

	// Wrap for frontend
	api_response.Success(c, gin.H{
		"subject": subject,
		"data": gin.H{
			"subject": subject,
			"course":  subject,
		},
	})
}

func GetCourseLessons(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	id := c.Param("id")
	var subject models.Subject

	query := database.Preload(preloadTopicsSubTopics)
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "fetching course lessons")
		return
	}

	// Simplified lesson structure for frontend
	type Lesson struct {
		ID              string `json:"id"`
		Title           string `json:"title"`
		Description     string `json:"description"`
		VideoUrl        string `json:"videoUrl"`
		IsFree          bool   `json:"isFree"`
		Order           int    `json:"order"`
		DurationMinutes int    `json:"durationMinutes"`
	}

	var lessons []Lesson
	for _, topic := range subject.Topics {
		for _, st := range topic.SubTopics {
			lessons = append(lessons, Lesson{
				ID:              st.ID,
				Title:           st.Title,
				Description:     stringOrEmpty(st.Description),
				VideoUrl:        stringOrEmpty(st.VideoUrl),
				IsFree:          st.IsFree,
				Order:           st.Order,
				DurationMinutes: st.DurationMinutes,
			})
		}
	}

	api_response.Success(c, gin.H{"lessons": lessons})
}

// applyIDOrSlugQuery applies where clause based on whether id is a UUID or a slug
func applyIDOrSlugQuery(query *gorm.DB, id string) *gorm.DB {
	if len(id) == 36 && strings.Contains(id, "-") {
		return query.Where(idQuery, id)
	}
	return query.Where("slug = ? OR id = ?", id, id)
}

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
	if subject.Price > 0 {
		if !hasPaidForSubject(userId, courseId) {
			api_response.Success(c, gin.H{
				"error":           "Payment required for this course",
				"courseId":        courseId,
				"price":           subject.Price,
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

	api_response.Success(c, gin.H{"success": true, "message": "Enrolled successfully"})
}

func getAuthenticatedUserID(c *gin.Context) (string, bool) {
	userIdValue, exists := c.Get("userId")
	if !exists || userIdValue == nil {
		api_response.Error(c, http.StatusUnauthorized, msgUserNotAuthenticated)
		return "", false
	}
	userId, ok := userIdValue.(string)
	if !ok {
		api_response.Error(c, http.StatusInternalServerError, "Invalid user ID type")
		return "", false
	}
	return userId, true
}

func handleSubjectError(c *gin.Context, id string, err error, contextMsg string) {
	if err == gorm.ErrRecordNotFound {
		api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
	} else {
		log.Printf("Error %s %q: %v", contextMsg, id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to "+contextMsg)
	}
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

func CourseCheckout(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	courseId := c.Param("id")

	var input struct {
		PaymentMethod string `json:"paymentMethod" binding:"required"`
		CouponCode    string `json:"couponCode"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "fetching course for checkout")
		return
	}

	if input.PaymentMethod == "internal_wallet" {
		processInternalWalletPayment(c, userId, courseId, subject)
		return
	}

	if input.PaymentMethod == "card" || input.PaymentMethod == "wallet" || input.PaymentMethod == "fawry" {
		processPaymobPayment(c, userId, courseId, subject, input.PaymentMethod)
		return
	}

	api_response.Error(c, http.StatusBadRequest, "Unsupported payment method")
}

func processInternalWalletPayment(c *gin.Context, userId string, courseId string, subject models.Subject) {
	payment := models.Payment{
		UserID:    userId,
		SubjectID: &courseId,
		Amount:    subject.Price,
		Method:    "internal_wallet",
		Status:    models.PaymentPending,
		Reference: generateSecureReference("COURSE"),
	}

	txErr := db.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, idQuery, userId).Error; err != nil {
			return err
		}

		if user.Balance < subject.Price {
			return gorm.ErrInvalidData // Insufficient balance
		}

		if err := tx.Model(&user).Update("balance", gorm.Expr("balance - ?", subject.Price)).Error; err != nil {
			return err
		}

		payment.Status = models.PaymentCompleted
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}

		return enrollUserInTransaction(tx, userId, courseId)
	})

	if txErr != nil {
		api_response.Error(c, http.StatusBadRequest, "رصيدك غير كافٍ")
		return
	}

	api_response.Success(c, gin.H{
		"success": true,
		"message": "Payment successful and enrolled",
	})
}

func processPaymobPayment(c *gin.Context, userId string, courseId string, subject models.Subject, method string) {
	paymobSvc := services.NewPaymobService()

	token, err := paymobSvc.Authenticate()
	if err != nil {
		log.Printf("Paymob Auth Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "فشل الاتصال ببوابة الدفع")
		return
	}

	amountCents := int64(subject.Price * 100)
	orderID, err := paymobSvc.RegisterOrder(token, amountCents, []interface{}{
		map[string]interface{}{
			"name":         subject.Name,
			"amount_cents": amountCents,
			"description":  fmt.Sprintf("Course: %s", subject.Name),
			"quantity":     1,
		},
	})
	if err != nil {
		log.Printf("Paymob Order Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "فشل إنشاء طلب الدفع")
		return
	}

	var user models.User
	db.DB.First(&user, idQuery, userId)
	billingData := getBillingData(user)

	integrationID := getIntegrationID(paymobSvc, method)

	paymentKey, err := paymobSvc.GetPaymentKey(token, orderID, amountCents, integrationID, billingData)
	if err != nil {
		log.Printf("Paymob Key Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "فشل استخراج مفتاح الدفع")
		return
	}

	if err := createPendingPayment(userId, courseId, subject.Price, method, orderID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save payment record")
		return
	}

	if method == "wallet" {
		handleWalletRedirect(c, paymobSvc, paymentKey, billingData["phone_number"], orderID)
		return
	}

	api_response.Success(c, gin.H{
		"paymentKey": paymentKey,
		"iframeId":   paymobSvc.IframeID,
		"orderId":    orderID,
	})
}

func getBillingData(user models.User) map[string]string {
	billingData := map[string]string{
		"first_name":   "Student",
		"last_name":    "User",
		"email":        user.Email,
		"phone_number": "01000000000",
	}
	if user.Name != nil && *user.Name != "" {
		billingData["first_name"] = *user.Name
	}
	if user.Phone != nil && *user.Phone != "" {
		billingData["phone_number"] = *user.Phone
	}
	return billingData
}

func getIntegrationID(svc *services.PaymobService, method string) string {
	switch method {
	case "wallet":
		return svc.WalletIntegrationID
	case "fawry":
		return svc.FawryIntegrationID
	default:
		return svc.CardIntegrationID
	}
}

func createPendingPayment(userId, courseId string, amount float64, method string, orderID int64) error {
	payment := models.Payment{
		UserID:        userId,
		SubjectID:     &courseId,
		Amount:        amount,
		Method:        method,
		Status:        models.PaymentPending,
		Reference:     generateSecureReference("COURSE"),
		PaymobOrderID: orderID,
	}
	return SafeCreate(db.DB, &payment)
}

func handleWalletRedirect(c *gin.Context, svc *services.PaymobService, paymentKey, phone string, orderID int64) {
	walletUrl, err := svc.CreateWalletRequest(paymentKey, phone)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل معالجة طلب المحفظة")
		return
	}
	api_response.Success(c, gin.H{
		"redirectUrl": walletUrl,
		"orderId":     orderID,
	})
}

func UpdateLessonProgress(c *gin.Context) {
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

	api_response.Success(c, nil)
}

// Admin handlers
func CreateSubject(c *gin.Context) {
	var subject models.Subject
	if err := c.ShouldBindJSON(&subject); err != nil {
		log.Printf("CreateSubject: JSON Binding error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid input format: "+err.Error())
		return
	}

	normalizeSubjectFields(&subject)

	log.Printf("Attempting to create subject: Name=%q, Code=%v, Slug=%v", subject.Name, subject.Code, subject.Slug)
	if err := getSubjectRepo().Create(&subject); err != nil {
		log.Printf("CreateSubject: Repository error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, getCreateSubjectErrorMessage(err))
		return
	}

	LogAudit(c, "CREATE", "subject", subject.ID, subject)
	api_response.Created(c, gin.H{"course": subject})
}

func normalizeSubjectFields(s *models.Subject) {
	if s.Code != nil && *s.Code == "" {
		s.Code = nil
	}
	if s.Slug != nil && *s.Slug == "" {
		s.Slug = nil
	}
	if s.InstructorId != nil && *s.InstructorId == "" {
		s.InstructorId = nil
	}
	if s.CategoryId != nil && *s.CategoryId == "" {
		s.CategoryId = nil
	}
}

func getCreateSubjectErrorMessage(err error) string {
	if !strings.Contains(err.Error(), "duplicate key") {
		return "Failed to create subject"
	}
	if strings.Contains(err.Error(), "Subject_name_key") {
		return "A course with this name already exists"
	}
	if strings.Contains(err.Error(), "Subject_code_key") {
		return "A course with this code already exists"
	}
	if strings.Contains(err.Error(), "Subject_slug_key") {
		return "A course with this slug already exists"
	}
	return "A duplicate entry was found"
}

func UpdateSubject(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("UpdateSubject: JSON Binding error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid input format: "+err.Error())
		return
	}

	id, ok := input["id"].(string)
	if !ok || id == "" {
		api_response.Error(c, http.StatusBadRequest, "Subject ID is required")
		return
	}

	var subject models.Subject
	if err := db.DB.First(&subject, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
		return
	}

	normalizeInputMap(input)
	updates, err := mapInputToSubjectUpdatesMap(input)
	if err != nil {
		log.Printf("UpdateSubject: mapping error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid array input format: "+err.Error())
		return
	}

	if err := db.DB.Model(&models.Subject{}).Where(idQuery, subject.ID).
		Updates(updates).Error; err != nil {
		log.Printf("UpdateSubject: Database error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, getUpdateSubjectErrorMessage(err))
		return
	}

	// Refresh from DB to get all fields
	db.DB.First(&subject, idQuery, id)
	getSubjectRepo().Update(&subject) // Update cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	LogAudit(c, "UPDATE", "subject", id, input)
	api_response.Success(c, gin.H{"course": subject})
}

func mapInputToSubjectUpdatesMap(input map[string]interface{}) (map[string]interface{}, error) {
	updates := make(map[string]interface{})

	// Field mapping from input key (camelCase) to db column (snake_case)
	mappings := map[string]string{
		"name":           "name",
		"nameAr":         "name_ar",
		"description":    "description",
		"categoryId":     "category_id",
		"color":          "color",
		"image":          "image",
		"code":           "code",
		"icon":           "icon",
		"instructorName": "instructor_name",
		"instructorId":   "instructor_id",
		"slug":           "slug",
		"thumbnailUrl":   "thumbnail_url",
		"trailerUrl":     "trailer_url",
		"seoTitle":       "seo_title",
		"seoDescription": "seo_description",
		"level":          "level",
		"language":       "language",
		"type":           "type",
	}

	for inputKey, dbCol := range mappings {
		if val, exists := input[inputKey]; exists {
			updates[dbCol] = val
		}
	}

	// Boolean & numeric type-safe conversions or direct mappings
	numMappings := map[string]string{
		"price":                  "price",
		"durationHours":          "duration_hours",
		"trailerDurationMinutes": "trailer_duration_minutes",
	}
	for inputKey, dbCol := range numMappings {
		if val, exists := input[inputKey]; exists {
			updates[dbCol] = val
		}
	}

	boolMappings := map[string]string{
		"isActive":    "is_active",
		"isPublished": "is_published",
		"isFree":      "is_free",
		"isFeatured":  "is_featured",
	}
	for inputKey, dbCol := range boolMappings {
		if val, exists := input[inputKey]; exists {
			updates[dbCol] = val
		}
	}

	// PGStringArray fields
	arrayFields := map[string]string{
		"coursePrerequisites": "course_prerequisites",
		"targetAudience":      "target_audience",
		"whatYouLearn":        "what_you_learn",
	}
	for inputKey, dbCol := range arrayFields {
		if val, exists := input[inputKey]; exists {
			sa, err := parseStringArray(val)
			if err != nil {
				return nil, err
			}
			if sa != nil {
				updates[dbCol] = *sa
			}
		}
	}

	return updates, nil
}

func parseStringArray(val interface{}) (*models.PGStringArray, error) {
	if val == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	var sa models.PGStringArray
	if err := json.Unmarshal(bytes, &sa); err != nil {
		return nil, err
	}
	return &sa, nil
}

func normalizeInputMap(input map[string]interface{}) {
	pointerFields := []string{"code", "slug", "instructorId", "categoryId", "thumbnailUrl", "trailerUrl", "nameAr", "description", "icon", "instructorName", "seoTitle", "seoDescription"}
	for _, field := range pointerFields {
		if val, exists := input[field]; exists {
			if str, ok := val.(string); ok && str == "" {
				input[field] = nil
			}
		}
	}
}

func getUpdateSubjectErrorMessage(err error) string {
	if strings.Contains(err.Error(), "duplicate key") {
		return "A duplicate entry was found (name, code, or slug already exists)"
	}
	return "Failed to update subject"
}

func DeleteSubject(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		var input struct {
			ID string `json:"id"`
		}
		_ = c.ShouldBindJSON(&input)
		id = input.ID
	}

	log.Printf("Attempting to delete subject with ID: %q", id)

	// First, check if subject exists
	var subject models.Subject
	if err := db.DB.First(&subject, idQuery, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
			return
		}
		log.Printf("Error checking subject existence: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to verify subject")
		return
	}

	// Delete in transaction to ensure atomicity
	tx := db.DB.Begin()
	defer tx.Rollback()

	// Delete related records that don't have CASCADE constraints
	// Delete StudySessions referencing this subject
	if err := tx.Where(subjectIDQuery, id).Delete(&models.StudySession{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting study sessions for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete related study sessions")
		return
	}

	// Delete Books referencing this subject
	if err := tx.Where(subjectIDQuery, id).Delete(&models.Book{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting books for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete related books")
		return
	}

	// Delete Challenges referencing this subject
	if err := tx.Where(subjectIDQuery, id).Delete(&models.Challenge{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting challenges for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete related challenges")
		return
	}

	// Delete Payments referencing this subject (set to null instead of delete)
	if err := tx.Model(&models.Payment{}).Where(subjectIDQuery, id).Update("subject_id", nil).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating payments for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to update related payments")
		return
	}

	// Now delete the subject (Topics, Enrollments will be cascade deleted)
	if err := tx.Delete(&subject).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete subject: "+err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction for subject %q deletion: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to complete deletion")
		return
	}

	// Clear cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	LogAudit(c, "DELETE", "subject", id, nil)
	log.Printf("Successfully deleted subject: %q (%q)", id, subject.Name)
	api_response.Success(c, gin.H{"message": "Subject deleted successfully"})
}

func GetSubjectCurriculum(c *gin.Context) {
	id := c.Param("id")
	var subject models.Subject

	query := db.DB.Preload(preloadTopicsSubTopics)
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "fetching curriculum for subject")
		return
	}

	// Calculate stats
	chaptersCount := len(subject.Topics)
	lessonsCount := 0
	freeLessonsCount := 0
	totalDuration := 0

	for _, topic := range subject.Topics {
		lessonsCount += len(topic.SubTopics)
		for _, subtopic := range topic.SubTopics {
			if subtopic.IsFree {
				freeLessonsCount++
			}
			totalDuration += subtopic.DurationMinutes
		}
	}

	api_response.Success(c, gin.H{
		"stats": gin.H{
			"chaptersCount":        chaptersCount,
			"lessonsCount":         lessonsCount,
			"freeLessonsCount":     freeLessonsCount,
			"totalDurationMinutes": totalDuration,
		},
		"topics": subject.Topics,
	})
}

func GetUserSubjects(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var enrollments []models.Enrollment
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", userId).
		Limit(limit).
		Offset(offset).
		Find(&enrollments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch enrollments"})
		return
	}

	// For the frontend useTimeData.ts which expects { id, subject: "MATH" }
	type subjectResponse struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
	}

	response := []subjectResponse{}
	for _, e := range enrollments {
		if e.Subject.ID != "" {
			response = append(response, subjectResponse{
				ID:      e.ID,
				Subject: e.Subject.Name,
			})
		}
	}

	api_response.Success(c, response)
}

func GetMyCourses(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

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

	api_response.Success(c, gin.H{
		"courses": courses,
		"data": gin.H{
			"courses": courses,
		},
	})
}

func UpdateCourseCurriculum(c *gin.Context) {
	id := c.Param("id")
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	chaptersRaw, err := extractChaptersRaw(raw)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var chapters []incomingChapter
	if err := json.Unmarshal(chaptersRaw, &chapters); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid curriculum format: "+err.Error())
		return
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		clearSubjectCurriculum(tx, id)
		for i, chapter := range chapters {
			if err := createTopicFromIncoming(tx, id, chapter, i); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save curriculum: "+err.Error())
		return
	}

	getSubjectRepo().InvalidateSubjectCache(id)
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	var subject models.Subject
	if err := db.DB.Preload(preloadTopicsSubTopics).First(&subject, idQuery, id).Error; err != nil {
		api_response.Success(c, gin.H{"success": true, "message": "Curriculum updated"})
		return
	}

	api_response.Success(c, gin.H{"curriculum": subject.Topics})
}

type incomingAttachment struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	FileUrl  string  `json:"fileUrl"`
	FileType *string `json:"fileType"`
	FileSize *int64  `json:"fileSize"`
}

type incomingLesson struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Title       string               `json:"title"`
	Order       int                  `json:"order"`
	Type        string               `json:"type"`
	VideoUrl    *string              `json:"videoUrl"`
	Duration    int                  `json:"duration"`
	DurationMin int                  `json:"durationMinutes"`
	IsFree      bool                 `json:"isFree"`
	Description *string              `json:"description"`
	Attachments []incomingAttachment `json:"attachments"`
}

type incomingChapter struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Title     string           `json:"title"`
	Order     int              `json:"order"`
	SubTopics []incomingLesson `json:"subTopics"`
}

func extractChaptersRaw(raw map[string]json.RawMessage) (json.RawMessage, error) {
	if v, ok := raw["curriculum"]; ok {
		return v, nil
	}
	if v, ok := raw["topics"]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("Missing curriculum or topics field")
}

func clearSubjectCurriculum(tx *gorm.DB, subjectId string) {
	var existingTopics []models.Topic
	if err := tx.Where(subjectIDQuotedQuery, subjectId).Find(&existingTopics).Error; err != nil {
		return
	}

	var topicIDs []string
	for _, t := range existingTopics {
		topicIDs = append(topicIDs, t.ID)
	}

	if len(topicIDs) > 0 {
		var existingSubTopics []models.SubTopic
		if err := tx.Where("topic_id IN ?", topicIDs).Find(&existingSubTopics).Error; err == nil {
			var subTopicIDs []string
			for _, st := range existingSubTopics {
				subTopicIDs = append(subTopicIDs, st.ID)
			}
			if len(subTopicIDs) > 0 {
				tx.Unscoped().Where("sub_topic_id IN ?", subTopicIDs).Delete(&models.LessonAttachment{})
			}
		}
		tx.Unscoped().Where("topic_id IN ?", topicIDs).Delete(&models.SubTopic{})
	}
	tx.Unscoped().Where(subjectIDQuotedQuery, subjectId).Delete(&models.Topic{})
}

func createTopicFromIncoming(tx *gorm.DB, subjectId string, chapter incomingChapter, order int) error {
	title := chapter.Name
	if title == "" {
		title = chapter.Title
	}
	topic := models.Topic{
		SubjectID: subjectId,
		Title:     title,
		Order:     order,
	}
	if chapter.ID != "" && !strings.HasPrefix(chapter.ID, "new-") {
		topic.ID = chapter.ID
	}

	if err := tx.Create(&topic).Error; err != nil {
		return err
	}

	for j, lesson := range chapter.SubTopics {
		if err := createSubTopicFromIncoming(tx, topic.ID, lesson, j); err != nil {
			return err
		}
	}
	return nil
}

func createSubTopicFromIncoming(tx *gorm.DB, topicId string, lesson incomingLesson, order int) error {
	title := lesson.Name
	if title == "" {
		title = lesson.Title
	}
	duration := lesson.Duration
	if duration == 0 {
		duration = lesson.DurationMin
	}
	lessonType := models.SubTopicVideo
	if lesson.Type != "" {
		lessonType = models.SubTopicType(lesson.Type)
	}

	st := models.SubTopic{
		TopicID:         topicId,
		Title:           title,
		Order:           order,
		Type:            lessonType,
		VideoUrl:        lesson.VideoUrl,
		DurationMinutes: duration,
		IsFree:          lesson.IsFree,
		Description:     lesson.Description,
	}
	if lesson.ID != "" && !strings.HasPrefix(lesson.ID, "new-") {
		st.ID = lesson.ID
	}

	if err := tx.Create(&st).Error; err != nil {
		return err
	}

	for _, att := range lesson.Attachments {
		newAtt := models.LessonAttachment{
			SubTopicID: st.ID,
			Title:      att.Title,
			FileUrl:    att.FileUrl,
		}
		if att.FileType != nil {
			newAtt.FileType = *att.FileType
		}
		if att.FileSize != nil {
			newAtt.FileSize = *att.FileSize
		}
		if att.ID != "" && !strings.HasPrefix(att.ID, "new-") {
			newAtt.ID = att.ID
		}
		if err := tx.Create(&newAtt).Error; err != nil {
			return err
		}
	}

	return nil
}

func AddLessonAttachment(c *gin.Context) {
	lessonId := c.Param("id")
	var attachment models.LessonAttachment
	if err := c.ShouldBindJSON(&attachment); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	attachment.SubTopicID = lessonId
	if err := SafeCreate(db.DB, &attachment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add attachment")
		return
	}

	// Invalidate parent subject cache
	var subTopic models.SubTopic
	if err := db.DB.First(&subTopic, idQuery, lessonId).Error; err == nil {
		var topic models.Topic
		if err := db.DB.First(&topic, idQuery, subTopic.TopicID).Error; err == nil && topic.SubjectID != "" {
			getSubjectRepo().InvalidateSubjectCache(topic.SubjectID)
			cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), topic.SubjectID)
		}
	}

	api_response.Created(c, attachment)
}

func CreateCourseReview(c *gin.Context) {
	userId, _ := c.Get("userId")
	subjectId := c.Param("id")

	var subject models.Subject
	query := db.DB.Select("id")
	query = applyIDOrSlugQuery(query, subjectId)
	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, subjectId, err, "resolving subject for review creation")
		return
	}

	var review models.CourseReview
	if err := c.ShouldBindJSON(&review); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgInvalidInput})
		return
	}

	review.UserID = userId.(string)
	review.SubjectID = subject.ID

	if err := SafeCreate(db.DB, &review); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create review"})
		return
	}

	// Update subject rating (simplified calculation)
	var avg float64
	db.DB.Model(&models.CourseReview{}).Where(subjectIDQuotedQuery, subject.ID).Select("avg(rating)").Scan(&avg)
	db.DB.Model(&models.Subject{}).Where(idQuery, subject.ID).Update("rating", avg)

	// Award 10 XP points for submitting a course review
	xpAmount := 10
	gamificationService := commands.NewGamificationCommandService()
	_ = gamificationService.AwardXP(commands.AwardXPCommand{
		UserID:   review.UserID,
		XPType:   "quest",
		XPAmount: xpAmount,
		Source:   "course_review",
		SourceID: review.ID,
	})

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subject.ID)

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"review":    review,
			"xpAwarded": xpAmount,
		},
	})
}

func GetCourseReviews(c *gin.Context) {
	id := c.Param("id")
	var reviews []models.CourseReview

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var subject models.Subject
	query := db.DB.Select("id").WithContext(c.Request.Context())
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "resolving subject for reviews")
		return
	}

	if err := db.DB.WithContext(c.Request.Context()).
		Preload("User").
		Where("subject_id = ? AND is_visible = ?", subject.ID, true).
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reviews"})
		return
	}

	c.JSON(http.StatusOK, reviews)
}

// DuplicateCourse duplicates an existing course (Subject) along with its topics, subtopics, and attachments
func DuplicateCourse(c *gin.Context) {
	var input struct {
		CourseID string `json:"courseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	var oldSubject models.Subject
	if err := db.DB.Preload("Topics.SubTopics.Attachments").First(&oldSubject, "id = ?", input.CourseID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	// Generate unique Code and Slug
	uniqueSuffix := fmt.Sprintf("-%d", time.Now().Unix()%10000)
	var newCode *string
	if oldSubject.Code != nil {
		codeVal := *oldSubject.Code + uniqueSuffix
		newCode = &codeVal
	}
	var newSlug *string
	if oldSubject.Slug != nil {
		slugVal := *oldSubject.Slug + uniqueSuffix
		newSlug = &slugVal
	}

	nameCopy := oldSubject.Name + " (Copy)"
	var nameArCopy *string
	if oldSubject.NameAr != nil {
		valAr := *oldSubject.NameAr + " (نسخة)"
		nameArCopy = &valAr
	}

	newSubject := models.Subject{
		Name:                   nameCopy,
		NameAr:                 nameArCopy,
		Code:                   newCode,
		Slug:                   newSlug,
		Description:            oldSubject.Description,
		Icon:                   oldSubject.Icon,
		Color:                  oldSubject.Color,
		IsActive:               true,
		IsPublished:            false,
		Price:                  oldSubject.Price,
		Level:                  oldSubject.Level,
		InstructorName:         oldSubject.InstructorName,
		InstructorId:           oldSubject.InstructorId,
		CategoryId:             oldSubject.CategoryId,
		DurationHours:          oldSubject.DurationHours,
		Requirements:           oldSubject.Requirements,
		LearningObjectives:     oldSubject.LearningObjectives,
		ThumbnailUrl:           oldSubject.ThumbnailUrl,
		TrailerUrl:             oldSubject.TrailerUrl,
		TrailerDurationMinutes: oldSubject.TrailerDurationMinutes,
		SeoTitle:               oldSubject.SeoTitle,
		SeoDescription:         oldSubject.SeoDescription,
		Language:               oldSubject.Language,
		Type:                   oldSubject.Type,
		CoursePrerequisites:    oldSubject.CoursePrerequisites,
		TargetAudience:         oldSubject.TargetAudience,
		WhatYouLearn:           oldSubject.WhatYouLearn,
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newSubject).Error; err != nil {
			return err
		}

		for _, topic := range oldSubject.Topics {
			newTopic := models.Topic{
				SubjectID:   newSubject.ID,
				Title:       topic.Title,
				Description: topic.Description,
				Order:       topic.Order,
			}
			if err := tx.Create(&newTopic).Error; err != nil {
				return err
			}

			for _, subTopic := range topic.SubTopics {
				newSubTopic := models.SubTopic{
					TopicID:         newTopic.ID,
					Title:           subTopic.Title,
					Description:     subTopic.Description,
					Content:         subTopic.Content,
					VideoUrl:        subTopic.VideoUrl,
					Type:            subTopic.Type,
					ExamID:          subTopic.ExamID,
					Order:           subTopic.Order,
					DurationMinutes: subTopic.DurationMinutes,
					IsFree:          subTopic.IsFree,
				}
				if err := tx.Create(&newSubTopic).Error; err != nil {
					return err
				}

				for _, att := range subTopic.Attachments {
					newAtt := models.LessonAttachment{
						SubTopicID: newSubTopic.ID,
						Title:      att.Title,
						FileUrl:    att.FileUrl,
						FileType:   att.FileType,
						FileSize:   att.FileSize,
					}
					if err := tx.Create(&newAtt).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	}); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to duplicate course: "+err.Error())
		return
	}

	getSubjectRepo().InvalidateSubjectCache(newSubject.ID)
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), newSubject.ID)

	api_response.Success(c, gin.H{
		"message": "Course duplicated successfully",
		"course":  newSubject,
	})
}

// BatchCourseAction performs batch operations (publish, unpublish, activate, deactivate, delete) on multiple courses
func BatchCourseAction(c *gin.Context) {
	var input struct {
		IDs    []string `json:"ids" binding:"required"`
		Action string   `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "IDs and Action are required")
		return
	}

	if len(input.IDs) == 0 {
		api_response.Success(c, gin.H{"message": "No courses selected"})
		return
	}

	var err error
	switch input.Action {
	case "publish":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Update("is_published", true).Error
	case "unpublish":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Update("is_published", false).Error
	case "activate":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Update("is_active", true).Error
	case "deactivate":
		err = db.DB.Model(&models.Subject{}).Where("id IN ?", input.IDs).Update("is_active", false).Error
	case "delete":
		var count int64
		db.DB.Model(&models.Enrollment{}).Where("subject_id IN ?", input.IDs).Count(&count)
		if count > 0 {
			api_response.Error(c, http.StatusBadRequest, "Cannot delete courses with enrolled students")
			return
		}
		err = db.DB.Where("id IN ?", input.IDs).Delete(&models.Subject{}).Error
	default:
		api_response.Error(c, http.StatusBadRequest, "Invalid action")
		return
	}

	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to execute batch action: "+err.Error())
		return
	}

	invalidator := cache.NewCacheInvalidator()
	for _, id := range input.IDs {
		getSubjectRepo().InvalidateSubjectCache(id)
		invalidator.InvalidateSubject(c.Request.Context(), id)
	}

	api_response.Success(c, gin.H{
		"message": "Batch action executed successfully",
	})
}

// GetPopularCourses returns the most popular published courses for the homepage,
// ordered by enrolled_count descending, then by rating descending.
// Limit and offset can be passed via query params; default limit is 8.
// Requires isPublished=true; also filters to isActive=true.
func GetPopularCourses(c *gin.Context) {
	const SubjectCacheTTL = 2 * time.Hour
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "8"))
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	cacheKey := fmt.Sprintf("subject:popular:limit=%d:offset=%d", limit, offset)

	if db.Redis != nil {
		cached, err := db.Redis.Get(c.Request.Context(), cacheKey).Result()
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
	if err := readDB.Model(&models.Subject{}).
		Where("is_published = ? AND is_active = ?", true, true).
		Order("enrolled_count DESC").
		Order("rating DESC").
		Offset(offset).
		Limit(limit).
		Find(&subjects).Error; err != nil {

		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch popular courses")
		return
	}

	subjectIDs := make([]string, len(subjects))
	for i, s := range subjects {
		subjectIDs[i] = s.ID
	}

	type countResult struct {
		SubjectID string
		Count     int64
	}
	var topicCounts []countResult
	if len(subjectIDs) > 0 {
		db.ReadDB(c.Request.Context()).Table("Topic").
			Select("subject_id, count(*) as count").
			Where("subject_id IN ?", subjectIDs).
			Group("subject_id").
			Scan(&topicCounts)
	}

	topicCountMap := make(map[string]int64)
	for _, c := range topicCounts {
		topicCountMap[c.SubjectID] = c.Count
	}

	items := make([]gin.H, 0, len(subjects))
	for _, subject := range subjects {
		items = append(items, gin.H{
			"id":                     subject.ID,
			"name":                   subject.Name,
			"nameAr":                 subject.NameAr,
			"code":                   subject.Code,
			"description":            subject.Description,
			"icon":                   subject.Icon,
			"color":                  subject.Color,
			"type":                   "COURSE",
			"isActive":               subject.IsActive,
			"isPublished":            subject.IsPublished,
			"price":                  subject.Price,
			"level":                  subject.Level,
			"instructorName":         subject.InstructorName,
			"instructorId":           subject.InstructorId,
			"categoryId":             subject.CategoryId,
			"thumbnailUrl":           subject.ThumbnailUrl,
			"trailerUrl":             subject.TrailerUrl,
			"trailerDurationMinutes": subject.TrailerDurationMinutes,
			"slug":                   subject.Slug,
			"rating":                 subject.Rating,
			"enrolledCount":          subject.EnrolledCount,
			"durationHours":          subject.DurationHours,
			"topicCount":             topicCountMap[subject.ID],
		})
	}

	response := gin.H{
		"items": items,
		"total": len(items),
	}
	if db.Redis != nil {
		data, _ := json.Marshal(response)
		go func(key string, payload []byte) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			db.Redis.Set(ctx, key, payload, SubjectCacheTTL)
		}(cacheKey, data)
	}

	api_response.Success(c, response)
}
