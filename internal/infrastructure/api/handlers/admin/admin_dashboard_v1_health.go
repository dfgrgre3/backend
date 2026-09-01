package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/handlers/shared"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"
	worker "thanawy-backend/internal/infrastructure/workers"

	"thanawy-backend/internal/infrastructure/monitoring"

	"github.com/gin-gonic/gin"
)

// Registers this package's probe runner as the implementation the worker
// package's periodic health-check task calls. See the doc comment on
// worker.ServiceHealthSnapshotPersister for why this indirection exists
// (avoids an admin<->workers import cycle).
func init() {
	worker.ServiceHealthSnapshotPersister = PersistDashboardServiceHealthSnapshot
}

// dashboardServiceCheck يصف فحصًا واحدًا لخدمة أساسية.
type dashboardServiceCheck struct {
	Key       string
	Name      string
	ActionURL string
	// Probe يعيد زمن الاستجابة والخطأ إن وجد.
	Probe func(ctx context.Context) error
}

// dashboardProbeResult is one service's outcome from a single probe run.
type dashboardProbeResult struct {
	Check     dashboardServiceCheck
	Status    string
	Details   string
	Latency   float64
	ErrorRate *float64
}

// runDashboardServiceProbes executes every selected probe concurrently and
// classifies the outcome (healthy/degraded/unhealthy). canSeeDetails controls
// whether the raw error message (which may leak internal hosts/paths) is
// included, or the generic Arabic fallback instead.
//
// Shared by the live HTTP endpoint (GetDashboardSystemHealth) and the
// periodic background job that persists results into service_health_checks,
// so both surfaces classify status identically.
func runDashboardServiceProbes(ctx context.Context, selected []dashboardServiceCheck, canSeeDetails bool) ([]dashboardProbeResult, string) {
	type rawResult struct {
		err     error
		latency float64
	}
	raw := make([]rawResult, len(selected))

	var wg sync.WaitGroup
	for i, check := range selected {
		wg.Add(1)
		go func(idx int, ch dashboardServiceCheck) {
			defer wg.Done()
			start := time.Now()
			raw[idx].err = ch.Probe(ctx)
			raw[idx].latency = float64(time.Since(start).Microseconds()) / 1000
		}(i, check)
	}
	wg.Wait()

	// معدل أخطاء الـ API يُقرأ من مقاييس HTTP المُجمّعة لآخر ساعة.
	now := time.Now()
	_, summary, summaryErr := monitoring.QueryPerformance(ctx, now.Add(-time.Hour), now, time.Minute)
	overall := "healthy"

	results := make([]dashboardProbeResult, len(selected))
	for i, check := range selected {
		status := "healthy"
		details := "تعمل بشكل طبيعي"

		switch {
		case raw[i].err != nil:
			status = "unhealthy"
			// رسالة الخطأ الخام قد تحتوي مسارات أو مضيفين داخليين.
			details = "الخدمة غير متاحة"
			if canSeeDetails {
				details = raw[i].err.Error()
			}
		case check.Key == "api" && summaryErr == nil && summary.ErrorRate >= 0.05:
			status = "degraded"
			details = "معدل أخطاء مرتفع"
		}

		if status == "unhealthy" {
			overall = "unhealthy"
		} else if status == "degraded" && overall == "healthy" {
			overall = "degraded"
		}

		result := dashboardProbeResult{
			Check:   check,
			Status:  status,
			Details: details,
			Latency: raw[i].latency,
		}
		// Only the API surface has a measured error rate. Reporting a literal 0
		// for the other probes asserted a measurement that was never taken, so
		// they report null instead of a fabricated healthy figure.
		if check.Key == "api" && summaryErr == nil {
			errRate := summary.ErrorRate
			result.ErrorRate = &errRate
		}
		results[i] = result
	}

	return results, overall
}

// selectDashboardServiceChecks filters the known probes down to the
// includeServices allow-list, or every known service when it is empty.
func selectDashboardServiceChecks(requested map[string]bool) []dashboardServiceCheck {
	checks := dashboardServiceChecks()
	selected := make([]dashboardServiceCheck, 0, len(checks))
	for _, check := range checks {
		if len(requested) == 0 || requested[check.Key] {
			selected = append(selected, check)
		}
	}
	return selected
}

