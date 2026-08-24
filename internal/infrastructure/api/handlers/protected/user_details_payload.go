package protected

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// userStats holds all computed statistics for a single user.
type userStats struct {
	TasksCompleted      int64
	TotalTasks          int64
	TotalStudySessions  int64
	TotalStudyTime      int64
	ExamsPassed         int64
	ExamResultsCount    int64
	UnreadNotifications int64
	TotalEnrollments    int64
	AchievementsCount   int64
}

// loadUserStats fetches user statistics from Redis cache if available,
// otherwise falls back to direct database queries and caches the result.
func loadUserStats(user models.User) userStats {
	cacheKey := fmt.Sprintf("user_stats:%s", user.ID)
	bgCtx := context.Background()

	// Try Redis cache first
	if cache.Redis != nil {
		readCtx, cancel := context.WithTimeout(bgCtx, 200*time.Millisecond)
		cachedVal, err := cache.Redis.Get(readCtx, cacheKey).Result()
		cancel()
		if err == nil {
			var cached userStats
			if json.Unmarshal([]byte(cachedVal), &cached) == nil {
				return cached
			}
		}
	}

	// Cache miss — query the database
	getDB := func() *gorm.DB {
		if rdb := db.ReadDB(); rdb != nil {
			return rdb
		}
		return db.DB.Session(&gorm.Session{})
	}

	var s userStats
	getDB().Model(&models.Task{}).Where("user_id = ? AND status = ?", user.ID, models.TaskCompleted).Count(&s.TasksCompleted)
	getDB().Model(&models.Task{}).Where(userIDQuery, user.ID).Count(&s.TotalTasks)
	getDB().Model(&models.StudySession{}).Where(userIDQuery, user.ID).Count(&s.TotalStudySessions)
	getDB().Model(&models.StudySession{}).Where(userIDQuery, user.ID).Select("COALESCE(SUM(duration_min), 0)").Scan(&s.TotalStudyTime)
	getDB().Model(&models.ExamResult{}).Where("user_id = ? AND passed = ?", user.ID, true).Count(&s.ExamsPassed)
	getDB().Model(&models.ExamResult{}).Where(userIDQuery, user.ID).Count(&s.ExamResultsCount)
	getDB().Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", user.ID, false).Count(&s.UnreadNotifications)
	getDB().Model(&models.Enrollment{}).Where(userIDQuery, user.ID).Count(&s.TotalEnrollments)

	// Cache the results for 3 minutes
	if cache.Redis != nil {
		cachedData, _ := json.Marshal(map[string]interface{}{
			"tasksCompleted":      s.TasksCompleted,
			"totalTasks":          s.TotalTasks,
			"totalStudySessions":  s.TotalStudySessions,
			"totalStudyTime":      s.TotalStudyTime,
			"examsPassed":         s.ExamsPassed,
			"examResultsCount":    s.ExamResultsCount,
			"unreadNotifications": s.UnreadNotifications,
			"totalEnrollments":    s.TotalEnrollments,
			"achievementsCount":   s.AchievementsCount,
		})
		cache.Redis.Set(bgCtx, cacheKey, cachedData, 3*time.Minute)
	}

	return s
}

// buildUserDetailsPayload assembles the full user-details API response payload.
func buildUserDetailsPayload(user models.User) gin.H {
	s := loadUserStats(user)

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
		"totalStudyTime":        s.TotalStudyTime,
		"tasksCompleted":        s.TasksCompleted,
		"examsPassed":           s.ExamsPassed,
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
			"tasks":              s.TotalTasks,
			"studySessions":      s.TotalStudySessions,
			"achievements":       s.AchievementsCount,
			"notifications":      s.UnreadNotifications,
			"examResults":        s.ExamResultsCount,
			"subjectEnrollments": s.TotalEnrollments,
			"customGoals":        0,
			"reminders":          0,
			"sessions":           0,
		},
		"achievements":  []interface{}{},
		"examResults":   []interface{}{},
		"studySessions": []interface{}{},
	}
}
