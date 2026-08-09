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

	"thanawy-backend/internal/infrastructure/monitoring"

	"github.com/gin-gonic/gin"
)

// dashboardServiceCheck يصف فحصًا واحدًا لخدمة أساسية.
type dashboardServiceCheck struct {
	Key       string
	Name      string
	ActionURL string
	// Probe يعيد زمن الاستجابة والخطأ إن وجد.
	Probe func(ctx context.Context) error
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

	checks := dashboardServiceChecks()
	selected := make([]dashboardServiceCheck, 0, len(checks))
	for _, check := range checks {
		if len(requested) == 0 || requested[check.Key] {
			selected = append(selected, check)
		}
	}
	if len(selected) == 0 {
		api_response.Error(c, http.StatusBadRequest, "includeServices did not match any known service")
		return
	}

	canSeeDetails := dashboardCan(c, models.PermDashboardViewSensitive)
	now := time.Now()

	type probeResult struct {
		err     error
		latency float64
	}
	results := make([]probeResult, len(selected))

	var wg sync.WaitGroup
	for i, check := range selected {
		wg.Add(1)
		go func(idx int, ch dashboardServiceCheck) {
			defer wg.Done()
			start := time.Now()
			results[idx].err = ch.Probe(ctx)
			results[idx].latency = float64(time.Since(start).Microseconds()) / 1000
		}(i, check)
	}
	wg.Wait()

	// معدل أخطاء الـ API يُقرأ من مقاييس HTTP المُجمّعة لآخر ساعة.
	_, summary, summaryErr := monitoring.QueryPerformance(ctx, now.Add(-time.Hour), now, time.Minute)
	overall := "healthy"

	services := make([]gin.H, 0, len(selected))
	for i, check := range selected {
		status := "healthy"
		details := "تعمل بشكل طبيعي"

		switch {
		case results[i].err != nil:
			status = "unhealthy"
			// رسالة الخطأ الخام قد تحتوي مسارات أو مضيفين داخليين.
			details = "الخدمة غير متاحة"
			if canSeeDetails {
				details = results[i].err.Error()
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

		service := gin.H{
			"serviceKey":    check.Key,
			"serviceName":   check.Name,
			"status":        status,
			"latency":       results[i].latency,
			"lastCheckedAt": now,
			"details":       details,
			"actionUrl":     check.ActionURL,
		}
		// Only the API surface has a measured error rate. Reporting a literal 0
		// for the other probes asserted a measurement that was never taken, so
		// they report null instead of a fabricated healthy figure.
		if check.Key == "api" && summaryErr == nil {
			service["errorRate"] = summary.ErrorRate
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
