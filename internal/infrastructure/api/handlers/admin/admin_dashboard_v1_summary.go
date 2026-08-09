package admin

import (
	"fmt"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// dashboardMetricSpec declares one KPI card: how to compute it and who may see it.
// Keeping the declaration and the gate together means a new metric cannot be
// added without an explicit visibility decision.
type dashboardMetricSpec struct {
	Key             string
	Title           string
	Unit            string
	Permission      string
	DrillDownEntity string
	DrillDownFilter string
	Compute         func(conn *gorm.DB, from, to time.Time, f dashboardFilters) float64
	// Series resolves the whole bucketed range in a single grouped query and
	// returns values keyed by truncated bucket date (YYYY-MM-DD). The
	// time-series endpoint used to call Compute once per bucket, which meant up
	// to 400 sequential round-trips for one chart.
	Series func(conn *gorm.DB, from, to time.Time, granularity string, f dashboardFilters) map[string]float64
	// PointInTime marks metrics that describe the present moment (for example
	// "courses currently in review"). They have no historical dimension, so the
	// value is resolved once and repeated across the series.
	PointInTime bool
}

// GetDashboardSummary returns the KPI cards for the top of the dashboard.
// Metrics the caller lacks permission for are omitted from the api_response.
//
// GET /api/admin/dashboard/summary
func GetDashboardSummary(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardAccess) {
		return
	}
	readDB, abort := safeReadDB(c)
	if abort {
		return
	}
	filters, ok := parseDashboardFilters(c)
	if !ok {
		return
	}

	specs := dashboardMetricSpecs()
	visible := make([]dashboardMetricSpec, 0, len(specs))
	for _, spec := range specs {
		if dashboardCan(c, spec.Permission) {
			visible = append(visible, spec)
		}
	}

	prevStart, prevEnd := filters.previousPeriod()
	type metricResult struct {
		current  float64
		previous float64
	}
	results := make([]metricResult, len(visible))

	var wg sync.WaitGroup
	for i, spec := range visible {
		wg.Add(1)
		go func(idx int, s dashboardMetricSpec) {
			defer wg.Done()
			results[idx].current = s.Compute(readDB, filters.StartDate, filters.EndDate, filters)
			if filters.Compare {
				results[idx].previous = s.Compute(readDB, prevStart, prevEnd, filters)
			}
		}(i, spec)
	}
	wg.Wait()

	now := time.Now()
	metrics := make([]gin.H, 0, len(visible))
	for i, spec := range visible {
		previous, delta, percentage, direction := dashboardDelta(results[i].current, results[i].previous, filters.Compare)
		metrics = append(metrics, gin.H{
			"metricKey":            spec.Key,
			"title":                spec.Title,
			"value":                results[i].current,
			"unit":                 spec.Unit,
			"previousValue":        previous,
			"deltaValue":           delta,
			"deltaPercentage":      percentage,
			"trendDirection":       direction,
			"updatedAt":            now,
			"isCached":             false,
			"visibilityPermission": spec.Permission,
			"drillDownEntityType":  spec.DrillDownEntity,
			"drillDownFilter":      spec.DrillDownFilter,
		})
	}

	api_response.Success(c, gin.H{
		"metrics": metrics,
		"period": gin.H{
			"startDate":   filters.StartDate,
			"endDate":     filters.EndDate,
			"granularity": filters.Granularity,
			"compare":     filters.Compare,
		},
		"currency": filters.Currency,
	})
}

