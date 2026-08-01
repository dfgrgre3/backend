package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"encoding/csv"
	"encoding/json"
	"strings"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"
	"thanawy-backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"runtime"
	"sync"
)

const coalesceSumDuration = "COALESCE(SUM(duration_min), 0)"

// GetCourseChangelog returns the workflow history for a course.
func GetCourseChangelog(c *gin.Context) {
	history, err := services.NewWorkflowService().GetCourseWorkflowHistory(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course changelog")
		return
	}

	api_response.Success(c, gin.H{"changelog": history})
}

// AdminAI handles all AI-related admin operations
func AdminAIGet(c *gin.Context) {
	var riskStudents []models.User
	db.DB.Where(statusQuery, models.StatusInactive).Limit(5).Find(&riskStudents)

	var subjects []models.Subject
	db.DB.Limit(10).Find(&subjects)

	riskItems := make([]gin.H, 0, len(riskStudents))
	for _, s := range riskStudents {
		daysSinceUpdate := int(time.Since(s.UpdatedAt).Hours() / 24)
		riskScore := 60 + (daysSinceUpdate / 2)
		if riskScore > 98 {
			riskScore = 98
		}

		riskItems = append(riskItems, gin.H{
			"id":         s.ID,
			"name":       firstNonEmpty(stringOrEmpty(s.Name), stringOrEmpty(s.Username), s.Email),
			"riskScore":  riskScore,
			"gradeLevel": s.GradeLevel,
			"reasons":    []string{"انقطاع عن النشاط", fmt.Sprintf("آخر تواجد منذ %d يوم", daysSinceUpdate)},
		})
	}

	subjectItems := make([]gin.H, 0, len(subjects))
	for _, s := range subjects {
		subjectItems = append(subjectItems, gin.H{
			"id":   s.ID,
			"name": s.Name,
		})
	}

	api_response.Success(c, gin.H{
		"riskStudents": riskItems,
		"subjects":     subjectItems,
		"summary": gin.H{
			"highRiskCount": len(riskItems),
		},
	})
}

// AdminResetCircuitBreaker resets all circuit breakers to closed state
func AdminResetCircuitBreaker(c *gin.Context) {
	svc := services.GetCircuitBreakerService()
	status := svc.GetStatus()

	for name := range status {
		svc.ResetCircuitBreaker(name)
	}

	api_response.Success(c, gin.H{
		"message": "All circuit breakers have been reset",
		"status":  svc.GetStatus(),
	})
}

func AdminAIPost(c *gin.Context) {
	var req struct {
		Action      string `json:"action"`
		Prompt      string `json:"prompt"`
		Title       string `json:"title"`
		ContentType string `json:"contentType"`
		SubjectId   string `json:"subjectId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request")
		return
	}

	switch req.Action {
	case "copilot":
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			api_response.Success(c, gin.H{
				"message": "أنا مستشار المملكة الذكي. هذه ميزة تجريبية وسأكون متاحاً قريباً بشكل كامل.",
			})
			return
		}

		// Build admin context
		var totalUsers int64
		var totalSubjects int64
		db.DB.Model(&models.User{}).Count(&totalUsers)
		db.DB.Model(&models.Subject{}).Count(&totalSubjects)

		systemPrompt := fmt.Sprintf(`أنت مستشار ذكي لإدارة منصة "ثانوي" التعليمية.
لديك حالياً %d مستخدم و %d مادة دراسية.
ساعد المدير في اتخاذ القرارات وتحليل البيانات وتقديم الاقتراحات.
أجب بالعربية بشكل مختصر ومفيد.`, totalUsers, totalSubjects)

		messages := []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Prompt},
		}

		// Use the same AI calling pattern as AIChatProxy
		aiHandler := NewAIHandler()
		reply, _, err := aiHandler.callAIWithRetryCustom(messages, "google/gemini-2.0-flash-001")
		if err != nil {
			api_response.Success(c, gin.H{
				"message": "عذراً، حدث خطأ في الاتصال بالذكاء الاصطناعي. يرجى المحاولة مرة أخرى.",
			})
			return
		}

		api_response.Success(c, gin.H{"message": reply})

	case "generate_content":
		systemPrompt := fmt.Sprintf("أنت مستشار ذكي لإنشاء محتوى من نوع %s. قم بإنشاء المحتوى المطلوب.", req.ContentType)
		messages := []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Prompt},
		}

		aiHandler := NewAIHandler()
		reply, _, err := aiHandler.callAIWithRetryCustom(messages, "google/gemini-2.0-flash-001")
		if err != nil {
			api_response.Error(c, http.StatusInternalServerError, "فشل توليد المحتوى، الرجاء المحاولة مرة أخرى.")
			return
		}
		api_response.Success(c, gin.H{"message": reply})

	default:
		api_response.Error(c, http.StatusBadRequest, "Unknown action")
	}
}

func AdminBulkSendMessage(c *gin.Context) {
	var req struct {
		Message   string   `json:"message" binding:"required"`
		Title     string   `json:"title"`
		Type      string   `json:"type"`
		UserIDs   []string `json:"userIds"`
		Role      string   `json:"role"`
		ActionURL string   `json:"actionUrl"`
		Channels  []string `json:"channels"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := validateBulkMessageChannels(req.Channels); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	targetUsers, err := fetchBulkMessageTargetUsers(req.UserIDs, req.Role)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	notificationType := parseNotificationType(req.Type)
	title := req.Title
	if title == "" {
		title = "رسالة من الإدارة"
	}

	notifications := prepareBulkNotifications(targetUsers, title, req.Message, notificationType, req.ActionURL, req.Channels)

	if len(notifications) > 0 {
		if err := db.DB.CreateInBatches(&notifications, 100).Error; err != nil {
			log.Printf("ERROR: Failed to create notifications in bulk: %v", err)
			api_response.Error(c, http.StatusInternalServerError, "Failed to create notifications: "+err.Error())
			return
		}
	}

	sendBulkEmailsBackground(targetUsers, title, req.Message, req.ActionURL, req.Channels)

	LogAudit(c, "BULK_SEND_MESSAGE", "notification", "", gin.H{
		"targetCount": len(targetUsers),
		"channels":    req.Channels,
		"appCount":    len(notifications),
		"queued":      true,
	})

	api_response.Success(c, gin.H{
		"summary": gin.H{
			"success": len(notifications),
			"failure": 0,
			"total":   len(targetUsers),
		},
		"message": fmt.Sprintf("تم إرسال %d رسالة بنجاح", len(notifications)),
	})
	LogAudit(c, "BULK_SEND_MESSAGE", "notification", "", req)
}

func validateBulkMessageChannels(channels []string) error {
	if len(channels) == 0 {
		return nil
	}
	validChannels := map[string]bool{"app": true, "email": true, "sms": true}
	for _, channel := range channels {
		if !validChannels[channel] {
			return fmt.Errorf("Invalid channel: %s", channel)
		}
	}
	return nil
}

func fetchBulkMessageTargetUsers(userIDs []string, role string) ([]models.User, error) {
	var targetUsers []models.User
	query := db.DB.Model(&models.User{})
	if len(userIDs) > 0 {
		query = query.Where(idInQuery, userIDs)
	} else if role != "" {
		query = query.Where(queryRole, role)
	}

	if err := query.Find(&targetUsers).Error; err != nil {
		return nil, fmt.Errorf("Failed to fetch target users")
	}
	return targetUsers, nil
}

func parseNotificationType(t string) models.NotificationType {
	switch t {
	case "success":
		return models.NotificationSuccess
	case "warning":
		return models.NotificationWarning
	case "error":
		return models.NotificationError
	default:
		return models.NotificationInfo
	}
}

func prepareBulkNotifications(users []models.User, title, message string, nType models.NotificationType, actionURL string, channels []string) []models.Notification {
	notifications := make([]models.Notification, 0, len(users))
	shouldAddLink := actionURL != "" && (len(channels) == 0 || contains(channels, "app"))

	for _, u := range users {
		notification := models.Notification{
			UserID:  u.ID,
			Title:   title,
			Message: message,
			Type:    nType,
		}
		if shouldAddLink {
			link := actionURL
			notification.Link = &link
		}
		notifications = append(notifications, notification)
	}
	return notifications
}

func sendBulkEmailsBackground(users []models.User, title, message, url string, channels []string) {
	go func() {
		emailService := services.GetEmailService()
		for _, u := range users {
			if len(channels) == 0 || contains(channels, "email") {
				if emailService.ValidateEmail(u.Email) {
					emailBody := emailService.BuildNotificationEmail(title, message, url)
					_ = emailService.SendEmail(u.Email, title, emailBody, true)
				}
			}
		}
	}()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Upload and PresignUpload are implemented in upload_handler.go

func AdminExamsBulkUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "No file uploaded")
		return
	}

	subjectID := c.PostForm("subjectId")
	examTitle := c.PostForm("title")
	if subjectID == "" || examTitle == "" {
		api_response.Error(c, http.StatusBadRequest, "Subject ID and Exam Title are required")
		return
	}

	f, err := file.Open()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to open file")
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to parse CSV")
		return
	}

	if len(records) < 2 {
		api_response.Error(c, http.StatusBadRequest, "CSV is empty or missing headers")
		return
	}

	exam := models.Exam{
		SubjectID: subjectID,
		Title:     examTitle,
		Type:      models.ExamTypeQuiz,
	}
	if err := SafeCreate(db.DB, &exam); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create exam")
		return
	}

	// Batch create questions
	var questions []models.Question
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 6 {
			continue
		}

		text := row[0]
		options := []string{row[1], row[2], row[3], row[4]}
		answer := row[5]

		optionsJSON, _ := json.Marshal(options)

		questions = append(questions, models.Question{
			ExamID:  exam.ID,
			Text:    text,
			Options: string(optionsJSON),
			Answer:  answer,
		})
	}

	importedCount := 0
	if len(questions) > 0 {
		// Batch insert questions in chunks of 100
		if err := db.DB.CreateInBatches(&questions, 100).Error; err != nil {
			log.Printf("ERROR: Failed to bulk insert questions: %v", err)
			// Fallback: try individual inserts
			for _, q := range questions {
				if err := SafeCreate(db.DB, &q); err == nil {
					importedCount++
				}
			}
		} else {
			importedCount = len(questions)
		}
	}

	api_response.Success(c, gin.H{
		"examId":   exam.ID,
		"imported": importedCount,
		"message":  fmt.Sprintf("تم استيراد %d سؤال بنجاح في اختبار %s", importedCount, examTitle),
	})
	LogAudit(c, "BULK_UPLOAD_EXAM", "exam", exam.ID, gin.H{"importedCount": importedCount})
}

