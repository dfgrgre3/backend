package protected

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"

	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"thanawy-backend/internal/infrastructure/api/handlers/shared"
	"thanawy-backend/internal/infrastructure/monitoring"

	"github.com/gin-gonic/gin"
)

var healthProcessStartedAt = time.Now()

type healthRange struct {
	Name     string
	Duration time.Duration
	Interval time.Duration
}

type healthHistoryPoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

type detailedPerformance struct {
	RequestCount        int64                `json:"requestCount"`
	AvgResponseTime     float64              `json:"avgResponseTime"`
	P95ResponseTime     float64              `json:"p95ResponseTime"`
	P99ResponseTime     float64              `json:"p99ResponseTime"`
	RequestsPerMinute   float64              `json:"requestsPerMinute"`
	ErrorRate           float64              `json:"errorRate"`
	CPUUsage            *float64             `json:"cpuUsage"`
	MemoryUsage         float64              `json:"memoryUsage"`
	DatabaseConnections int                  `json:"databaseConnections"`
	ResponseTimeHistory []healthHistoryPoint `json:"responseTimeHistory"`
	ErrorRateHistory    []healthHistoryPoint `json:"errorRateHistory"`
	RequestsHistory     []healthHistoryPoint `json:"requestsHistory"`
}

type detailedHealthReport struct {
	System      gin.H               `json:"system"`
	Exams       gin.H               `json:"exams"`
	Security    gin.H               `json:"security"`
	Performance detailedPerformance `json:"performance"`
}

func parseHealthRange(value string) (healthRange, bool) {
	ranges := map[string]healthRange{
		"15m": {"15m", 15 * time.Minute, time.Minute},
		"1h":  {"1h", time.Hour, 5 * time.Minute},
		"6h":  {"6h", 6 * time.Hour, 15 * time.Minute},
		"24h": {"24h", 24 * time.Hour, time.Hour},
		"7d":  {"7d", 7 * 24 * time.Hour, 6 * time.Hour},
	}
	selected, ok := ranges[value]
	return selected, ok
}

func GetDetailedAdminHealth(c *gin.Context) {
	selected, ok := parseHealthRange(c.DefaultQuery("range", "1h"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range must be one of 15m, 1h, 6h, 24h, 7d"})
		return
	}
	report, err := queryDetailedHealth(c.Request.Context(), selected)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query health metrics"})
		return
	}
	c.JSON(http.StatusOK, report)
}

func ExportDetailedAdminHealth(c *gin.Context) {
	selected, ok := parseHealthRange(c.DefaultQuery("range", "24h"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range must be one of 15m, 1h, 6h, 24h, 7d"})
		return
	}
	report, err := queryDetailedHealth(c.Request.Context(), selected)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query health metrics"})
		return
	}
	format := strings.ToLower(c.DefaultQuery("format", "csv"))
	filename := "health-report-" + time.Now().UTC().Format("2006-01-02T150405Z")
	if format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be csv"})
		return
	}
	writeHealthCSV(c, filename, report)
}

func writeHealthCSV(c *gin.Context, filename string, report detailedHealthReport) {
	c.Header("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"time", "avg_response_ms", "error_rate_ratio", "request_count"}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build health export"})
		return
	}
	for i, point := range report.Performance.ResponseTimeHistory {
		errorRate := float64(0)
		requestCount := float64(0)
		if i < len(report.Performance.ErrorRateHistory) {
			errorRate = report.Performance.ErrorRateHistory[i].Value
		}
		if i < len(report.Performance.RequestsHistory) {
			requestCount = report.Performance.RequestsHistory[i].Value
		}
		if err := writer.Write([]string{point.Time, strconv.FormatFloat(point.Value, 'f', 2, 64), strconv.FormatFloat(errorRate, 'f', 6, 64), strconv.FormatFloat(requestCount, 'f', 0, 64)}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build health export"})
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build health export"})
		return
	}
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
}

