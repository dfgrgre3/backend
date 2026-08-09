package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// dashboardTopCourseSortKeys are the orderings the endpoint supports.
var dashboardTopCourseSortKeys = map[string]string{
	"value":           "value",
	"deltaPercentage": "value",
	"title":           `"LmsCourse".title`,
}

// GetDashboardTopCourses ranks courses by a chosen metric over a period.
//
// GET /api/admin/dashboard/top-courses
func GetDashboardTopCourses(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardViewTopCourses) {
		return
	}
	readDB, abort := safeReadDB(c)
	if abort {
		return
	}

	metric := strings.ToLower(strings.TrimSpace(c.Query("metric")))
	if metric == "" {
		api_response.Error(c, http.StatusBadRequest, "metric is required")
		return
	}
	allowedMetrics := map[string]bool{
		"enrollment": true, "completion": true, "revenue": true, "rating": true, "engagement": true,
	}
	if !allowedMetrics[metric] {
		api_response.Error(c, http.StatusBadRequest,
			"metric must be one of enrollment, completion, revenue, rating, engagement")
		return
	}
	// Revenue rankings expose commercial performance per course.
	if metric == "revenue" && !dashboardRequire(c, models.PermDashboardViewFinancialMetrics) {
		return
	}

	filters, ok := parseDashboardFilters(c)
	if !ok {
		return
	}
	page, ok := parseDashboardPage(c, dashboardTopCourseSortKeys, "value")
	if !ok {
		return
	}

	// `limit` is the simple first-render path; `pageSize` drives "show more".
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			api_response.Error(c, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if limit > dashboardMaxTopLimit {
			api_response.Error(c, http.StatusBadRequest,
				fmt.Sprintf("limit must not exceed %d", dashboardMaxTopLimit))
			return
		}
		page.PageSize = limit
	}

	category := strings.TrimSpace(firstNonEmpty(c.Query("category"), filters.CourseCategory))
	status := strings.TrimSpace(c.Query("status"))

	rows, total, err := queryTopCourses(readDB, metric, filters.StartDate, filters.EndDate,
		category, status, page)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load top courses")
		return
	}

	// The previous window is only needed when the caller asked to compare.
	previous := map[string]float64{}
	if filters.Compare {
		prevStart, prevEnd := filters.previousPeriod()
		prevRows, _, prevErr := queryTopCourses(readDB, metric, prevStart, prevEnd,
			category, status, dashboardPageParams{PageSize: dashboardMaxTopLimit, SortBy: "value", Direction: "desc"})
		if prevErr == nil {
			for _, row := range prevRows {
				previous[row.CourseID] = row.Value
			}
		}
	}

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		prevValue, hasPrev := previous[row.CourseID]
		_, _, percentage, _ := dashboardDelta(row.Value, prevValue, filters.Compare && hasPrev)

		item := gin.H{
			"courseId":        row.CourseID,
			"title":           row.Title,
			"category":        row.Category,
			"status":          row.Status,
			"value":           row.Value,
			"enrollmentCount": row.EnrollmentCount,
			"completionRate":  row.CompletionRate,
			"rating":          row.Rating,
			"lastUpdatedAt":   row.UpdatedAt,
			"actionUrl":       "/admin/courses/" + row.CourseID,
		}
		if filters.Compare {
			item["previousValue"] = prevValue
			item["deltaPercentage"] = percentage
		} else {
			item["previousValue"] = nil
			item["deltaPercentage"] = nil
		}
		// Revenue is only attached for financially-cleared callers, even when
		// ranking by another metric.
		if dashboardCan(c, models.PermDashboardViewFinancialMetrics) {
			item["revenue"] = row.Revenue
		}
		items = append(items, item)
	}

	api_response.Success(c, dashboardListResponse(items, total, page, gin.H{
		"metric": metric,
		"period": gin.H{"startDate": filters.StartDate, "endDate": filters.EndDate},
	}))
}

