package protected

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func buildUserDetailsPayload(user models.User) gin.H {
	var tasksCompleted int64
	var totalTasks int64
	var totalStudySessions int64
	var totalStudyTime int64
	var examsPassed int64
	var examResultsCount int64
	var unreadNotifications int64
	var totalEnrollments int64
	var achievementsCount int64

	// Try Redis cache first for user stats
	statsCached := false
	cacheKey := fmt.Sprintf("user_stats:%s", user.ID)
	bgCtx := context.Background()
	if cache.Redis != nil {
		cachedVal, err := cache.Redis.Get(bgCtx, cacheKey).Result()
		if err == nil {
			var cached struct {
				TasksCompleted      int64 `json:"tasksCompleted"`
				TotalTasks          int64 `json:"totalTasks"`
				TotalStudySessions  int64 `json:"totalStudySessions"`
				TotalStudyTime      int64 `json:"totalStudyTime"`
				ExamsPassed         int64 `json:"examsPassed"`
				ExamResultsCount    int64 `json:"examResultsCount"`
				UnreadNotifications int64 `json:"unreadNotifications"`
				TotalEnrollments    int64 `json:"totalEnrollments"`
				AchievementsCount   int64 `json:"achievementsCount"`
			}
			if json.Unmarshal([]byte(cachedVal), &cached) == nil {
				tasksCompleted = cached.TasksCompleted
				totalTasks = cached.TotalTasks
				totalStudySessions = cached.TotalStudySessions
				totalStudyTime = cached.TotalStudyTime
				examsPassed = cached.ExamsPassed
				examResultsCount = cached.ExamResultsCount
				unreadNotifications = cached.UnreadNotifications
				totalEnrollments = cached.TotalEnrollments
				achievementsCount = cached.AchievementsCount
				statsCached = true
			}
		}
	}

	if !statsCached {
		// Merge into fewer queries using subqueries for better performance
		getDB := func() *gorm.DB {
			if rdb := db.ReadDB(); rdb != nil {
				return rdb
			}
			return db.DB.Session(&gorm.Session{})
		}

		getDB().Model(&models.Task{}).Where("user_id = ? AND status = ?", user.ID, models.TaskCompleted).Count(&tasksCompleted)
		getDB().Model(&models.Task{}).Where(userIDQuery, user.ID).Count(&totalTasks)
		getDB().Model(&models.StudySession{}).Where(userIDQuery, user.ID).Count(&totalStudySessions)
		getDB().Model(&models.StudySession{}).Where(userIDQuery, user.ID).Select("COALESCE(SUM(duration_min), 0)").Scan(&totalStudyTime)
		getDB().Model(&models.ExamResult{}).Where("user_id = ? AND passed = ?", user.ID, true).Count(&examsPassed)
		getDB().Model(&models.ExamResult{}).Where(userIDQuery, user.ID).Count(&examResultsCount)
		getDB().Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", user.ID, false).Count(&unreadNotifications)
		getDB().Model(&models.Enrollment{}).Where(userIDQuery, user.ID).Count(&totalEnrollments)

		// Cache the results for 3 minutes
		if cache.Redis != nil {
			cachedData, _ := json.Marshal(map[string]interface{}{
				"tasksCompleted":      tasksCompleted,
				"totalTasks":          totalTasks,
				"totalStudySessions":  totalStudySessions,
				"totalStudyTime":      totalStudyTime,
				"examsPassed":         examsPassed,
				"examResultsCount":    examResultsCount,
				"unreadNotifications": unreadNotifications,
				"totalEnrollments":    totalEnrollments,
				"achievementsCount":   achievementsCount,
			})
			cache.Redis.Set(bgCtx, cacheKey, cachedData, 3*time.Minute)
		}
	}

	return gin.H{
		"id":                    user.ID,
		"email":                 user.Email,
		"name":                  user.Name,
		"username":              user.Username,
		"avatar":                user.Avatar,
		"role":                  user.Role,
		"emailVerified":         user.EmailVerified,
		"phone":                 user.Phone,
		"phoneVerified":         user.PhoneVerified,
		"twoFactorEnabled":      false,
		"createdAt":             user.CreatedAt,
		"updatedAt":             user.UpdatedAt,
		"lastLogin":             user.LastLogin,
		"totalXP":               user.TotalXP,
		"level":                 user.Level,
		"currentStreak":         user.CurrentStreak,
		"longestStreak":         0,
		"totalStudyTime":        totalStudyTime,
		"tasksCompleted":        tasksCompleted,
		"examsPassed":           examsPassed,
		"pomodoroSessions":      0,
		"deepWorkSessions":      0,
		"studyXP":               0,
		"taskXP":                0,
		"examXP":                0,
		"challengeXP":           0,
		"questXP":               0,
		"seasonXP":              0,
		"gradeLevel":            user.GradeLevel,
		"educationType":         user.EducationType,
		"section":               user.Section,
		"interestedSubjects":    []string{},
		"studyGoal":             nil,
		"bio":                   user.Bio,
		"school":                nil,
		"country":               user.Country,
		"dateOfBirth":           nil,
		"gender":                nil,
		"balance":               user.Balance,
		"aiCredits":             user.AiCredits,
		"examCredits":           user.ExamCredits,
		"activeSubscriptionId":  user.ActiveSubscriptionID,
		"subscriptionExpiresAt": user.SubscriptionExpiresAt,
		"statusReason":          user.StatusReason,
		"statusExpiresAt":       user.StatusExpiresAt,
		"createdBy":             nil,
		"archivedAt":            nil,
		"googleId":              nil,
		"githubId":              nil,
		"authProvider":          nil,
		"adminNotes":            []interface{}{},
		"_count": gin.H{
			"tasks":              totalTasks,
			"studySessions":      totalStudySessions,
			"achievements":       achievementsCount,
			"notifications":      unreadNotifications,
			"examResults":        examResultsCount,
			"subjectEnrollments": totalEnrollments,
			"customGoals":        0,
			"reminders":          0,
			"sessions":           0,
		},
		"achievements":  []interface{}{},
		"examResults":   []interface{}{},
		"studySessions": []interface{}{},
	}
}