func queryDetailedHealth(ctx context.Context, selected healthRange) (detailedHealthReport, error) {
	now := time.Now().UTC()
	points, summary, err := monitoring.QueryPerformance(ctx, now.Add(-selected.Duration), now, selected.Interval)
	if err != nil {
		return detailedHealthReport{}, err
	}
	system := querySystemHealth(ctx, now, summary)
	exams, err := queryExamHealth(ctx, now.Add(-selected.Duration))
	if err != nil {
		return detailedHealthReport{}, err
	}
	security, err := querySecurityHealth(ctx, now.Add(-selected.Duration))
	if err != nil {
		return detailedHealthReport{}, err
	}

	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	dbConnections := 0
	if sqlDB, sqlErr := db.DB.DB(); sqlErr == nil {
		dbConnections = sqlDB.Stats().OpenConnections
	}
	performance := detailedPerformance{
		RequestCount:    summary.RequestCount,
		AvgResponseTime: summary.AvgResponseTime, P95ResponseTime: summary.P95ResponseTime,
		P99ResponseTime: summary.P99ResponseTime, RequestsPerMinute: summary.RequestsPerMin,
		ErrorRate: summary.ErrorRate, CPUUsage: nil,
		MemoryUsage: float64(memory.Alloc) / 1024 / 1024, DatabaseConnections: dbConnections,
		ResponseTimeHistory: make([]healthHistoryPoint, 0, len(points)),
		ErrorRateHistory:    make([]healthHistoryPoint, 0, len(points)),
		RequestsHistory:     make([]healthHistoryPoint, 0, len(points)),
	}
	for _, point := range points {
		label := point.Time.Format(time.RFC3339)
		performance.ResponseTimeHistory = append(performance.ResponseTimeHistory, healthHistoryPoint{label, point.AvgDurationMS})
		performance.ErrorRateHistory = append(performance.ErrorRateHistory, healthHistoryPoint{label, point.ErrorRate})
		performance.RequestsHistory = append(performance.RequestsHistory, healthHistoryPoint{label, float64(point.RequestCount)})
	}
	return detailedHealthReport{System: system, Exams: exams, Security: security, Performance: performance}, nil
}

func querySystemHealth(ctx context.Context, now time.Time, summary monitoring.PerformanceSummary) gin.H {
	dbStart := time.Now()
	dbErr := db.DB.WithContext(ctx).Exec("SELECT 1").Error
	dbMS := float64(time.Since(dbStart).Microseconds()) / 1000
	dbComponent := healthComponent("Database", dbErr, dbMS, now, "PostgreSQL connectivity")

	redisStart := time.Now()
	var redisErr error
	if cache.Redis == nil {
		redisErr = fmt.Errorf("Redis client is not configured")
	} else {
		redisErr = cache.Redis.Ping(ctx).Err()
	}
	redisMS := float64(time.Since(redisStart).Microseconds()) / 1000
	redisComponent := healthComponent("Redis", redisErr, redisMS, now, "Redis connectivity")

	apiDetails := "HTTP metrics in selected range"
	apiStatus := "healthy"
	if summary.RequestCount == 0 {
		apiStatus = "degraded"
		apiDetails = "No HTTP request sample in selected range"
	} else if summary.ErrorRate >= 0.05 || summary.P95ResponseTime >= 1000 {
		apiStatus = "degraded"
	}
	apiComponent := gin.H{"name": "API", "status": apiStatus, "responseTime": summary.AvgResponseTime, "lastCheck": now.Format(time.RFC3339), "details": apiDetails}
	storageStart := time.Now()
	storageErr := probeStorage(ctx)
	storageMS := float64(time.Since(storageStart).Microseconds()) / 1000
	storageComponent := healthComponent("Storage", storageErr, storageMS, now, "Storage upload/download/delete probe")
	overall := "healthy"
	if dbErr != nil || redisErr != nil || storageErr != nil {
		overall = "unhealthy"
	} else if apiStatus != "healthy" {
		overall = "degraded"
	}
	components := []gin.H{dbComponent, redisComponent, apiComponent, storageComponent}
	return gin.H{
		"overall":    gin.H{"status": overall, "timestamp": now.Format(time.RFC3339), "uptime": time.Since(healthProcessStartedAt).Seconds(), "version": envOrDefault("APP_VERSION", "unknown")},
		"components": components,
		"checks":     gin.H{"database": dbComponent, "redis": redisComponent, "api": apiComponent, "storage": storageComponent},
	}
}

func probeStorage(ctx context.Context) error {
	return shared.ProbeStorage(ctx)
}

func healthComponent(name string, err error, responseMS float64, now time.Time, details string) gin.H {
	status := "healthy"
	if err != nil {
		status = "unhealthy"
		details = err.Error()
	}
	return gin.H{"name": name, "status": status, "responseTime": responseMS, "lastCheck": now.Format(time.RFC3339), "details": details}
}