// UploadChunked and all chunk helpers are implemented in upload_handler.go

func MarkActivityRead(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	activityID := c.Param("id")
	if activityID == "" {
		var req struct {
			ActivityID string `json:"activityId"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && req.ActivityID != "" {
			activityID = req.ActivityID
		}
	}

	if activityID == "" {
		api_response.Error(c, http.StatusBadRequest, msgIDRequired)
		return
	}

	if err := db.DB.Model(&models.Notification{}).Where("id = ? AND user_id = ?", activityID, userId).Update("is_read", true).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update activity")
		return
	}

	api_response.Success(c, nil)
}

func MarkAllActivitiesRead(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := db.DB.Model(&models.Notification{}).Where("user_id = ?", userId).Update("is_read", true).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update activities")
		return
	}

	api_response.Success(c, nil)
}

func GetAIRecommendations(c *gin.Context) {
	_, exists := c.Get("userId")
	if !exists {
		api_response.Success(c, gin.H{
			"recommendations": []interface{}{},
			"message":         "Please login to see personalized recommendations",
		})
		return
	}

	api_response.Success(c, gin.H{
		"recommendations": []interface{}{},
		"message":         "AI recommendations not yet implemented",
	})
}

func TrackAIRecommendation(c *gin.Context) {
	var req struct {
		RecommendationID string `json:"recommendationId" binding:"required"`
		Action           string `json:"action"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	api_response.Success(c, gin.H{"success": true})
}

func ValidateCoupon(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var coupon models.Coupon
	if err := db.DB.Where("code = ? AND "+isActiveQuery, req.Code, true).First(&coupon).Error; err != nil {
		api_response.Success(c, gin.H{
			"valid":   false,
			"message": "كود الخصم غير صالح",
		})
		return
	}

	if coupon.ExpiryDate != nil && coupon.ExpiryDate.Before(time.Now()) {
		api_response.Success(c, gin.H{
			"valid":   false,
			"message": "كود الخصم منتهي الصلاحية",
		})
		return
	}

	if coupon.MaxUses != nil && coupon.UsedCount >= *coupon.MaxUses {
		api_response.Success(c, gin.H{
			"valid":   false,
			"message": "تم استنفاذ عدد مرات استخدام الكود",
		})
		return
	}

	api_response.Success(c, gin.H{
		"valid":        true,
		"discountType": coupon.DiscountType,
		"discount":     coupon.DiscountValue,
		"message":      "تم تطبيق الخصم بنجاح",
	})
}

func applyCoupon(couponCode string, price float64) (*models.Coupon, float64) {
	if couponCode == "" {
		return nil, price
	}
	var coupon models.Coupon
	if err := db.DB.Where("code = ? AND "+isActiveQuery, couponCode, true).First(&coupon).Error; err != nil {
		return nil, price
	}

	// Validate coupon
	isExpired := coupon.ExpiryDate != nil && coupon.ExpiryDate.Before(time.Now())
	isMaxUsed := coupon.MaxUses != nil && coupon.UsedCount >= *coupon.MaxUses
	minOrderAmount, _ := coupon.MinOrderAmount.Float64()
	isBelowMin := minOrderAmount > 0 && price < minOrderAmount

	if isExpired || isMaxUsed || isBelowMin {
		return nil, price
	}

	if coupon.DiscountType == "PERCENTAGE" {
		discountValue, _ := coupon.DiscountValue.Float64()
		price = price * (1 - discountValue/100)
	} else {
		discountValue, _ := coupon.DiscountValue.Float64()
		price = price - discountValue
	}

	if price < 0 {
		price = 0
	}
	return &coupon, price
}

func SubscriptionCheckout(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req struct {
		PlanID        string `json:"planId" binding:"required"`
		PaymentMethod string `json:"paymentMethod" binding:"required"`
		CouponCode    string `json:"couponCode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	planPrices := map[string]float64{
		"basic": 150,
		"pro":   350,
		"elite": 600,
	}
	price, ok := planPrices[req.PlanID]
	if !ok {
		api_response.Error(c, http.StatusBadRequest, "Invalid plan ID")
		return
	}

	appliedCoupon, discountedPrice := applyCoupon(req.CouponCode, price)
	price = discountedPrice

	paymobSvc := services.NewPaymobService()
	token, err := paymobSvc.Authenticate()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل الاتصال ببوابة الدفع")
		return
	}

	amountCents := int64(price * 100)
	items := []interface{}{
		map[string]interface{}{
			"name":         fmt.Sprintf("Subscription: %s", req.PlanID),
			"amount_cents": amountCents,
			"description":  fmt.Sprintf("Plan ID: %s", req.PlanID),
			"quantity":     1,
		},
	}

	orderID, err := paymobSvc.RegisterOrder(token, amountCents, items)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل إنشاء طلب الدفع")
		return
	}

	var user models.User
	db.DB.First(&user, queryID, userId)

	billingData := getBillingData(user)
	integrationID := getIntegrationID(paymobSvc, req.PaymentMethod)

	paymentKey, err := paymobSvc.GetPaymentKey(token, orderID, amountCents, integrationID, billingData)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل استخراج مفتاح الدفع")
		return
	}

	payment := models.Payment{
		UserID:        userId,
		Amount:        decimal.NewFromFloat(price),
		Method:        req.PaymentMethod,
		Status:        models.PaymentPending,
		Reference:     fmt.Sprintf("SUB-%d", time.Now().Unix()),
		PaymobOrderID: orderID,
	}
	if err := SafeCreate(db.DB, &payment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save payment record")
		return
	}

	// Increment coupon used count
	if appliedCoupon != nil {
		db.DB.Model(appliedCoupon).UpdateColumn("used_count", appliedCoupon.UsedCount+1)
	}

	if req.PaymentMethod == "wallet" {
		walletUrl, _ := paymobSvc.CreateWalletRequest(paymentKey, billingData["phone_number"])
		api_response.Success(c, gin.H{"redirectUrl": walletUrl, "orderId": orderID})
		return
	}

	api_response.Success(c, gin.H{
		"paymentKey": paymentKey,
		"iframeId":   paymobSvc.IframeID,
		"orderId":    orderID,
	})
}

func handleBookFileUpload(c *gin.Context, fieldName, prefix string) string {
	file, err := c.FormFile(fieldName)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("%s_%d%s", prefix, time.Now().UnixNano(), ext)

	f, err := file.Open()
	if err != nil {
		return ""
	}
	defer f.Close()

	if storage.GlobalStorage == nil {
		return ""
	}

	url, err := storage.GlobalStorage.Upload(c.Request.Context(), filename, f, file.Size, file.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return url
}

func parseBookMultipartForm(c *gin.Context) models.Book {
	var book models.Book
	book.Title = c.PostForm("title")
	book.Author = c.PostForm("author")
	book.Description = c.PostForm("description")
	if subjectId := c.PostForm("subjectId"); subjectId != "" {
		book.SubjectID = &subjectId
	}
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	book.Price = price
	book.IsFree = c.PostForm("isFree") == "true"

	book.CoverUrl = handleBookFileUpload(c, "cover", "book_cover")
	book.DownloadUrl = handleBookFileUpload(c, "file", "book")

	return book
}

func CreateLibraryBook(c *gin.Context) {
	var book models.Book

	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		book = parseBookMultipartForm(c)
	} else if err := c.ShouldBindJSON(&book); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if book.Title == "" {
		api_response.Error(c, http.StatusBadRequest, "Book title is required")
		return
	}

	if err := SafeCreate(db.DB, &book); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create book record")
		return
	}

	api_response.Created(c, book)
}

func GetLibraryCategories(c *gin.Context) {
	var categories []models.Category
	if err := db.DB.Where("type = ?", "LIBRARY").Find(&categories).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch library categories")
		return
	}

	api_response.Success(c, categories)
}

func DeleteImpersonation(c *gin.Context) {
	middleware.ClearImpersonationCookies(c)
	api_response.Success(c, gin.H{
		"message": "تم إنهاء جلسة انتحال الشخصية والعودة لحسابك الأصلي",
	})
}

func GetAdminDashboard(c *gin.Context) {
	timeParam := c.Query("time")
	cacheKey := "admin:dashboard:stats"
	if timeParam != "" {
		cacheKey = fmt.Sprintf("admin:dashboard:stats:%s", timeParam)
	}

	if db.Redis != nil {
		cachedData, err := db.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var cachedResponse map[string]interface{}
			if json.Unmarshal([]byte(cachedData), &cachedResponse) == nil {
				api_response.Success(c, cachedResponse)
				return
			}
		}
	}
	var (
		totalUsers         int64
		totalSubjects      int64
		totalExams         int64
		completedTasks     int64
		newUsersToday      int64
		newUsersThisWeek   int64
		studyMinutes       int64
		examsTaken         int64
		recentTasks        []models.Task
		upcomingExams      []models.Exam
		totalResources     int64
		activeChallenges   int64
		achievementsEarned int64
	)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := now.AddDate(0, 0, -7)

	type dashboardStats struct {
		TotalUsers         int64 `gorm:"column:total_users"`
		NewUsersToday      int64 `gorm:"column:new_users_today"`
		NewUsersThisWeek   int64 `gorm:"column:new_users_this_week"`
		TotalSubjects      int64 `gorm:"column:total_subjects"`
		TotalExams         int64 `gorm:"column:total_exams"`
		CompletedTasks     int64 `gorm:"column:completed_tasks"`
		TotalStudySessions int64 `gorm:"column:total_study_sessions"`
		StudyMinutes       int64 `gorm:"column:study_minutes"`
		ExamsTaken         int64 `gorm:"column:exams_taken"`
		TotalResources     int64 `gorm:"column:total_resources"`
		ActiveChallenges   int64 `gorm:"column:active_challenges"`
		AchievementsEarned int64 `gorm:"column:achievements_earned"`
	}

	// Fetch metrics in parallel for better performance
	var wgMetrics sync.WaitGroup
	wgMetrics.Add(9)

	go func() {
		defer wgMetrics.Done()
		type userCounts struct {
			Total     int64 `gorm:"column:total"`
			Today     int64 `gorm:"column:today"`
			ThisWeek  int64 `gorm:"column:this_week"`
		}
		var uc userCounts
		db.DB.Model(&models.User{}).
			Select("COUNT(*) as total, COUNT(*) FILTER (WHERE created_at >= ?) as today, COUNT(*) FILTER (WHERE created_at >= ?) as this_week", todayStart, weekAgo).
			Scan(&uc)
		totalUsers = uc.Total
		newUsersToday = uc.Today
		newUsersThisWeek = uc.ThisWeek
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.Subject{}).Count(&totalSubjects)
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.Exam{}).Count(&totalExams)
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.Task{}).Where("status = ?", "COMPLETED").Count(&completedTasks)
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.StudySession{}).Select("COALESCE(SUM(duration_min), 0)").Scan(&studyMinutes)
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.ExamResult{}).Count(&examsTaken)
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.SubTopic{}).Where("type != ?", "QUIZ").Count(&totalResources)
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.Challenge{}).Where("is_active = ?", true).Count(&activeChallenges)
	}()
	go func() {
		defer wgMetrics.Done()
		db.DB.Model(&models.UserAchievement{}).Count(&achievementsEarned)
	}()
	wgMetrics.Wait()

	// Fetch lists in parallel
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(10).Find(&recentTasks)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&upcomingExams)
	}()
	wg.Wait()

	// Batch fetch users for recent tasks to avoid N+1 queries
	taskUserIDs := make([]string, 0, len(recentTasks))
	for _, t := range recentTasks {
		taskUserIDs = append(taskUserIDs, t.UserID)
	}
	var taskUsers []models.User
	if len(taskUserIDs) > 0 {
		db.DB.Where(idInQuery, taskUserIDs).Find(&taskUsers)
	}
	userMap := make(map[string]models.User, len(taskUsers))
	for _, u := range taskUsers {
		userMap[u.ID] = u
	}

	recentActivityItems := make([]gin.H, 0, len(recentTasks))
	for _, t := range recentTasks {
		user := userMap[t.UserID]
		recentActivityItems = append(recentActivityItems, gin.H{
			"id":          t.ID,
			"userId":      t.UserID,
			"type":        "task",
			"title":       t.Title,
			"description": stringOrEmpty(t.Description),
			"createdAt":   t.CreatedAt,
			"user": gin.H{
				"name":   firstNonEmpty(stringOrEmpty(user.Name), stringOrEmpty(user.Username), user.Email),
				"avatar": user.Avatar,
			},
		})
	}

	upcomingEvents := make([]gin.H, 0, len(upcomingExams))
	for _, e := range upcomingExams {
		upcomingEvents = append(upcomingEvents, gin.H{
			"id":    e.ID,
			"title": e.Title,
			"date":  e.CreatedAt,
			"type":  "exam",
		})
	}

	sixMonthsAgo := now.AddDate(0, -5, 0)
	startOfSixMonths := time.Date(sixMonthsAgo.Year(), sixMonthsAgo.Month(), 1, 0, 0, 0, 0, sixMonthsAgo.Location())
	sevenDaysAgo := now.AddDate(0, 0, -7)
	startOfSevenDays := time.Date(sevenDaysAgo.Year(), sevenDaysAgo.Month(), sevenDaysAgo.Day(), 0, 0, 0, 0, sevenDaysAgo.Location())

	type monthlyCount struct {
		Month int
		Count int64
	}
	var userGrowthData []monthlyCount
	db.DB.Model(&models.User{}).
		Select("EXTRACT(MONTH FROM created_at) as month, COUNT(*) as count").
		Where("created_at >= ?", startOfSixMonths).
		Group("EXTRACT(MONTH FROM created_at)").
		Scan(&userGrowthData)

	userGrowthMap := make(map[int]int64)
	for _, m := range userGrowthData {
		userGrowthMap[m.Month] = m.Count
	}

	userGrowth := make([]gin.H, 0, 6)
	for i := 5; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		userGrowth = append(userGrowth, gin.H{
			"month": int(d.Month()),
			"users": userGrowthMap[int(d.Month())],
		})
	}

	type dailyCount struct {
		Day   string
		Count int64
	}
	var activityData []dailyCount
	db.DB.Model(&models.StudySession{}).
		Select("TO_CHAR(start_time, 'DD/MM') as day, COUNT(*) as count").
		Where("start_time >= ?", startOfSevenDays).
		Group("TO_CHAR(start_time, 'DD/MM')").
		Scan(&activityData)

	activityMap := make(map[string]int64)
	for _, d := range activityData {
		activityMap[d.Day] = d.Count
	}

	activityChart := make([]gin.H, 0, 7)
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dayKey := d.Format("02/01")
		activityChart = append(activityChart, gin.H{
			"day":      dayKey,
			"sessions": activityMap[dayKey],
		})
	}

	// Fetch additional data for dashboard
	var (
		totalTeachers     int64
		activeStudents    int64
		publishedCourses  int64
		draftCourses      int64
		reviewCourses     int64
		totalLessons      int64
		totalEnrollments  int64
		dailyRevenue      float64
		monthlyRevenue    float64
		yearlyRevenue     float64
		pendingRevenue    float64
		recentUsers       []models.User
		recentTeachers    []models.User
		recentCourses     []models.LmsCourse
		recentOrders      []models.UserSubscription
		recentPayments    []models.Payment
		recentExams       []models.Exam
		recentAssignments []models.Task
		announcements     []models.Notification
		liveClasses       []models.LiveSession
		securityAlerts    []models.SecurityLog
	)

	// Fetch teachers count (users with TEACHER role)
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND role = ?", models.RoleTeacher).Count(&totalTeachers)

	// Fetch active students (users with recent activity)
	monthAgo := now.AddDate(0, -1, 0)
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND updated_at >= ?", monthAgo).Count(&activeStudents)

	// Fetch course statistics
	db.DB.Model(&models.LmsCourse{}).Where("deleted_at IS NULL AND status = ?", "PUBLISHED").Count(&publishedCourses)
	db.DB.Model(&models.LmsCourse{}).Where("deleted_at IS NULL AND status = ?", "DRAFT").Count(&draftCourses)
	db.DB.Model(&models.LmsCourse{}).Where("deleted_at IS NULL AND status = ?", "REVIEW").Count(&reviewCourses)
	db.DB.Model(&models.Lesson{}).Where("deleted_at IS NULL").Count(&totalLessons)
	db.DB.Model(&models.Enrollment{}).Where("deleted_at IS NULL").Count(&totalEnrollments)

	// Fetch revenue data
	db.DB.Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("deleted_at IS NULL AND created_at >= ?", todayStart).
		Scan(&dailyRevenue)

	db.DB.Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("deleted_at IS NULL AND created_at >= ?", weekAgo).
		Scan(&monthlyRevenue)

	db.DB.Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("deleted_at IS NULL AND created_at >= ?", sixMonthsAgo).
		Scan(&yearlyRevenue)

	db.DB.Model(&models.UserSubscription{}).
		Select("COALESCE(SUM(sp.price), 0)").
		Joins("LEFT JOIN \"SubscriptionPlan\" sp ON \"UserSubscription\".plan_id = sp.id").
		Where("\"UserSubscription\".status = ?", "PENDING").
		Scan(&pendingRevenue)

	// Fetch recent items
	wg.Add(8)
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&recentUsers)
	}()
	go func() {
		defer wg.Done()
		db.DB.Where("deleted_at IS NULL AND role = ?", models.RoleTeacher).Order(createdAtDescSort).Limit(5).Find(&recentTeachers)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&recentCourses)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&recentOrders)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&recentPayments)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&recentExams)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&recentAssignments)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&announcements)
	}()
	wg.Wait()

	// Fetch live classes and security alerts
	wg.Add(2)
	go func() {
		defer wg.Done()
		db.DB.Where("status = ?", "LIVE").Order(createdAtDescSort).Limit(5).Find(&liveClasses)
	}()
	go func() {
		defer wg.Done()
		db.DB.Order(createdAtDescSort).Limit(5).Find(&securityAlerts)
	}()
	wg.Wait()

	// Calculate growth rates
	var newUsersThisMonth int64
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND created_at >= ?", monthAgo).Count(&newUsersThisMonth)

	studentGrowthRate := float64(0)
	if totalUsers > 0 {
		studentGrowthRate = float64(newUsersThisWeek) / float64(totalUsers) * 100
	}

	var newTeachersToday int64
	var newTeachersThisWeek int64
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND role = ? AND created_at >= ?", models.RoleTeacher, todayStart).Count(&newTeachersToday)
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND role = ? AND created_at >= ?", models.RoleTeacher, weekAgo).Count(&newTeachersThisWeek)

	teacherGrowthRate := float64(0)
	if totalTeachers > 0 {
		teacherGrowthRate = float64(newTeachersThisWeek) / float64(totalTeachers) * 100
	}

	responsePayload := gin.H{
		"stats": gin.H{
			"totalUsers":       totalUsers,
			"totalSubjects":    totalSubjects,
			"totalExams":       totalExams,
			"totalResources":   totalResources,
			"activeChallenges": activeChallenges,
			"newUsersToday":    newUsersToday,
			"newUsersThisWeek": newUsersThisWeek,
		},
		"trends": gin.H{
			"userGrowth": calculateUserGrowthTrend(),
			"studyTime":  calculateStudyTimeTrend(),
		},
		"charts": gin.H{
			"userGrowth": userGrowth,
			"activity":   activityChart,
		},
		"activity": gin.H{
			"tasksCompleted":     completedTasks,
			"examsTaken":         examsTaken,
			"achievementsEarned": achievementsEarned,
			"studyMinutes":       studyMinutes,
		},
		"recentActivity": recentActivityItems,
		"upcomingEvents": upcomingEvents,
		// New data structure for frontend
		"revenue": gin.H{
			"dailyRevenue":   dailyRevenue,
			"monthlyRevenue": monthlyRevenue,
			"yearlyRevenue":  yearlyRevenue,
			"pendingRevenue": pendingRevenue,
			"dailyTrend":     calculateUserGrowthTrend(),
			"monthlyTrend":   calculateUserGrowthTrend(),
			"yearlyTrend":    calculateUserGrowthTrend(),
		},
		"users": gin.H{
			"totalUsers":        totalUsers,
			"activeStudents":    activeStudents,
			"newUsersToday":     newUsersToday,
			"newUsersThisWeek":  newUsersThisWeek,
			"newUsersThisMonth": newUsersThisMonth,
			"studentGrowthRate": studentGrowthRate,
			"recentStudents":    recentUsers,
		},
		"teachers": gin.H{
			"totalTeachers":       totalTeachers,
			"activeTeachers":      totalTeachers,
			"newTeachersToday":    newTeachersToday,
			"newTeachersThisWeek": newTeachersThisWeek,
			"teacherGrowthRate":   teacherGrowthRate,
			"recentTeachers":      recentTeachers,
		},
		"courses": gin.H{
			"totalCourses":      totalSubjects,
			"publishedCourses":  publishedCourses,
			"draftCourses":      draftCourses,
			"reviewCourses":     reviewCourses,
			"totalLessons":      totalLessons,
			"totalEnrollments":  totalEnrollments,
			"activeEnrollments": totalEnrollments,
			"recentCourses":     recentCourses,
		},
		"payments": gin.H{
			"recentOrders":   recentOrders,
			"recentPayments": recentPayments,
		},
		"exams": gin.H{
			"exams": recentExams,
		},
		"assignments": gin.H{
			"assignments": recentAssignments,
		},
		"announcements": gin.H{
			"announcements": announcements,
		},
		"live": gin.H{
			"classes": liveClasses,
		},
		"security": gin.H{
			"alerts": securityAlerts,
		},
	}

	if db.Redis != nil {
		dataBytes, err := json.Marshal(responsePayload)
		if err == nil {
			db.Redis.Set(c.Request.Context(), cacheKey, dataBytes, 5*time.Minute)
		}
	}

	api_response.Success(c, responsePayload)
}

type liveActivitySummary struct {
	SubjectMap map[string]models.Subject
	ExamMap    map[string]models.Exam
}

func buildLiveActivityMaps(examResults []models.ExamResult, studySessions []models.StudySession) liveActivitySummary {
	summary := liveActivitySummary{
		SubjectMap: make(map[string]models.Subject),
		ExamMap:    make(map[string]models.Exam),
	}

	populateExamMaps(examResults, &summary)
	populateSessionSubjectMap(studySessions, &summary)

	return summary
}

func populateExamMaps(results []models.ExamResult, summary *liveActivitySummary) {
	examIDs := make([]string, 0, len(results))
	for _, r := range results {
		examIDs = append(examIDs, r.ExamID)
	}

	if len(examIDs) == 0 {
		return
	}

	var exams []models.Exam
	db.DB.Where(idInQuery, examIDs).Find(&exams)

	subjectIDs := make([]string, 0, len(exams))
	for _, e := range exams {
		summary.ExamMap[e.ID] = e
		subjectIDs = append(subjectIDs, e.SubjectID)
	}

	if len(subjectIDs) > 0 {
		var subjects []models.Subject
		db.DB.Where(idInQuery, subjectIDs).Find(&subjects)
		for _, s := range subjects {
			summary.SubjectMap[s.ID] = s
		}
	}
}

func populateSessionSubjectMap(sessions []models.StudySession, summary *liveActivitySummary) {
	sessionSubjectIDs := make([]string, 0, len(sessions))
	for _, s := range sessions {
		if s.SubjectID != nil && *s.SubjectID != "" {
			sessionSubjectIDs = append(sessionSubjectIDs, *s.SubjectID)
		}
	}

	if len(sessionSubjectIDs) > 0 {
		var subjects []models.Subject
		db.DB.Where(idInQuery, sessionSubjectIDs).Find(&subjects)
		for _, s := range subjects {
			summary.SubjectMap[s.ID] = s
		}
	}
}

type liveUserActivity struct {
	Type    string
	Details interface{}
	Time    time.Time
}

func determineUserLiveActivity(user models.User, examResults []models.ExamResult, studySessions []models.StudySession, summary liveActivitySummary) liveUserActivity {
	// Check Exams first (higher priority activity)
	for _, result := range examResults {
		if result.UserID != user.ID {
			continue
		}

		activity := liveUserActivity{
			Type: "taking_exam",
			Time: result.TakenAt,
		}

		if exam, found := summary.ExamMap[result.ExamID]; found {
			subject := summary.SubjectMap[exam.SubjectID]
			activity.Details = gin.H{
				"type": "exam",
				"exam": gin.H{
					"id":    exam.ID,
					"title": exam.Title,
					"subject": gin.H{
						"name":   subject.Name,
						"nameAr": stringOrEmpty(subject.NameAr),
					},
				},
				"takenAt": result.TakenAt.Format(time.RFC3339),
				"score":   result.Score,
			}
		}
		return activity
	}

	// Check Study Sessions
	for _, session := range studySessions {
		if session.UserID != user.ID {
			continue
		}

		var subject models.Subject
		if session.SubjectID != nil && *session.SubjectID != "" {
			subject = summary.SubjectMap[*session.SubjectID]
		}

		return liveUserActivity{
			Type: "studying",
			Time: session.UpdatedAt,
			Details: gin.H{
				"type": "study",
				"subject": gin.H{
					"id":     subject.ID,
					"name":   subject.Name,
					"nameAr": stringOrEmpty(subject.NameAr),
				},
				"startTime": session.StartTime.Format(time.RFC3339),
				"duration":  session.DurationMin,
			},
		}
	}

	return liveUserActivity{
		Type: "online",
		Time: user.UpdatedAt,
	}
}

func filterLiveUsers(users []gin.H, filterType string) []gin.H {
	if filterType == "all" {
		return users
	}
	filtered := make([]gin.H, 0)
	for _, user := range users {
		if (filterType == "exam" && user["currentActivity"] == "taking_exam") ||
			(filterType == "study" && user["currentActivity"] == "studying") ||
			(filterType == "online" && user["currentActivity"] == "online") {
			filtered = append(filtered, user)
		}
	}
	return filtered
}

func GetAdminLive(c *gin.Context) {
	minutes, _ := strconv.Atoi(c.DefaultQuery("minutes", "5"))
	if minutes <= 0 {
		minutes = 5
	}
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)

	var users []models.User
	if err := db.DB.Where(statusQuery, models.StatusActive).Order("updated_at desc").Limit(200).Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch active users")
		return
	}

	var studySessions []models.StudySession
	_ = db.DB.Where("updated_at >= ? OR start_time >= ? OR end_time >= ?", cutoff, cutoff, cutoff).Find(&studySessions).Error

	var examResults []models.ExamResult
	_ = db.DB.Where("taken_at >= ?", cutoff).Find(&examResults).Error

	summary := buildLiveActivityMaps(examResults, studySessions)

	activeUsers := make([]gin.H, 0, len(users))
	stats := struct {
		Studying   int
		TakingExam int
		Online     int
		RoleStats  map[string]int
	}{
		RoleStats: map[string]int{"students": 0, "teachers": 0, "admins": 0},
	}

	for _, user := range users {
		switch user.Role {
		case models.RoleStudent:
			stats.RoleStats["students"]++
		case models.RoleTeacher:
			stats.RoleStats["teachers"]++
		case models.RoleAdmin:
			stats.RoleStats["admins"]++
		}

		activity := determineUserLiveActivity(user, examResults, studySessions, summary)

		switch activity.Type {
		case "taking_exam":
			stats.TakingExam++
		case "studying":
			stats.Studying++
		case "online":
			stats.Online++
		}

		activeUsers = append(activeUsers, gin.H{
			"userId": user.ID,
			"user": gin.H{
				"id":     user.ID,
				"name":   firstNonEmpty(stringOrEmpty(user.Name), stringOrEmpty(user.Username), user.Email),
				"email":  user.Email,
				"role":   user.Role,
				"avatar": user.Avatar,
			},
			"lastAccessed":    activity.Time.Format(time.RFC3339),
			"currentActivity": activity.Type,
			"activityDetails": activity.Details,
			"isActive":        true,
			"sessionId":       nil,
			"ip":              nil,
			"deviceInfo":      nil,
		})
	}

	filteredUsers := filterLiveUsers(activeUsers, c.DefaultQuery("type", "all"))

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"activeUsers": filteredUsers,
		"stats": gin.H{
			"totalActive": len(activeUsers),
			"studying":    stats.Studying,
			"takingExam":  stats.TakingExam,
			"online":      stats.Online,
			"byRole":      stats.RoleStats,
		},
	})
}

func GetAdminAnnouncements(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Notification{})
	if search := c.Query("search"); search != "" {
		like := "%" + search + "%"
		query = query.Where("title ILIKE ? OR message ILIKE ?", like, like)
	}

	var total int64
	query.Count(&total)

	var notifications []models.Notification
	if err := query.Order(createdAtDescSort).Offset(offset).Limit(limit).Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcements"})
		return
	}

	items := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, gin.H{
			"id":        n.ID,
			"title":     n.Title,
			"content":   n.Message,
			"type":      n.Type,
			"priority":  0,
			"isActive":  true,
			"createdAt": n.CreatedAt,
			"author": gin.H{
				"id":     "system",
				"name":   "System",
				"avatar": nil,
			},
		})
	}

	api_response.List(c, items, api_response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: calculateTotalPages(total, limit),
	}, gin.H{
		"announcements": items,
	})
}

func CreateAdminAnnouncement(c *gin.Context) {
	var input struct {
		Title    string `json:"title" binding:"required"`
		Content  string `json:"content" binding:"required"`
		Type     string `json:"type"`
		IsActive bool   `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notification := models.Notification{
		UserID:  "system",
		Title:   input.Title,
		Message: input.Content,
		Type:    models.NotificationType(input.Type),
		IsRead:  false,
	}
	if notification.Type == "" {
		notification.Type = models.NotificationInfo
	}

	if err := SafeCreate(db.DB, &notification); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create announcement"})
		return
	}

	broadcastMsg, _ := json.Marshal(gin.H{
		"type": "notification",
		"payload": gin.H{
			"title":   notification.Title,
			"message": notification.Message,
			"type":    notification.Type,
		},
	})
	GlobalHub.broadcast <- broadcastMsg

	c.JSON(http.StatusCreated, gin.H{"success": true})
}

func UpdateAdminAnnouncement(c *gin.Context) {
	var input struct {
		ID       string `json:"id" binding:"required"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Type     string `json:"type"`
		IsActive bool   `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type announcementUpdates struct {
		Title   *string `gorm:"column:title"`
		Message *string `gorm:"column:message"`
		Type    *string `gorm:"column:type"`
	}

	updates := announcementUpdates{}
	if input.Title != "" {
		updates.Title = &input.Title
	}
	if input.Content != "" {
		updates.Message = &input.Content
	}
	if input.Type != "" {
		updates.Type = &input.Type
	}

	if err := db.DB.Model(&models.Notification{}).Where(queryID, input.ID).
		Updates(&updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update announcement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteAdminAnnouncement(c *gin.Context) {
	var input struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.DB.Delete(&models.Notification{}, queryID, input.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete announcement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetAdminAnalytics(c *gin.Context) {
	var totalUsers int64
	var activeUsers int64
	var totalSubjects int64
	var totalExams int64
	var totalNotifications int64
	var totalXP int64
	var totalBlogPosts int64
	var achievementsEarned int64
	var challengesCompleted int64

	db.DB.Model(&models.User{}).Count(&totalUsers)
	db.DB.Model(&models.User{}).Where(statusQuery, models.StatusActive).Count(&activeUsers)
	db.DB.Model(&models.Subject{}).Count(&totalSubjects)
	db.DB.Model(&models.Exam{}).Count(&totalExams)
	db.DB.Model(&models.Notification{}).Count(&totalNotifications)
	db.DB.Model(&models.User{}).Select("COALESCE(SUM(total_xp), 0)").Scan(&totalXP)
	db.DB.Model(&models.BlogPost{}).Count(&totalBlogPosts)
	db.DB.Model(&models.Achievement{}).Select("COALESCE(SUM(unlocked_count), 0)").Scan(&achievementsEarned)
	db.DB.Model(&models.Challenge{}).Where(isActiveQuery, false).Count(&challengesCompleted)

	roleStats := gin.H{}
	for _, role := range []models.UserRole{models.RoleAdmin, models.RoleTeacher, models.RoleStudent} {
		var count int64
		db.DB.Model(&models.User{}).Where(queryRole, role).Count(&count)
		roleStats[string(role)] = count
	}

	type point struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	dailyUsers := make([]point, 0, 7)
	dailyRegistrations := make([]point, 0, 7)
	now := time.Now()
	for i := 6; i >= 0; i-- {
		start := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		end := start.Add(24 * time.Hour)

		var createdCount int64
		var activeCount int64
		db.DB.Model(&models.User{}).Where(createdAtRangeQuery, start, end).Count(&createdCount)
		db.DB.Model(&models.StudySession{}).Where(createdAtRangeQuery, start, end).Distinct("user_id").Count(&activeCount)

		dailyUsers = append(dailyUsers, point{Date: start.Format(dateFormat), Count: activeCount})
		dailyRegistrations = append(dailyRegistrations, point{Date: start.Format(dateFormat), Count: createdCount})
	}

	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"total":  totalUsers,
			"new":    dailyRegistrations[len(dailyRegistrations)-1].Count,
			"active": activeUsers,
			"byRole": roleStats,
		},
		"content": gin.H{
			"subjects":  totalSubjects,
			"exams":     totalExams,
			"blogPosts": totalBlogPosts,
		},
		"gamification": gin.H{
			"totalXP":             totalXP,
			"achievementsEarned":  achievementsEarned,
			"challengesCompleted": challengesCompleted,
		},
		"charts": gin.H{
			"dailyActiveUsers":   dailyUsers,
			"dailyRegistrations": dailyRegistrations,
		},
	})
}

func GetAdminReportsOverview(c *gin.Context) {
	var totalUsers int64
	var usersToday int64
	var usersWeek int64
	var usersMonth int64
	var totalSubjects int64
	var activeSubjects int64
	var totalNotifications int64
	var totalStudySessions int64

	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, -1, 0)

	db.DB.Model(&models.User{}).Count(&totalUsers)
	db.DB.Model(&models.User{}).Where(createdAtGte, dayAgo).Count(&usersToday)
	db.DB.Model(&models.User{}).Where(createdAtGte, weekAgo).Count(&usersWeek)
	db.DB.Model(&models.User{}).Where(createdAtGte, monthAgo).Count(&usersMonth)
	db.DB.Model(&models.Subject{}).Count(&totalSubjects)
	db.DB.Model(&models.Subject{}).Where(isActiveQuery, true).Count(&activeSubjects)
	db.DB.Model(&models.Notification{}).Count(&totalNotifications)
	db.DB.Model(&models.StudySession{}).Count(&totalStudySessions)

	var subjects []models.Subject
	db.DB.Order("enrolled_count desc").Limit(5).Find(&subjects)
	popularSubjects := make([]gin.H, 0, len(subjects))
	for _, subject := range subjects {
		popularSubjects = append(popularSubjects, gin.H{
			"id":            subject.ID,
			"title":         firstNonEmpty(stringOrEmpty(subject.NameAr), subject.Name),
			"enrolledCount": subject.EnrolledCount,
			"isPublished":   subject.IsPublished,
		})
	}

	type trendPoint struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	registrationTrend := make([]trendPoint, 0, 7)
	for i := 6; i >= 0; i-- {
		start := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		end := start.Add(24 * time.Hour)
		var count int64
		db.DB.Model(&models.User{}).Where(createdAtRangeQuery, start, end).Count(&count)
		registrationTrend = append(registrationTrend, trendPoint{Date: start.Format(dateFormat), Count: count})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"users": gin.H{
				"total":        totalUsers,
				"newToday":     usersToday,
				"newThisWeek":  usersWeek,
				"newThisMonth": usersMonth,
			},
			"books": gin.H{
				"total":              0,
				"newThisMonth":       0,
				"totalDownloads":     0,
				"downloadsThisMonth": 0,
			},
			"subjects": gin.H{
				"total":  totalSubjects,
				"active": activeSubjects,
			},
			"engagement": gin.H{
				"totalReviews":     0,
				"reviewsThisMonth": 0,
				"activeSessions":   totalStudySessions,
			},
			"popularBooks":    []interface{}{},
			"popularSubjects": popularSubjects,
			"trends": gin.H{
				"userRegistrations": registrationTrend,
			},
		},
	})
}

func GetAdminReportsUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.User{}).Count(&total)

	var users []models.User
	if err := db.DB.Order(createdAtDescSort).Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users report"})
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":                user.ID,
			"name":              user.Name,
			"email":             user.Email,
			"username":          user.Username,
			"role":              user.Role,
			"status":            user.Status,
			"createdAt":         user.CreatedAt,
			"lastLogin":         nil,
			"monthlyAiMessages": 0,
			"monthlyExams":      0,
			"uploadedBooks":     0,
			"reviews":           0,
			"sessions":          0,
			"studySessions":     0,
		})
	}

	roleCounts := []gin.H{}
	for _, role := range []models.UserRole{models.RoleAdmin, models.RoleTeacher, models.RoleStudent} {
		var count int64
		db.DB.Model(&models.User{}).Where(queryRole, role).Count(&count)
		roleCounts = append(roleCounts, gin.H{"role": role, "count": count})
	}

	api_response.Success(c, gin.H{
		"items": items,
		"users": items,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalCount": total,
			"totalPages": calculateTotalPages(total, limit),
		},
		"stats": gin.H{
			"totalUsers": total,
			"byRole":     roleCounts,
		},
	})
}

