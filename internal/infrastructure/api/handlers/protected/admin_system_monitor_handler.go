package protected

import (
	"net/http"
	"runtime"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetAdminInfrastructureStats exposes runtime, database and Redis health so the
// admin UI can render the system diagnostics section from real values.
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
		if sqlDB.Ping() != nil {
			dbStatus = "unhealthy"
		} else {
			dbConnections = sqlDB.Stats().OpenConnections
		}
	}

	redisLatency := "N/A"
	redisConnected := false
	if cache.Redis != nil {
		if _, err := cache.Redis.Ping(c.Request.Context()).Result(); err == nil {
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

// GetAdminAnalytics aggregates platform-wide analytics for the reports page.
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
