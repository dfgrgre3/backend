package middleware

import (
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsCollector holds request metrics for Prometheus/grafana
type MetricsCollector struct {
	mu               sync.RWMutex
	totalRequests    int64
	activeRequests   int64
	errorRequests    int64
	slowRequests     int64
	statusCounts     map[int]int64
	methodCounts     map[string]int64
	pathLatencies    map[string]time.Duration
	pathRequestCount map[string]int64
}

var (
	globalMetrics = &MetricsCollector{
		statusCounts:     make(map[int]int64),
		methodCounts:     make(map[string]int64),
		pathLatencies:    make(map[string]time.Duration),
		pathRequestCount: make(map[string]int64),
	}
	slowRequestThreshold = getSlowRequestThreshold()
)

func getSlowRequestThreshold() time.Duration {
	val := os.Getenv("SLOW_REQUEST_THRESHOLD_MS")
	if val != "" {
		if ms, err := strconv.Atoi(val); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 500 * time.Millisecond
}

// PerformanceMonitor logs the duration of each request and warns if it exceeds a threshold
// Also collects metrics for monitoring dashboards
func PerformanceMonitor() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		method := c.Request.Method

		// Track active requests
		globalMetrics.mu.Lock()
		globalMetrics.activeRequests++
		globalMetrics.totalRequests++
		globalMetrics.mu.Unlock()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)
		status := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		// Update metrics
		globalMetrics.mu.Lock()
		globalMetrics.activeRequests--
		globalMetrics.statusCounts[status]++
		globalMetrics.methodCounts[method]++
		globalMetrics.pathRequestCount[path]++
		globalMetrics.pathLatencies[path] += duration

		if status >= 400 {
			globalMetrics.errorRequests++
		}

		if duration > slowRequestThreshold {
			globalMetrics.slowRequests++
			globalMetrics.mu.Unlock()
			log.Printf("[PERF] SLOW REQUEST: %s %s | Status: %d | Duration: %v ⚠️", method, path, status, duration)
		} else {
			globalMetrics.mu.Unlock()
			// Only log in debug mode or for errors
			if gin.Mode() == gin.DebugMode || status >= 400 {
				log.Printf("[PERF] %s %s | Status: %d | Duration: %v", method, path, status, duration)
			}
		}
	}
}

// GetMetrics returns a snapshot of current metrics
func GetMetrics() map[string]interface{} {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	avgLatencies := make(map[string]string)
	for path, totalLatency := range globalMetrics.pathLatencies {
		if count := globalMetrics.pathRequestCount[path]; count > 0 {
			avg := totalLatency / time.Duration(count)
			avgLatencies[path] = avg.String()
		}
	}

	return map[string]interface{}{
		"total_requests":       globalMetrics.totalRequests,
		"active_requests":      globalMetrics.activeRequests,
		"error_requests":       globalMetrics.errorRequests,
		"slow_requests":        globalMetrics.slowRequests,
		"status_counts":        globalMetrics.statusCounts,
		"method_counts":        globalMetrics.methodCounts,
		"avg_latencies":        avgLatencies,
		"slow_threshold_ms":    slowRequestThreshold.Milliseconds(),
		"uptime":               time.Now().Format(time.RFC3339),
	}
}

// GetMetricsPrometheus returns metrics in Prometheus exposition format
func GetMetricsPrometheus() string {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("# HELP thanawy_http_requests_total Total HTTP requests\n")
	sb.WriteString("# TYPE thanawy_http_requests_total counter\n")
	for method, count := range globalMetrics.methodCounts {
		sb.WriteString(thanawyMetric("http_requests_total", map[string]string{"method": method}, count))
	}

	sb.WriteString("# HELP thanawy_http_requests_active Active HTTP requests\n")
	sb.WriteString("# TYPE thanawy_http_requests_active gauge\n")
	sb.WriteString(thanawyMetric("http_requests_active", nil, globalMetrics.activeRequests))

	sb.WriteString("# HELP thanawy_http_errors_total Total HTTP error responses\n")
	sb.WriteString("# TYPE thanawy_http_errors_total counter\n")
	sb.WriteString(thanawyMetric("http_errors_total", nil, globalMetrics.errorRequests))

	sb.WriteString("# HELP thanawy_http_slow_requests_total Total slow requests\n")
	sb.WriteString("# TYPE thanawy_http_slow_requests_total counter\n")
	sb.WriteString(thanawyMetric("http_slow_requests_total", nil, globalMetrics.slowRequests))

	sb.WriteString("# HELP thanawy_http_status_codes HTTP status code counts\n")
	sb.WriteString("# TYPE thanawy_http_status_codes counter\n")
	for status, count := range globalMetrics.statusCounts {
		sb.WriteString(thanawyMetric("http_status_codes", map[string]string{"code": strconv.Itoa(status)}, count))
	}

	sb.WriteString("# HELP thanawy_requests_total Total requests processed\n")
	sb.WriteString("# TYPE thanawy_requests_total counter\n")
	sb.WriteString(thanawyMetric("requests_total", nil, globalMetrics.totalRequests))

	return sb.String()
}

func thanawyMetric(name string, labels map[string]string, value int64) string {
	var labelStr string
	if len(labels) > 0 {
		pairs := make([]string, 0, len(labels))
		for k, v := range labels {
			pairs = append(pairs, k+"=\""+v+"\"")
		}
		labelStr = "{" + strings.Join(pairs, ",") + "}"
	}
	return "thanawy_" + name + labelStr + " " + strconv.FormatInt(value, 10) + "\n"
}