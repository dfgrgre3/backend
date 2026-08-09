package admin

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// dashboardActivitySortKeys maps API sort keys to physical columns.
var dashboardActivitySortKeys = map[string]string{
	"occurredAt": "\"AuditLog\".created_at",
	"action":     "\"AuditLog\".event_type",
}

// dashboardHighRiskActions are audit events that change security posture or
// move money, and are therefore surfaced as high risk in the activity feed.
var dashboardHighRiskActions = []string{
	"role.change", "permission.change", "user.delete", "user.impersonate",
	"payment.refund", "settings.update", "admin.create",
}

// GetDashboardRecentActivities returns the administrative audit feed.
//
// GET /api/admin/dashboard/recent-activities
func GetDashboardRecentActivities(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardViewRecentActivity) {
		return
	}
	readDB, abort := safeReadDB(c)
	if abort {
		return
	}
	page, ok := parseDashboardPage(c, dashboardActivitySortKeys, "occurredAt")
	if !ok {
		return
	}

	// Joining User lets the feed search and display actor names without an
	// N+1 lookup per row.
	query := readDB.Model(&models.AuditLog{}).
		Joins(`LEFT JOIN "User" ON "User".id = "AuditLog".user_id`)

	if actorID := strings.TrimSpace(c.Query("actorId")); actorID != "" {
		query = query.Where("\"AuditLog\".user_id = ?", actorID)
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		query = query.Where("\"AuditLog\".event_type = ?", action)
	}
	if entityType := strings.TrimSpace(c.Query("entityType")); entityType != "" {
		query = query.Where("\"AuditLog\".resource = ?", entityType)
	}
	if entityID := strings.TrimSpace(c.Query("entityId")); entityID != "" {
		query = query.Where("\"AuditLog\".resource_id = ?", entityID)
	}

	if raw := strings.TrimSpace(c.Query("fromDate")); raw != "" {
		parsed, valid := parseDashboardDate(raw)
		if !valid {
			api_response.Error(c, http.StatusBadRequest, "fromDate must be a valid date")
			return
		}
		query = query.Where("\"AuditLog\".created_at >= ?", parsed)
	}
	if raw := strings.TrimSpace(c.Query("toDate")); raw != "" {
		parsed, valid := parseDashboardDate(raw)
		if !valid {
			api_response.Error(c, http.StatusBadRequest, "toDate must be a valid date")
			return
		}
		query = query.Where("\"AuditLog\".created_at <= ?", parsed)
	}

	if riskLevel := strings.ToLower(strings.TrimSpace(c.Query("riskLevel"))); riskLevel != "" {
		switch riskLevel {
		case "high":
			query = query.Where("\"AuditLog\".event_type IN ?", dashboardHighRiskActions)
		case "low":
			query = query.Where("\"AuditLog\".event_type NOT IN ?", dashboardHighRiskActions)
		default:
			api_response.Error(c, http.StatusBadRequest, "riskLevel must be high or low")
			return
		}
	}

	if page.Query != "" {
		needle := "%" + page.Query + "%"
		query = query.Where(
			`"AuditLog".event_type ILIKE ? OR "AuditLog".resource ILIKE ? OR "User".name ILIKE ? OR "User".email ILIKE ?`,
			needle, needle, needle, needle)
	}

	var total int64
	query.Count(&total)

	type activityRow struct {
		models.AuditLog
		ActorName  string `gorm:"column:actor_name"`
		ActorEmail string `gorm:"column:actor_email"`
		ActorRole  string `gorm:"column:actor_role"`
	}

	var rows []activityRow
	if err := query.
		Select(`"AuditLog".*, "User".name AS actor_name, "User".email AS actor_email, "User".role AS actor_role`).
		Order(page.orderClause(dashboardActivitySortKeys)).
		Offset(page.Offset).Limit(page.PageSize).
		Scan(&rows).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load recent activities")
		return
	}

	// Network and device fields identify an individual, so they are only
	// included for callers cleared for sensitive metrics.
	canSeeSensitive := dashboardCan(c, models.PermDashboardViewSensitive)
	canViewDetails := dashboardCan(c, models.PermAuditLogsView)

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		action := firstNonEmpty(row.EventType, row.Action)
		item := gin.H{
			"id":                row.ID,
			"occurredAt":        row.CreatedAt,
			"actorId":           stringOrEmpty(row.UserID),
			"actorName":         firstNonEmpty(row.ActorName, row.ActorEmail),
			"actorType":         firstNonEmpty(row.ActorRole, "SYSTEM"),
			"action":            action,
			"entityType":        row.Resource,
			"entityId":          row.ResourceID,
			"entityDisplayName": firstNonEmpty(row.Resource, action),
			"summary":           dashboardActivitySummary(action, row.Resource),
			"riskLevel":         dashboardRiskLevel(action),
			"canViewDetails":    canViewDetails,
		}
		if canSeeSensitive {
			item["sourceIp"] = row.IP
			item["userAgent"] = row.UserAgent
			item["metadata"] = row.Metadata
		} else {
			item["sourceIp"] = nil
			item["userAgent"] = nil
			item["metadata"] = nil
		}
		items = append(items, item)
	}

	api_response.Success(c, dashboardListResponse(items, total, page, nil))
}

// dashboardActivitySummary renders a short human-readable line for the feed.
func dashboardActivitySummary(action, resource string) string {
	switch {
	case action == "" && resource == "":
		return "نشاط غير محدد"
	case resource == "":
		return action
	default:
		return action + " · " + resource
	}
}

// dashboardRiskLevel classifies an audit action for the feed badge.
func dashboardRiskLevel(action string) string {
	for _, risky := range dashboardHighRiskActions {
		if strings.EqualFold(action, risky) {
			return "high"
		}
	}
	return "low"
}