func queryExamHealth(ctx context.Context, since time.Time) (gin.H, error) {
	read := db.ReadDB(ctx)
	var total, enabled, passed, failed int64
	var averageDuration float64
	queries := []error{
		read.Model(&models.Exam{}).Count(&total).Error,
		read.Model(&models.Exam{}).Where("is_active = ?", true).Count(&enabled).Error,
		read.Model(&models.ExamResult{}).Where("taken_at >= ? AND passed = ?", since, true).Count(&passed).Error,
		read.Model(&models.ExamResult{}).Where("taken_at >= ? AND passed = ?", since, false).Count(&failed).Error,
	}
	if err := read.Model(&models.Exam{}).Select("COALESCE(AVG(duration), 0)").Scan(&averageDuration).Error; err != nil {
		return nil, err
	}
	for _, err := range queries {
		if err != nil {
			return nil, err
		}
	}
	type issueRow struct {
		ExamID         string
		Count          int64
		LastOccurrence time.Time
	}
	var rows []issueRow
	if err := read.Model(&models.ExamResult{}).Select("exam_id, COUNT(*) AS count, MAX(taken_at) AS last_occurrence").
		Where("taken_at >= ? AND passed = ?", since, false).Group("exam_id").Order("count DESC").Limit(10).Scan(&rows).Error; err != nil {
		return nil, err
	}
	issues := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		issues = append(issues, gin.H{"examId": row.ExamID, "issue": fmt.Sprintf("%d failed attempts", row.Count), "timestamp": row.LastOccurrence.Format(time.RFC3339), "severity": examIssueSeverity(row.Count)})
	}
	successRate := 0.0
	if passed+failed > 0 {
		successRate = float64(passed) * 100 / float64(passed+failed)
	}
	return gin.H{"totalExams": total, "enabledExams": enabled, "passedAttempts": passed, "failedAttempts": failed, "averageDuration": averageDuration, "successRate": successRate, "recentIssues": issues}, nil
}

func examIssueSeverity(count int64) string {
	if count >= 20 {
		return "high"
	}
	if count >= 5 {
		return "medium"
	}
	return "low"
}

func querySecurityHealth(ctx context.Context, since time.Time) (gin.H, error) {
	read := db.ReadDB(ctx)
	var activeThreats, blockedIPs, suspicious, enabled2FA, totalUsers int64
	var criticalThreats, warningThreats int64
	queries := []error{
		read.Model(&models.SecurityEvent{}).Where("resolved = ?", false).Count(&activeThreats).Error,
		read.Model(&models.SecurityEvent{}).Where("resolved = ? AND severity = ?", false, "critical").Count(&criticalThreats).Error,
		read.Model(&models.SecurityEvent{}).Where("resolved = ? AND severity = ?", false, "warning").Count(&warningThreats).Error,
		read.Model(&models.BlacklistedIP{}).Where("is_permanent = ? OR expires_at > ?", true, time.Now()).Count(&blockedIPs).Error,
		read.Model(&models.LoginHistory{}).Where("created_at >= ? AND suspicious = ?", since, true).Count(&suspicious).Error,
		read.Model(&models.User{}).Where("two_factor_enabled = ?", true).Count(&enabled2FA).Error,
		read.Model(&models.User{}).Count(&totalUsers).Error,
	}
	for _, err := range queries {
		if err != nil {
			return nil, err
		}
	}
	type incidentRow struct {
		Type           string
		Count          int64
		LastOccurrence time.Time
	}
	var rows []incidentRow
	if err := read.Model(&models.SecurityEvent{}).Select("event_type AS type, COUNT(*) AS count, MAX(created_at) AS last_occurrence").Where("created_at >= ?", since).Group("event_type").Order("count DESC").Limit(10).Scan(&rows).Error; err != nil {
		return nil, err
	}
	incidents := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		incidents = append(incidents, gin.H{"type": row.Type, "count": row.Count, "lastOccurrence": row.LastOccurrence.Format(time.RFC3339)})
	}
	threatLevelValue := threatLevelBySeverity(criticalThreats, warningThreats, activeThreats)
	return gin.H{"threatLevel": threatLevelValue, "activeThreats": activeThreats, "blockedIPs": blockedIPs, "suspiciousActivities": suspicious, "twoFactorEnabled": enabled2FA, "twoFactorTotal": totalUsers, "recentIncidents": incidents}, nil
}

func threatLevelBySeverity(critical, warning, active int64) string {
	if critical > 0 {
		return "critical"
	}
	if warning > 0 {
		return "high"
	}
	return threatLevel(active)
}

func threatLevel(count int64) string {
	if count >= 20 {
		return "critical"
	}
	if count >= 10 {
		return "high"
	}
	if count > 0 {
		return "medium"
	}
	return "low"
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