func GetAdminInfrastructureStats(c *gin.Context) {
	numGoroutines := runtime.NumGoroutine()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryMiB := m.Alloc / 1024 / 1024
	totalAllocatedMiB := m.TotalAlloc / 1024 / 1024
	gcPauseTotalMs := m.PauseTotalNs / 1_000_000

	dbStatus := "healthy"
	dbConnections := 0
	sqlDB, err := db.DB.DB()
	if err != nil {
		dbStatus = "unhealthy"
	} else {
		pingCtx, pingCancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer pingCancel()
		if pingErr := sqlDB.PingContext(pingCtx); pingErr != nil {
			dbStatus = "unhealthy"
		} else {
			dbConnections = sqlDB.Stats().OpenConnections
		}
	}

	redisLatency := "N/A"
	redisConnected := false
	if db.Redis != nil {
		if _, err := db.Redis.Ping(c.Request.Context()).Result(); err == nil {
			redisConnected = true
			redisLatency = "connected"
		} else {
			redisLatency = "disconnected"
		}
	}

	queues := gin.H{
		"active":  0,
		"waiting": 0,
		"failed":  0,
	}

	// Include performance metrics from the monitoring middleware
	metrics := runtimeMetrics()

	c.JSON(http.StatusOK, gin.H{
		"goroutines":        numGoroutines,
		"memoryMiB":         memoryMiB,
		"totalAllocatedMiB": totalAllocatedMiB,
		"gcPauseTotalMs":    gcPauseTotalMs,
		"dbStatus":          dbStatus,
		"dbConnections":     dbConnections,
		"redisLatency":      redisLatency,
		"redisConnected":    redisConnected,
		"queues":            queues,
		"requestMetrics":    metrics,
	})
}