// dashboardMetricSpecs is the single registry of dashboard KPIs. The time-series
// endpoint validates metricKey against this same list.
func dashboardMetricSpecs() []dashboardMetricSpec {
	return []dashboardMetricSpec{
		{
			Key: "active_users", Title: "المستخدمون النشطون", Unit: "user",
			Permission:      models.PermDashboardViewKPIs,
			DrillDownEntity: "user", DrillDownFilter: "status=ACTIVE",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				return countIn(conn, &models.User{}, "updated_at", from, to, "deleted_at IS NULL")
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return countSeries(conn, &models.User{}, "updated_at", from, to, g, "deleted_at IS NULL")
			},
		},
		{
			Key: "new_users", Title: "المستخدمون الجدد", Unit: "user",
			Permission:      models.PermDashboardViewKPIs,
			DrillDownEntity: "user",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				return countIn(conn, &models.User{}, "created_at", from, to, "deleted_at IS NULL")
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return countSeries(conn, &models.User{}, "created_at", from, to, g, "deleted_at IS NULL")
			},
		},
		{
			Key: "active_instructors", Title: "المدربون النشطون", Unit: "user",
			Permission:      models.PermDashboardViewKPIs,
			DrillDownEntity: "instructor",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				return countIn(conn, &models.User{}, "updated_at", from, to,
					"deleted_at IS NULL AND role = 'TEACHER'")
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return countSeries(conn, &models.User{}, "updated_at", from, to, g,
					"deleted_at IS NULL AND role = 'TEACHER'")
			},
		},
		{
			Key: "published_courses", Title: "الدورات المنشورة", Unit: "course",
			Permission:      models.PermDashboardViewContentMetrics,
			DrillDownEntity: "course", DrillDownFilter: "status=PUBLISHED",
			PointInTime: true,
			Compute: func(conn *gorm.DB, _, _ time.Time, _ dashboardFilters) float64 {
				return countAll(conn, &models.LmsCourse{}, "deleted_at IS NULL AND status = 'PUBLISHED'")
			},
		},
		{
			Key: "courses_in_review", Title: "دورات قيد المراجعة", Unit: "course",
			Permission:      models.PermDashboardViewContentMetrics,
			DrillDownEntity: "course", DrillDownFilter: "status=REVIEW",
			PointInTime: true,
			Compute: func(conn *gorm.DB, _, _ time.Time, _ dashboardFilters) float64 {
				return countAll(conn, &models.LmsCourse{}, "deleted_at IS NULL AND status = 'REVIEW'")
			},
		},
		{
			Key: "new_enrollments", Title: "التسجيلات الجديدة", Unit: "enrollment",
			Permission:      models.PermDashboardViewLearningMetrics,
			DrillDownEntity: "enrollment",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				return countIn(conn, &models.LmsEnrollment{}, "created_at", from, to, "deleted_at IS NULL")
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return countSeries(conn, &models.LmsEnrollment{}, "created_at", from, to, g, "deleted_at IS NULL")
			},
		},
		{
			Key: "course_completions", Title: "الدورات المكتملة", Unit: "enrollment",
			Permission:      models.PermDashboardViewLearningMetrics,
			DrillDownEntity: "enrollment",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				return countIn(conn, &models.LmsEnrollment{}, "completed_at", from, to,
					"deleted_at IS NULL AND completed_at IS NOT NULL")
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return countSeries(conn, &models.LmsEnrollment{}, "completed_at", from, to, g,
					"deleted_at IS NULL AND completed_at IS NOT NULL")
			},
		},
		{
			Key: "study_minutes", Title: "دقائق التعلم", Unit: "minute",
			Permission: models.PermDashboardViewLearningMetrics,
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				var total float64
				conn.Model(&models.StudySession{}).
					Select("COALESCE(SUM(duration_min), 0)").
					Where("deleted_at IS NULL AND start_time >= ? AND start_time < ?", from, to).
					Scan(&total)
				return total
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return aggregateSeries(conn, &models.StudySession{}, "start_time", "SUM(duration_min)",
					from, to, g, "deleted_at IS NULL")
			},
		},
		{
			Key: "exam_attempts", Title: "محاولات الاختبارات", Unit: "attempt",
			Permission:      models.PermDashboardViewLearningMetrics,
			DrillDownEntity: "exam_result",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				return countIn(conn, &models.ExamResult{}, "created_at", from, to, "deleted_at IS NULL")
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return countSeries(conn, &models.ExamResult{}, "created_at", from, to, g, "deleted_at IS NULL")
			},
		},
		{
			Key: "open_tickets", Title: "تذاكر الدعم المفتوحة", Unit: "ticket",
			Permission:      models.PermDashboardViewSupportMetrics,
			DrillDownEntity: "support_ticket", DrillDownFilter: "status=open",
			PointInTime: true,
			Compute: func(conn *gorm.DB, _, _ time.Time, _ dashboardFilters) float64 {
				return countAll(conn, &models.SupportTicket{},
					"deleted_at IS NULL AND status IN ('open','in_progress','escalated')")
			},
		},
		{
			Key: "total_revenue", Title: "إجمالي الإيرادات", Unit: "currency",
			Permission:      models.PermDashboardViewFinancialMetrics,
			DrillDownEntity: "payment",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				var total float64
				conn.Model(&models.Payment{}).
					Select("COALESCE(SUM(amount), 0)").
					Where("deleted_at IS NULL AND status = ? AND created_at >= ? AND created_at < ?",
						models.PaymentCompleted, from, to).
					Scan(&total)
				return total
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return aggregateSeries(conn, &models.Payment{}, "created_at", "SUM(amount)", from, to, g,
					"deleted_at IS NULL AND status = '"+string(models.PaymentCompleted)+"'")
			},
		},
		{
			Key: "failed_payments", Title: "المدفوعات الفاشلة", Unit: "payment",
			Permission:      models.PermDashboardViewFinancialMetrics,
			DrillDownEntity: "payment", DrillDownFilter: "status=failed",
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				return countIn(conn, &models.Payment{}, "created_at", from, to,
					"deleted_at IS NULL AND status = 'failed'")
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return countSeries(conn, &models.Payment{}, "created_at", from, to, g,
					"deleted_at IS NULL AND status = 'failed'")
			},
		},
		{
			Key: "average_order_value", Title: "متوسط قيمة الطلب", Unit: "currency",
			Permission: models.PermDashboardViewFinancialMetrics,
			Compute: func(conn *gorm.DB, from, to time.Time, _ dashboardFilters) float64 {
				var avg float64
				conn.Model(&models.Payment{}).
					Select("COALESCE(AVG(amount), 0)").
					Where("deleted_at IS NULL AND status = ? AND created_at >= ? AND created_at < ?",
						models.PaymentCompleted, from, to).
					Scan(&avg)
				return avg
			},
			Series: func(conn *gorm.DB, from, to time.Time, g string, _ dashboardFilters) map[string]float64 {
				return aggregateSeries(conn, &models.Payment{}, "created_at", "AVG(amount)", from, to, g,
					"deleted_at IS NULL AND status = '"+string(models.PaymentCompleted)+"'")
			},
		},
		{
			Key: "pending_items", Title: "المهام المعلقة", Unit: "item",
			Permission:  models.PermDashboardViewPendingItems,
			PointInTime: true,
			Compute: func(conn *gorm.DB, _, _ time.Time, _ dashboardFilters) float64 {
				return countAll(conn, &models.LmsCourse{}, "deleted_at IS NULL AND status = 'REVIEW'") +
					countAll(conn, &models.SupportTicket{},
						"deleted_at IS NULL AND status IN ('open','escalated')")
			},
		},
		{
			Key: "open_alerts", Title: "التنبيهات غير المعالجة", Unit: "alert",
			Permission:      models.PermDashboardViewAlerts,
			DrillDownEntity: "alert", DrillDownFilter: "state=open",
			PointInTime: true,
			Compute: func(conn *gorm.DB, _, _ time.Time, _ dashboardFilters) float64 {
				return countAll(conn, &models.DashboardAlert{}, "deleted_at IS NULL AND state = 'open'")
			},
		},
	}
}

