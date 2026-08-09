package admin

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"thanawy-backend/internal/infrastructure/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// dashboardExportScopes هي النطاقات القابلة للتصدير مقابل الإذن اللازم لكل منها.
// التصدير لا يجوز أن يتجاوز صلاحيات العرض، لذا يُفحص إذن النطاق إضافة إلى
// إذن التصدير العام.
var dashboardExportScopes = map[string]string{
	"summary":          models.PermDashboardViewKPIs,
	"pendingActions":   models.PermDashboardViewPendingItems,
	"alerts":           models.PermDashboardViewAlerts,
	"recentActivities": models.PermDashboardViewRecentActivity,
	"topCourses":       models.PermDashboardViewTopCourses,
	"systemHealth":     models.PermDashboardViewSystemHealth,
}

// dashboardExportFormats هي الصيغ المدعومة فعليًا. الصيغ الأخرى تُرفض بدل
// إنشاء مهمة لن تكتمل أبدًا.
var dashboardExportFormats = map[string]bool{"csv": true}

// dashboardExportMaxRangeDays يحدّ نطاق التصدير لتفادي استهلاك ذاكرة مفرط.
const dashboardExportMaxRangeDays = 366

// ExportDashboardReport ينشئ مهمة تصدير وينفّذها ثم يعيد رابط التنزيل.
//
// POST /api/admin/dashboard/export
func ExportDashboardReport(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardExport) {
		return
	}
	conn, abort := safeDB(c)
	if abort {
		return
	}
	callerID, authenticated := getAuthenticatedUserID(c)
	if !authenticated {
		return
	}

	var body struct {
		ExportScope string `json:"exportScope"`
		FileFormat  string `json:"fileFormat"`
		DateRange   struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
		} `json:"dateRange"`
		Filters                json.RawMessage `json:"filters"`
		IncludeChartsMetadata  bool            `json:"includeChartsMetadata"`
		IncludeSensitiveFields bool            `json:"includeSensitiveFields"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	scope := strings.TrimSpace(body.ExportScope)
	requiredPermission, known := dashboardExportScopes[scope]
	if !known {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf(
			"exportScope must be one of %s", strings.Join(sortedKeys(dashboardExportScopes), ", ")))
		return
	}
	if !dashboardRequire(c, requiredPermission) {
		return
	}

	format := strings.ToLower(strings.TrimSpace(body.FileFormat))
	if format == "" {
		format = "csv"
	}
	if !dashboardExportFormats[format] {
		api_response.Error(c, http.StatusBadRequest, "fileFormat must be csv")
		return
	}

	// الحقول الحساسة لا تُصدَّر إلا لمن يملك إذنها صراحةً.
	includeSensitive := body.IncludeSensitiveFields
	if includeSensitive && !dashboardCan(c, models.PermDashboardViewSensitive) {
		api_response.Error(c, http.StatusForbidden,
			"Exporting sensitive fields requires dashboard:view_sensitive_metrics")
		return
	}

	now := time.Now()
	endDate := now
	startDate := now.AddDate(0, 0, -30)
	if body.DateRange.StartDate != "" || body.DateRange.EndDate != "" {
		parsedStart, okStart := parseDashboardDate(body.DateRange.StartDate)
		parsedEnd, okEnd := parseDashboardDate(body.DateRange.EndDate)
		if !okStart || !okEnd {
			api_response.Error(c, http.StatusBadRequest, "dateRange must contain valid startDate and endDate")
			return
		}
		if parsedEnd.Before(parsedStart) {
			api_response.Error(c, http.StatusBadRequest, "startDate must be before or equal to endDate")
			return
		}
		if parsedEnd.Sub(parsedStart) > time.Duration(dashboardExportMaxRangeDays)*24*time.Hour {
			api_response.Error(c, http.StatusUnprocessableEntity, fmt.Sprintf(
				"export range must not exceed %d days", dashboardExportMaxRangeDays))
			return
		}
		startDate, endDate = parsedStart, parsedEnd
	}

	job := models.DashboardExportJob{
		UserID:                callerID,
		ExportScope:           scope,
		FileFormat:            format,
		Status:                models.DashboardExportStatusProcessing,
		Progress:              0,
		Filters:               body.Filters,
		IncludeChartsMetadata: body.IncludeChartsMetadata,
		IncludeSensitive:      includeSensitive,
		StartDate:             &startDate,
		EndDate:               &endDate,
	}
	if err := conn.Create(&job).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create export job")
		return
	}

	rows, rowCount, err := buildDashboardExportRows(c, scope, startDate, endDate, includeSensitive)
	if err != nil {
		markDashboardExportFailed(conn, job.ID, "export_build_failed")
		api_response.Error(c, http.StatusInternalServerError, "Failed to build export")
		return
	}

	fileURL, err := storeDashboardExport(c.Request.Context(), job.ID, scope, rows)
	if err != nil {
		markDashboardExportFailed(conn, job.ID, "export_storage_unavailable")
		api_response.Error(c, http.StatusServiceUnavailable, "Export storage is unavailable")
		return
	}

	expiresAt := time.Now().Add(dashboardExportTTL)
	completedAt := time.Now()
	conn.Model(&models.DashboardExportJob{}).Where("id = ?", job.ID).
		Updates(map[string]interface{}{
			"status":       models.DashboardExportStatusCompleted,
			"progress":     100,
			"row_count":    rowCount,
			"file_url":     fileURL,
			"expires_at":   expiresAt,
			"completed_at": completedAt,
			"updated_at":   completedAt,
		})

	LogAudit(c, "dashboard_export_created", "dashboard_export_job", job.ID, gin.H{
		"scope":             scope,
		"format":            format,
		"row_count":         rowCount,
		"range_days":        int(endDate.Sub(startDate).Hours() / 24),
		"include_sensitive": includeSensitive,
	})

	api_response.Created(c, gin.H{
		"exportJobId": job.ID,
		"status":      models.DashboardExportStatusCompleted,
		"downloadUrl": fileURL,
		"expiresAt":   expiresAt,
		"rowCount":    rowCount,
	})
}

// GetDashboardExportStatus يتابع حالة مهمة تصدير.
//
// GET /api/admin/dashboard/export/:exportJobId
func GetDashboardExportStatus(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardExport) {
		return
	}
	conn, abort := safeDB(c)
	if abort {
		return
	}
	callerID, authenticated := getAuthenticatedUserID(c)
	if !authenticated {
		return
	}

	jobID := strings.TrimSpace(c.Param("exportJobId"))
	if jobID == "" {
		api_response.Error(c, http.StatusBadRequest, "exportJobId is required")
		return
	}

	var job models.DashboardExportJob
	if err := conn.Where("id = ?", jobID).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Export job not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to load export job")
		return
	}

	// المهمة ملك صاحبها؛ لا يطّلع عليها غيره إلا بإذن إدارة اللوحة.
	if job.UserID != callerID && !dashboardCan(c, models.PermDashboardManage) {
		api_response.Error(c, http.StatusForbidden, "You cannot access this export job")
		return
	}

	payload := gin.H{
		"exportJobId": job.ID,
		"status":      job.Status,
		"progress":    job.Progress,
		"expiresAt":   job.ExpiresAt,
		"errorCode":   job.ErrorCode,
	}

	// رابط التنزيل يُحجب بعد انتهاء الصلاحية بدل إعادة رابط ميت.
	if job.IsExpired(time.Now()) {
		payload["status"] = models.DashboardExportStatusExpired
		payload["fileUrl"] = nil
		c.JSON(http.StatusGone, gin.H{"success": false, "data": payload, "error": "Export file has expired"})
		return
	}
	payload["fileUrl"] = job.FileURL

	api_response.Success(c, payload)
}

// markDashboardExportFailed يسجّل سبب الفشل على المهمة.
func markDashboardExportFailed(conn *gorm.DB, jobID, code string) {
	conn.Model(&models.DashboardExportJob{}).Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":     models.DashboardExportStatusFailed,
			"error_code": code,
			"updated_at": time.Now(),
		})
}

// storeDashboardExport يكتب ملف CSV إلى التخزين ويعيد رابطه.
func storeDashboardExport(ctx context.Context, jobID, scope string, rows [][]string) (string, error) {
	if storage.GlobalStorage == nil {
		return "", fmt.Errorf("storage is not configured")
	}

	var buffer bytes.Buffer
	// BOM حتى تفتح Excel النص العربي بترميز صحيح.
	buffer.WriteString("\ufeff")
	writer := csv.NewWriter(&buffer)
	if err := writer.WriteAll(rows); err != nil {
		return "", err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("exports/dashboard/%s-%s.csv", scope, jobID)
	payload := buffer.Bytes()
	return storage.GlobalStorage.Upload(ctx, filename,
		bytes.NewReader(payload), int64(len(payload)), "text/csv; charset=utf-8")
}

// buildDashboardExportRows يبني صفوف التقرير حسب النطاق المطلوب.
func buildDashboardExportRows(c *gin.Context, scope string, from, to time.Time,
	includeSensitive bool) ([][]string, int64, error) {

	readDB := readDashboardDB()
	if readDB == nil {
		return nil, 0, fmt.Errorf("database is unavailable")
	}

	switch scope {
	case "summary":
		return buildSummaryExportRows(c, readDB, from, to)
	case "alerts":
		return buildAlertsExportRows(readDB)
	case "pendingActions":
		return buildPendingExportRows(readDB)
	case "topCourses":
		return buildTopCoursesExportRows(readDB, from, to)
	case "recentActivities":
		return buildActivitiesExportRows(readDB, from, to, includeSensitive)
	case "systemHealth":
		return buildSystemHealthExportRows(c)
	default:
		return nil, 0, fmt.Errorf("unsupported scope %q", scope)
	}
}

func buildSummaryExportRows(c *gin.Context, conn *gorm.DB, from, to time.Time) ([][]string, int64, error) {
	rows := [][]string{{"metricKey", "title", "value", "unit"}}
	filters := dashboardFilters{StartDate: from, EndDate: to}

	var count int64
	for _, spec := range dashboardMetricSpecs() {
		if !dashboardCan(c, spec.Permission) {
			continue
		}
		value := spec.Compute(conn, from, to, filters)
		rows = append(rows, []string{
			spec.Key, spec.Title, strconv.FormatFloat(value, 'f', 2, 64), spec.Unit,
		})
		count++
	}
	return rows, count, nil
}

func buildAlertsExportRows(conn *gorm.DB) ([][]string, int64, error) {
	var alerts []models.DashboardAlert
	if err := conn.Order("last_seen_at DESC").Limit(5000).Find(&alerts).Error; err != nil {
		return nil, 0, err
	}
	rows := [][]string{{"id", "severity", "category", "title", "state", "occurrenceCount", "firstSeenAt", "lastSeenAt"}}
	for _, a := range alerts {
		rows = append(rows, []string{
			a.ID, a.Severity, a.Category, a.Title, a.State,
			strconv.FormatInt(a.OccurrenceCount, 10),
			a.FirstSeenAt.Format(time.RFC3339), a.LastSeenAt.Format(time.RFC3339),
		})
	}
	return rows, int64(len(alerts)), nil
}

func buildPendingExportRows(conn *gorm.DB) ([][]string, int64, error) {
	rows := [][]string{{"type", "id", "title", "status", "createdAt"}}
	var count int64

	var courses []models.LmsCourse
	conn.Where("deleted_at IS NULL AND status = ?", "REVIEW").
		Order(createdAtDescSort).Limit(2000).Find(&courses)
	for _, course := range courses {
		rows = append(rows, []string{
			"course_review", course.ID.String(), course.Title, "pending",
			course.CreatedAt.Format(time.RFC3339),
		})
		count++
	}

	var tickets []models.SupportTicket
	conn.Where("deleted_at IS NULL AND status IN ?", []string{"open", "escalated"}).
		Order(createdAtDescSort).Limit(2000).Find(&tickets)
	for _, ticket := range tickets {
		rows = append(rows, []string{
			"support_ticket", ticket.ID, ticket.Subject, ticket.Status,
			ticket.CreatedAt.Format(time.RFC3339),
		})
		count++
	}

	return rows, count, nil
}

func buildTopCoursesExportRows(conn *gorm.DB, from, to time.Time) ([][]string, int64, error) {
	page := dashboardPageParams{Page: 1, PageSize: dashboardMaxTopLimit, SortBy: "value", Direction: "desc"}
	courses, _, err := queryTopCourses(conn, "enrollment", from, to, "", "", page)
	if err != nil {
		return nil, 0, err
	}
	rows := [][]string{{"courseId", "title", "category", "status", "enrollmentCount", "completionRate", "rating"}}
	for _, course := range courses {
		rows = append(rows, []string{
			course.CourseID, course.Title, course.Category, course.Status,
			strconv.FormatInt(course.EnrollmentCount, 10),
			strconv.FormatFloat(course.CompletionRate, 'f', 2, 64),
			strconv.FormatFloat(course.Rating, 'f', 2, 64),
		})
	}
	return rows, int64(len(courses)), nil
}

func buildActivitiesExportRows(conn *gorm.DB, from, to time.Time, includeSensitive bool) ([][]string, int64, error) {
	var logs []models.AuditLog
	if err := conn.Where("created_at >= ? AND created_at <= ?", from, to).
		Order(createdAtDescSort).Limit(5000).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	header := []string{"id", "occurredAt", "actorId", "action", "entityType", "entityId", "riskLevel"}
	if includeSensitive {
		header = append(header, "sourceIp", "userAgent")
	}
	rows := [][]string{header}

	for _, log := range logs {
		action := firstNonEmpty(log.EventType, log.Action)
		row := []string{
			log.ID, log.CreatedAt.Format(time.RFC3339), stringOrEmpty(log.UserID),
			action, log.Resource, log.ResourceID, dashboardRiskLevel(action),
		}
		if includeSensitive {
			row = append(row, log.IP, log.UserAgent)
		}
		rows = append(rows, row)
	}
	return rows, int64(len(logs)), nil
}

func buildSystemHealthExportRows(c *gin.Context) ([][]string, int64, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	checks := dashboardServiceChecks()
	rows := [][]string{{"serviceKey", "serviceName", "status", "latencyMs", "checkedAt"}}
	now := time.Now()

	for _, check := range checks {
		start := time.Now()
		err := check.Probe(ctx)
		latency := float64(time.Since(start).Microseconds()) / 1000
		status := "healthy"
		if err != nil {
			status = "unhealthy"
		}
		rows = append(rows, []string{
			check.Key, check.Name, status,
			strconv.FormatFloat(latency, 'f', 2, 64),
			now.Format(time.RFC3339),
		})
	}
	return rows, int64(len(checks)), nil
}