// runtimeMetrics returns a snapshot of runtime and request metrics
func runtimeMetrics() gin.H {
	numGoroutines := runtime.NumGoroutine()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return gin.H{
		"go_version":        runtime.Version(),
		"goroutines":        numGoroutines,
		"memory_alloc_mib":  m.Alloc / 1024 / 1024,
		"memory_total_mib":  m.TotalAlloc / 1024 / 1024,
		"memory_sys_mib":    m.Sys / 1024 / 1024,
		"gc_cycles":         m.NumGC,
		"gc_pause_total_ms": m.PauseTotalNs / 1_000_000,
		"cpu_logical_cores": runtime.NumCPU(),
		"cgo_calls":         runtime.NumCgoCall(),
	}
}

func GetAdminReportsBooks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	api_response.Success(c, gin.H{
		"items": []interface{}{},
		"books": []interface{}{},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      0,
			"totalCount": 0,
			"totalPages": 1,
		},
		"stats": gin.H{
			"totalBooks":     0,
			"avgRating":      0,
			"totalDownloads": 0,
			"totalViews":     0,
		},
	})
}

func calculateUserGrowthTrend() float64 {
	now := time.Now()
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := thisMonthStart.AddDate(0, -1, 0)

	var thisMonth int64
	var lastMonth int64

	db.DB.Model(&models.User{}).Where(createdAtGte, thisMonthStart).Count(&thisMonth)
	db.DB.Model(&models.User{}).Where(createdAtGte+" AND created_at < ?", lastMonthStart, thisMonthStart).Count(&lastMonth)

	if lastMonth == 0 {
		if thisMonth > 0 {
			return 100.0
		}
		return 0.0
	}

	return float64(thisMonth-lastMonth) / float64(lastMonth) * 100.0
}

func calculateStudyTimeTrend() float64 {
	now := time.Now()
	thisWeekStart := now.AddDate(0, 0, -int(now.Weekday()))
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)

	var thisWeek int64
	var lastWeek int64

	db.DB.Model(&models.StudySession{}).Where("start_time >= ?", thisWeekStart).Select(coalesceSumDuration).Scan(&thisWeek)
	db.DB.Model(&models.StudySession{}).Where("start_time >= ? AND start_time < ?", lastWeekStart, thisWeekStart).Select(coalesceSumDuration).Scan(&lastWeek)

	if lastWeek == 0 {
		if thisWeek > 0 {
			return 100.0
		}
		return 0.0
	}

	return float64(thisWeek-lastWeek) / float64(lastWeek) * 100.0
}
