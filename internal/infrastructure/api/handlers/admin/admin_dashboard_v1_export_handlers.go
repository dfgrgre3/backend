package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