// topCourseRow is the aggregate shape produced by the ranking query.
type topCourseRow struct {
	CourseID        string    `gorm:"column:course_id"`
	Title           string    `gorm:"column:title"`
	Category        string    `gorm:"column:category"`
	Status          string    `gorm:"column:status"`
	Value           float64   `gorm:"column:value"`
	EnrollmentCount int64     `gorm:"column:enrollment_count"`
	CompletionRate  float64   `gorm:"column:completion_rate"`
	Revenue         float64   `gorm:"column:revenue"`
	Rating          float64   `gorm:"column:rating"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

// queryTopCourses aggregates course performance in a single grouped query.
// All metrics are computed for every row so the response stays consistent
// regardless of which one was used for ranking.
func queryTopCourses(conn *gorm.DB, metric string, from, to time.Time,
	category, status string, page dashboardPageParams) ([]topCourseRow, int64, error) {

	// Enrollment aggregates are windowed; rating and revenue join their own
	// subqueries so a course with no reviews or payments still ranks at zero.
	enrollmentWindow := `
		SELECT course_id,
		       COUNT(*) AS enrollment_count,
		       COUNT(*) FILTER (WHERE completed_at IS NOT NULL) AS completion_count
		FROM "LmsEnrollment"
		WHERE deleted_at IS NULL AND created_at >= ? AND created_at < ?
		GROUP BY course_id`

	ratingAgg := `
		SELECT course_id, COALESCE(AVG(rating), 0) AS avg_rating
		FROM "LmsReview"
		WHERE deleted_at IS NULL AND status = 'APPROVED'
		GROUP BY course_id`

	valueExpr := map[string]string{
		"enrollment": "COALESCE(e.enrollment_count, 0)",
		"completion": "COALESCE(e.completion_count, 0)",
		"rating":     "COALESCE(r.avg_rating, 0)",
		// Engagement blends reach with follow-through so a large but inactive
		// cohort does not outrank a smaller, highly engaged one.
		"engagement": "COALESCE(e.enrollment_count, 0) * (1 + COALESCE(e.completion_count, 0)::float / NULLIF(e.enrollment_count, 0))",
		"revenue":    "COALESCE(e.enrollment_count, 0) * COALESCE(avg_pay.amount, 0)",
	}[metric]

	query := conn.Table(`"LmsCourse"`).
		Joins(`LEFT JOIN (`+enrollmentWindow+`) e ON e.course_id = "LmsCourse".id`, from, to).
		Joins(`LEFT JOIN (` + ratingAgg + `) r ON r.course_id = "LmsCourse".id`).
		Joins(`LEFT JOIN LATERAL (
			SELECT COALESCE(AVG(amount), 0) AS amount FROM "Payment"
			WHERE deleted_at IS NULL AND status = 'completed'
		) avg_pay ON true`).
		Joins(`LEFT JOIN "LmsCourseCategory" cc ON cc.course_id = "LmsCourse".id`).
		Joins(`LEFT JOIN "LmsCategory" cat ON cat.id = cc.category_id`).
		Where(`"LmsCourse".deleted_at IS NULL`)

	if category != "" {
		query = query.Where("cat.slug = ? OR cat.id::text = ?", category, category)
	}
	if status != "" {
		query = query.Where(`"LmsCourse".status = ?`, status)
	}
	if page.Query != "" {
		needle := "%" + page.Query + "%"
		query = query.Where(`"LmsCourse".title ILIKE ? OR cat.name ILIKE ?`, needle, needle)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).
		Distinct(`"LmsCourse".id`).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	direction := "DESC"
	if page.Direction == "asc" {
		direction = "ASC"
	}
	orderBy := "value " + direction
	if page.SortBy == "title" {
		orderBy = `"LmsCourse".title ` + direction
	}

	var rows []topCourseRow
	err := query.
		Select(`"LmsCourse".id AS course_id,
			"LmsCourse".title AS title,
			COALESCE(cat.name, '') AS category,
			"LmsCourse".status AS status,
			"LmsCourse".updated_at AS updated_at,
			COALESCE(e.enrollment_count, 0) AS enrollment_count,
			COALESCE(e.completion_count, 0)::float / NULLIF(e.enrollment_count, 0) * 100 AS completion_rate,
			COALESCE(e.enrollment_count, 0) * COALESCE(avg_pay.amount, 0) AS revenue,
			COALESCE(r.avg_rating, 0) AS rating,
			` + valueExpr + ` AS value`).
		Group(`"LmsCourse".id, "LmsCourse".title, cat.name, "LmsCourse".status, "LmsCourse".updated_at,
			e.enrollment_count, e.completion_count, r.avg_rating, avg_pay.amount`).
		Order(orderBy).
		Offset(page.Offset).
		Limit(page.PageSize).
		Scan(&rows).Error

	return rows, total, err
}