// countIn counts rows whose timestamp column falls inside the window.
func countIn(conn *gorm.DB, model interface{}, column string, from, to time.Time, where string) float64 {
	var total int64
	conn.Model(model).
		Where(where).
		Where(column+" >= ? AND "+column+" < ?", from, to).
		Count(&total)
	return float64(total)
}

// countAll counts rows matching a point-in-time condition (no window).
func countAll(conn *gorm.DB, model interface{}, where string) float64 {
	var total int64
	conn.Model(model).Where(where).Count(&total)
	return float64(total)
}

// dashboardSeriesBucketExpr maps a granularity onto a Postgres date_trunc unit.
// The granularity value is validated against an allowlist in
// parseDashboardFilters before it ever reaches here.
func dashboardSeriesBucketExpr(column, granularity string) string {
	unit := "day"
	switch granularity {
	case "week":
		unit = "week"
	case "month":
		unit = "month"
	}
	return fmt.Sprintf("TO_CHAR(date_trunc('%s', %s), 'YYYY-MM-DD')", unit, column)
}

// dashboardSeriesRow is the shared shape of every grouped series query.
type dashboardSeriesRow struct {
	Bucket string
	Value  float64
}

// countSeries resolves a bucketed COUNT over the whole window in one query.
func countSeries(conn *gorm.DB, model interface{}, column string, from, to time.Time, granularity, where string) map[string]float64 {
	return aggregateSeries(conn, model, column, "COUNT(*)", from, to, granularity, where)
}

// aggregateSeries resolves a bucketed aggregate (COUNT/SUM/AVG) in one query.
func aggregateSeries(conn *gorm.DB, model interface{}, column, aggregate string, from, to time.Time, granularity, where string) map[string]float64 {
	bucket := dashboardSeriesBucketExpr(column, granularity)

	var rows []dashboardSeriesRow
	conn.Model(model).
		Select(bucket+" as bucket, COALESCE("+aggregate+", 0) as value").
		Where(where).
		Where(column+" >= ? AND "+column+" < ?", from, to).
		Group(bucket).
		Scan(&rows)

	series := make(map[string]float64, len(rows))
	for _, row := range rows {
		series[row.Bucket] = row.Value
	}
	return series
}
