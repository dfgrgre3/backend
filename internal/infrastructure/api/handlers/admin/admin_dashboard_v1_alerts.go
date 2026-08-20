package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// dashboardAlertSortKeys maps API sort keys to physical columns.
var dashboardAlertSortKeys = map[string]string{
	"lastSeenAt":      "last_seen_at",
	"firstSeenAt":     "first_seen_at",
	"severity":        "severity",
	"occurrenceCount": "occurrence_count",
}

// GetDashboardAlerts lists persisted operational alerts.
//
// GET /api/admin/dashboard/alerts
func GetDashboardAlerts(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardViewAlerts) {
		return
	}
	readDB, abort := safeReadDB(c)
	if abort {
		return
	}
	page, ok := parseDashboardPage(c, dashboardAlertSortKeys, "lastSeenAt")
	if !ok {
		return
	}

	query := readDB.Model(&models.DashboardAlert{}).Where("deleted_at IS NULL")

	if severity := strings.ToLower(strings.TrimSpace(c.Query("severity"))); severity != "" {
		if !containsString(models.DashboardAlertSeverities(), severity) {
			api_response.Error(c, http.StatusBadRequest, fmt.Sprintf(
				"severity must be one of %s", strings.Join(models.DashboardAlertSeverities(), ", ")))
			return
		}
		query = query.Where("severity = ?", severity)
	}

	if state := strings.ToLower(strings.TrimSpace(c.Query("state"))); state != "" {
		if !containsString(models.DashboardAlertStates(), state) {
			api_response.Error(c, http.StatusBadRequest, fmt.Sprintf(
				"state must be one of %s", strings.Join(models.DashboardAlertStates(), ", ")))
			return
		}
		query = query.Where("state = ?", state)
	}

	if category := strings.TrimSpace(c.Query("category")); category != "" {
		query = query.Where("category = ?", category)
	}
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		query = query.Where("source = ?", source)
	}

	if raw := strings.TrimSpace(c.Query("fromDate")); raw != "" {
		parsed, valid := parseDashboardDate(raw)
		if !valid {
			api_response.Error(c, http.StatusBadRequest, "fromDate must be a valid date")
			return
		}
		query = query.Where("last_seen_at >= ?", parsed)
	}
	if raw := strings.TrimSpace(c.Query("toDate")); raw != "" {
		parsed, valid := parseDashboardDate(raw)
		if !valid {
			api_response.Error(c, http.StatusBadRequest, "toDate must be a valid date")
			return
		}
		query = query.Where("last_seen_at <= ?", parsed)
	}

	if page.Query != "" {
		needle := "%" + page.Query + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ? OR source ILIKE ?", needle, needle, needle)
	}

	// An alert may carry its own permission requirement; hide those the caller
	// cannot act on rather than leaking the existence of the condition.
	query = query.Where("required_permission IS NULL OR required_permission IN ?", dashboardVisiblePermissions(c))

	var total int64
	query.Count(&total)

	var alerts []models.DashboardAlert
	if err := query.Order(page.orderClause(dashboardAlertSortKeys)).
		Offset(page.Offset).Limit(page.PageSize).Find(&alerts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load alerts")
		return
	}

	api_response.Success(c, dashboardListResponse(alerts, total, page, nil))
}

// AcknowledgeDashboardAlert records that an admin has seen an alert.
//
// POST /api/admin/dashboard/alerts/:alertId/acknowledge
func AcknowledgeDashboardAlert(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardAcknowledgeAlerts) {
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

	alertID := strings.TrimSpace(c.Param("alertId"))
	if alertID == "" {
		api_response.Error(c, http.StatusBadRequest, "alertId is required")
		return
	}

	var body struct {
		Note string `json:"note"`
	}
	// The body is optional; only reject input that is present but malformed.
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid request body")
			return
		}
	}
	if len(body.Note) > 1000 {
		api_response.Error(c, http.StatusUnprocessableEntity, "note must not exceed 1000 characters")
		return
	}

	var alert models.DashboardAlert
	if err := conn.Where("id = ? AND deleted_at IS NULL", alertID).First(&alert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Alert not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to load alert")
		return
	}

	if alert.State == models.DashboardAlertStateAcknowledged {
		api_response.Error(c, http.StatusConflict, "Alert is already acknowledged")
		return
	}
	if alert.State == models.DashboardAlertStateResolved {
		api_response.Error(c, http.StatusConflict, "Alert is already resolved")
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"state":           models.DashboardAlertStateAcknowledged,
		"acknowledged_by": callerID,
		"acknowledged_at": now,
		"updated_at":      now,
	}
	if body.Note != "" {
		updates["acknowledge_note"] = body.Note
	}

	if err := conn.Model(&models.DashboardAlert{}).
		Where("id = ? AND state <> ?", alertID, models.DashboardAlertStateAcknowledged).
		Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to acknowledge alert")
		return
	}

	api_response.Success(c, gin.H{
		"alertId":        alertID,
		"state":          models.DashboardAlertStateAcknowledged,
		"acknowledgedBy": callerID,
		"acknowledgedAt": now,
	})
}

// dashboardVisiblePermissions returns the caller's grants for use in the alert
// visibility filter. A sentinel is returned when empty so the generated SQL
// stays valid and matches nothing.
func dashboardVisiblePermissions(c *gin.Context) []string {
	grants := dashboardGrants(c)
	if len(grants) == 0 {
		return []string{""}
	}
	return grants
}

func containsString(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}

// upsertDashboardAlert records an operational alert, collapsing repeats of the
// same condition onto one row via dedupeKey. It is safe to call from any
// background job or handler that detects a problem worth surfacing.