// GetDashboardSystemHealth يعرض الحالة التشغيلية للخدمات الأساسية.
// التفاصيل التقنية (رسائل الأخطاء) لا تُعرض إلا لمن يملك إذن البيانات الحساسة،
// لأنها قد تكشف بنية داخلية أو أسماء مضيفين.
//
// GET /api/admin/dashboard/system-health
func GetDashboardSystemHealth(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardViewSystemHealth) {
		return
	}

	if tenant := strings.TrimSpace(c.Query("tenantId")); len(tenant) > 64 {
		api_response.Error(c, http.StatusBadRequest, "tenantId is invalid")
		return
	}

	// includeServices يقيّد الفحص على خدمات محددة لتقليل زمن الاستجابة.
	requested := map[string]bool{}
	if raw := strings.TrimSpace(c.Query("includeServices")); raw != "" {
		for _, key := range strings.Split(raw, ",") {
			if key = strings.TrimSpace(key); key != "" {
				requested[key] = true
			}
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	selected := selectDashboardServiceChecks(requested)
	if len(selected) == 0 {
		api_response.Error(c, http.StatusBadRequest, "includeServices did not match any known service")
		return
	}

	canSeeDetails := dashboardCan(c, models.PermDashboardViewSensitive)
	now := time.Now()
	results, overall := runDashboardServiceProbes(ctx, selected, canSeeDetails)

	services := make([]gin.H, 0, len(results))
	for _, result := range results {
		service := gin.H{
			"serviceKey":    result.Check.Key,
			"serviceName":   result.Check.Name,
			"status":        result.Status,
			"latency":       result.Latency,
			"lastCheckedAt": now,
			"details":       result.Details,
			"actionUrl":     result.Check.ActionURL,
		}
		if result.ErrorRate != nil {
			service["errorRate"] = *result.ErrorRate
		} else {
			service["errorRate"] = nil
		}
		services = append(services, service)
	}

	api_response.Success(c, gin.H{
		"overallStatus": overall,
		"checkedAt":     now,
		"services":      services,
	})
}

// dashboardServiceChecks يعرّف الخدمات المفحوصة. الخدمات غير المُهيّأة تُبلّغ
// كخطأ صريح بدل تجاهلها بصمت.
func dashboardServiceChecks() []dashboardServiceCheck {
	return []dashboardServiceCheck{
		{
			Key: "database", Name: "قاعدة البيانات", ActionURL: "/admin/health",
			Probe: func(ctx context.Context) error {
				if db.DB == nil {
					return fmt.Errorf("database is not configured")
				}
				return db.DB.WithContext(ctx).Exec("SELECT 1").Error
			},
		},
		{
			Key: "cache", Name: "الذاكرة المؤقتة", ActionURL: "/admin/health",
			Probe: func(ctx context.Context) error {
				if cache.Redis == nil {
					return fmt.Errorf("redis is not configured")
				}
				return cache.Redis.Ping(ctx).Err()
			},
		},
		{
			Key: "storage", Name: "التخزين", ActionURL: "/admin/file-manager",
			Probe: shared.ProbeStorage,
		},
		{
			Key: "search", Name: "محرك البحث", ActionURL: "/admin/health",
			Probe: func(ctx context.Context) error {
				// البحث النصي مُنفَّذ داخل PostgreSQL عبر tsvector.
				if db.DB == nil {
					return fmt.Errorf("database is not configured")
				}
				return db.DB.WithContext(ctx).Exec("SELECT to_tsvector('simple', 'health')").Error
			},
		},
		{
			Key: "queue", Name: "طوابير المعالجة", ActionURL: "/admin/health",
			Probe: func(ctx context.Context) error {
				// الطوابير مبنية على Redis (asynq)، لذا توفّر Redis شرط لعملها.
				if cache.Redis == nil {
					return fmt.Errorf("queue backend is not configured")
				}
				return cache.Redis.Ping(ctx).Err()
			},
		},
		{
			Key: "scheduler", Name: "المهام المجدولة", ActionURL: "/admin/health",
			Probe: func(ctx context.Context) error {
				if db.DB == nil {
					return fmt.Errorf("database is not configured")
				}
				return db.DB.WithContext(ctx).
					Raw(`SELECT 1 FROM "MailTask" LIMIT 1`).Error
			},
		},
		{
			Key: "api", Name: "واجهة البرمجة", ActionURL: "/admin/api-logs",
			Probe: func(ctx context.Context) error {
				// The request is being served, so the process is up. What is
				// worth probing is whether the metrics pipeline that feeds this
				// service's error rate is actually readable — otherwise the
				// status would be reported as healthy on no evidence at all.
				if _, _, err := monitoring.QueryPerformance(ctx, time.Now().Add(-time.Minute), time.Now(), time.Minute); err != nil {
					return fmt.Errorf("api metrics are unavailable: %w", err)
				}
				return nil
			},
		},
	}
}

// ServiceHealthCheck is one persisted probe result. Written every minute by
// the background scheduler (worker.TypeServiceHealthCheck) so per-service
// detail pages can show real history instead of only the live probe.
type ServiceHealthCheck struct {
	CheckedAt  time.Time `gorm:"column:checked_at;primaryKey"`
	ServiceKey string    `gorm:"column:service_key;primaryKey"`
	Status     string    `gorm:"column:status"`
	LatencyMS  float64   `gorm:"column:latency_ms"`
	ErrorRate  *float64  `gorm:"column:error_rate"`
	Details    string    `gorm:"column:details"`
}

func (ServiceHealthCheck) TableName() string { return "service_health_checks" }

// PersistDashboardServiceHealthSnapshot runs every known probe and inserts
// one row per service into service_health_checks. Called from the periodic
// scheduler job — never from the HTTP request path — so a slow probe cannot
// delay a dashboard response. Error details are always stored (this table is
// internal-only, unlike the HTTP response which redacts them for callers
// without the sensitive-data permission).
func PersistDashboardServiceHealthSnapshot(ctx context.Context) error {
	if db.RawWriteDB() == nil {
		return fmt.Errorf("database is not configured")
	}

	selected := selectDashboardServiceChecks(nil)
	results, _ := runDashboardServiceProbes(ctx, selected, true)

	now := time.Now().UTC()
	rows := make([]ServiceHealthCheck, 0, len(results))
	for _, result := range results {
		rows = append(rows, ServiceHealthCheck{
			CheckedAt:  now,
			ServiceKey: result.Check.Key,
			Status:     result.Status,
			LatencyMS:  result.Latency,
			ErrorRate:  result.ErrorRate,
			Details:    result.Details,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return db.RawWriteDB(ctx).Create(&rows).Error
}

// serviceHealthHistoryRanges mirrors the TimeRange values the admin health UI
// already exposes (protected.parseHealthRange), duplicated locally because
// that type is unexported in the protected package.
var serviceHealthHistoryRanges = map[string]struct {
	Duration time.Duration
}{
	"15m": {15 * time.Minute},
	"1h":  {time.Hour},
	"6h":  {6 * time.Hour},
	"24h": {24 * time.Hour},
	"7d":  {7 * 24 * time.Hour},
}

// GetDashboardServiceHealthHistory يعرض السجل التاريخي الحقيقي لخدمة واحدة:
// حالتها ووقت استجابتها عبر الفترة المطلوبة، بالإضافة لملخص (نسبة التوفر،
// متوسط زمن الاستجابة، عدد الحوادث).
//
// GET /api/admin/dashboard/system-health/:service/history?range=1h
func GetDashboardServiceHealthHistory(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardViewSystemHealth) {
		return
	}

	serviceKey := strings.TrimSpace(c.Param("service"))
	var matched *dashboardServiceCheck
	for _, check := range dashboardServiceChecks() {
		if check.Key == serviceKey {
			c := check
			matched = &c
			break
		}
	}
	if matched == nil {
		api_response.Error(c, http.StatusNotFound, "unknown service")
		return
	}

	rangeValue := c.DefaultQuery("range", "1h")
	selectedRange, ok := serviceHealthHistoryRanges[rangeValue]
	if !ok {
		api_response.Error(c, http.StatusBadRequest, "range must be one of 15m, 1h, 6h, 24h, 7d")
		return
	}

	if db.RawReadDB(c.Request.Context()) == nil {
		api_response.Error(c, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	now := time.Now().UTC()
	from := now.Add(-selectedRange.Duration)

	var rows []ServiceHealthCheck
	if err := db.RawReadDB(c.Request.Context()).
		Where("service_key = ? AND checked_at >= ? AND checked_at <= ?", serviceKey, from, now).
		Order("checked_at asc").
		Find(&rows).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "failed to load service health history")
		return
	}

	canSeeDetails := dashboardCan(c, models.PermDashboardViewSensitive)

	history := make([]gin.H, 0, len(rows))
	var latencySum float64
	var healthyCount, incidentCount int
	for i, row := range rows {
		details := row.Details
		if !canSeeDetails && row.Status == "unhealthy" {
			details = "الخدمة غير متاحة"
		}
		history = append(history, gin.H{
			"checkedAt": row.CheckedAt,
			"status":    row.Status,
			"latencyMs": row.LatencyMS,
			"errorRate": row.ErrorRate,
			"details":   details,
		})
		latencySum += row.LatencyMS
		if row.Status == "healthy" {
			healthyCount++
		} else {
			incidentCount++
		}
		_ = i
	}

	uptimePercent := 100.0
	avgLatencyMs := 0.0
	if len(rows) > 0 {
		uptimePercent = float64(healthyCount) / float64(len(rows)) * 100
		avgLatencyMs = latencySum / float64(len(rows))
	}

	// الحالة اللحظية تُفحص مباشرة (بدل الاعتماد على آخر صف مُخزّن، اللي ممكن
	// يكون عمره حتى دقيقة) حتى تعكس الصفحة نفس ما يراه المستخدم في الداشبورد.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	currentResults, _ := runDashboardServiceProbes(ctx, []dashboardServiceCheck{*matched}, canSeeDetails)
	var current gin.H
	if len(currentResults) == 1 {
		result := currentResults[0]
		current = gin.H{
			"status":        result.Status,
			"latency":       result.Latency,
			"details":       result.Details,
			"lastCheckedAt": time.Now(),
		}
		if result.ErrorRate != nil {
			current["errorRate"] = *result.ErrorRate
		} else {
			current["errorRate"] = nil
		}
	}

	api_response.Success(c, gin.H{
		"service": gin.H{
			"key":       matched.Key,
			"name":      matched.Name,
			"actionUrl": matched.ActionURL,
		},
		"current": current,
		"summary": gin.H{
			"uptimePercent": uptimePercent,
			"avgLatencyMs":  avgLatencyMs,
			"incidentCount": incidentCount,
		},
		"history": history,
	})
}
