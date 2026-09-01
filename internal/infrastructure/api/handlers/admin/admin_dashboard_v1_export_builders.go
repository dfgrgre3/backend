package admin

import (
	"context"
	"fmt"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
			row = append(row, string(log.IP), log.UserAgent)
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