// GetUserProfile returns the authenticated user's profile details including recovery codes if configured
func GetUserProfile(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	var settings models.TwoFactorSettings
	recoveryCodesJSON := ""
	if err := db.DB.First(&settings, userIDQuery, userId).Error; err == nil {
		if len(settings.BackupCodes) > 0 {
			if codesBytes, err := json.Marshal(settings.BackupCodes); err == nil {
				recoveryCodesJSON = string(codesBytes)
			}
		}
	}

	api_response.Success(c, gin.H{
		"id":            user.ID,
		"email":         user.Email,
		"username":      user.Username,
		"name":          user.Name,
		"avatar":        user.Avatar,
		"phone":         user.Phone,
		"phoneVerified": user.PhoneVerified,
		"emailVerified": user.EmailVerified,
		"gradeLevel":    user.GradeLevel,
		"educationType": user.EducationType,
		"section":       user.Section,
		"bio":           user.Bio,
		"country":       user.Country,
		"recoveryCodes": recoveryCodesJSON,
	})
}

func GetUserEnrollments(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 50
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var total int64
	db.DB.Model(&models.Enrollment{}).Where("user_id = ?", userID).Count(&total)

	var enrollments []models.Enrollment
	if err := db.DB.Preload("Subject").Where("user_id = ?", userID).
		Order("enrolled_at DESC").
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch enrollments")
		return
	}

	totalProgress := 0.0
	items := make([]gin.H, 0, len(enrollments))
	for _, e := range enrollments {
		subjectName := ""
		subjectSlug := ""
		price := 0.0
		if e.Subject.ID != "" {
			subjectName = e.Subject.Name
			subjectSlug = stringOrEmpty(e.Subject.Slug)
			price, _ = e.Subject.Price.Float64()
		}
		progress, _ := e.Progress.Float64()
		totalProgress += progress
		items = append(items, gin.H{
			"id":         e.ID,
			"courseId":   e.SubjectID,
			"courseName": subjectName,
			"courseSlug": subjectSlug,
			"price":      price,
			"progress":   e.Progress,
			"status": func() string {
				if progress >= 100.0 {
					return "COMPLETED"
				}
				return "ACTIVE"
			}(),
			"enrolledAt": e.EnrolledAt,
		})
	}

	avgProgress := 0.0
	if len(enrollments) > 0 {
		avgProgress = totalProgress / float64(len(enrollments))
	}

	api_response.Success(c, gin.H{
		"userId":      userID,
		"total":       total,
		"avgProgress": avgProgress,
		"enrollments": items,
	})
}

func GetUserOrders(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 50
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var total int64
	db.DB.Model(&models.UserSubscription{}).Where("user_id = ?", userID).Count(&total)

	var subscriptions []models.UserSubscription
	if err := db.DB.Preload("Plan").Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&subscriptions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch orders")
		return
	}

	items := make([]gin.H, 0, len(subscriptions))
	for _, sub := range subscriptions {
		planName := ""
		planNameAr := ""
		price := decimal.NewFromInt(0)
		interval := ""
		if sub.Plan.ID != "" {
			planName = sub.Plan.Name
			planNameAr = sub.Plan.NameAr
			price = sub.Plan.Price
			interval = string(sub.Plan.Interval)
		}

		items = append(items, gin.H{
			"id":                   sub.ID,
			"planId":               sub.PlanID,
			"planName":             planName,
			"planNameAr":           planNameAr,
			"status":               string(sub.Status),
			"startDate":            sub.StartDate,
			"endDate":              sub.EndDate,
			"autoRenew":            sub.AutoRenew,
			"paymobSubscriptionId": sub.PaymobSubscriptionID,
			"price":                price,
			"interval":             interval,
			"createdAt":            sub.CreatedAt,
			"updatedAt":            sub.UpdatedAt,
		})
	}

	api_response.List(c, items, api_response.Pagination{
		Page:       1,
		Limit:      limit,
		Total:      total,
		TotalPages: calculateTotalPages(total, limit),
	}, gin.H{
		"orders": items,
	})
}

func AdminEnrollUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	var req struct {
		CourseID string `json:"courseId" binding:"required"`
		IsFree   bool   `json:"isFree"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, req.CourseID).First(&subject).Error; err != nil {
		handleSubjectError(c, req.CourseID, err, "verifying course for manual enrollment")
		return
	}

	if isAlreadyEnrolled(userID, subject.ID) {
		api_response.Success(c, gin.H{"success": true, "alreadyEnrolled": true, "message": "User is already enrolled"})
		return
	}

	if !req.IsFree {
		price, _ := subject.Price.Float64()
		if price > 0 && !hasPaidForSubject(userID, subject.ID) {
			api_response.Error(c, http.StatusBadRequest, "User has not paid for this course. Use isFree=true to bypass payment.")
			return
		}
	}

	if err := executeEnrollmentTransaction(userID, subject.ID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to enroll: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"success": true, "message": "User enrolled successfully"})
}

func GetUserVideoEngagement(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 100
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var progressRecords []models.LessonProgress
	if err := db.DB.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&progressRecords).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch video engagement")
		return
	}

	videos := make([]gin.H, 0, len(progressRecords))
	totalWatchSeconds := 0
	for _, p := range progressRecords {
		totalWatchSeconds += p.TimeSpentSeconds
		videos = append(videos, gin.H{
			"lessonId":            p.LessonID,
			"timeSpentSeconds":    p.TimeSpentSeconds,
			"timeSpentMinutes":    p.TimeSpentSeconds / 60,
			"completed":           p.Completed,
			"status":              string(p.Status),
			"lastWatchedPosition": p.LastWatchedPosition,
		})
	}

	api_response.Success(c, gin.H{
		"userId":            userID,
		"totalVideos":       len(videos),
		"totalWatchSeconds": totalWatchSeconds,
		"totalWatchMinutes": totalWatchSeconds / 60,
		"videos":            videos,
	})
}
