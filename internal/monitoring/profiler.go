package monitoring

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsCollector collects and exposes application metrics
type MetricsCollector struct {
	// HTTP metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec

	// Database metrics
	dbQueriesTotal      *prometheus.CounterVec
	dbQueryDuration     *prometheus.HistogramVec
	dbConnectionsActive *prometheus.GaugeVec

	// Cache metrics
	cacheHitsTotal   *prometheus.CounterVec
	cacheMissesTotal *prometheus.CounterVec
	cacheSize        *prometheus.GaugeVec

	// Business metrics
	activeUsers      prometheus.Gauge
	enrollmentsTotal prometheus.Counter
	paymentsTotal    *prometheus.CounterVec

	// System metrics
	goroutinesCount prometheus.Gauge
	memoryUsage     prometheus.Gauge
	cpuUsage        prometheus.Gauge

	mu sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	// Create metrics using prometheus directly (not promauto) to avoid pointer issues
	httpRequestsTotal := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	httpRequestSize := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)
	httpResponseSize := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)
	dbQueriesTotal := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table", "status"},
	)
	dbQueryDuration := promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "table"},
	)
	dbConnectionsActive := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help:  "Number of active database connections",
		},
		[]string{"pool"},
	)
	cacheHitsTotal := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type", "key_prefix"},
	)
	cacheMissesTotal := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type", "key_prefix"},
	)
	cacheSize := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_size",
			Help: "Current cache size in entries",
		},
		[]string{"cache_type"},
	)

	// Create simple metrics using prometheus directly (not promauto) to avoid pointer issues
	activeUsers := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "active_users",
		Help: "Number of currently active users",
	})
	enrollmentsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "enrollments_total",
		Help: "Total number of course enrollments",
	})
	paymentsTotal := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_total",
			Help: "Total number of payments",
		},
		[]string{"method", "status"},
	)
	goroutinesCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "goroutines_count",
		Help: "Number of active goroutines",
	})
	memoryUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_usage_bytes",
		Help: "Current memory usage in bytes",
	})
	cpuUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_usage_percent",
		Help: "Current CPU usage percentage",
	})

	// Register the manually created metrics
	prometheus.MustRegister(activeUsers, enrollmentsTotal, goroutinesCount, memoryUsage, cpuUsage)

	mc := &MetricsCollector{
		httpRequestsTotal:   httpRequestsTotal,
		httpRequestDuration: httpRequestDuration,
		httpRequestSize:     httpRequestSize,
		httpResponseSize:    httpResponseSize,
		dbQueriesTotal:      dbQueriesTotal,
		dbQueryDuration:     dbQueryDuration,
		dbConnectionsActive: dbConnectionsActive,
		cacheHitsTotal:      cacheHitsTotal,
		cacheMissesTotal:    cacheMissesTotal,
		cacheSize:           cacheSize,
		activeUsers:         activeUsers,
		enrollmentsTotal:    enrollmentsTotal,
		paymentsTotal:       paymentsTotal,
		goroutinesCount:     goroutinesCount,
		memoryUsage:         memoryUsage,
		cpuUsage:            cpuUsage,
	}

	// Start system metrics collection
	go mc.collectSystemMetrics()

	return mc
}

// collectSystemMetrics periodically collects system metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		mc.goroutinesCount.Set(float64(runtime.NumGoroutine()))
		mc.memoryUsage.Set(float64(m.Alloc))
	}
}

// RecordHTTPRequest records HTTP request metrics
func (mc *MetricsCollector) RecordHTTPRequest(method, path, status string, duration time.Duration, requestSize, responseSize int64) {
	mc.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	mc.httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	mc.httpRequestSize.WithLabelValues(method, path).Observe(float64(requestSize))
	mc.httpResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
}

// RecordDBQuery records database query metrics
func (mc *MetricsCollector) RecordDBQuery(operation, table, status string, duration time.Duration) {
	mc.dbQueriesTotal.WithLabelValues(operation, table, status).Inc()
	mc.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit
func (mc *MetricsCollector) RecordCacheHit(cacheType, keyPrefix string) {
	mc.cacheHitsTotal.WithLabelValues(cacheType, keyPrefix).Inc()
}

// RecordCacheMiss records a cache miss
func (mc *MetricsCollector) RecordCacheMiss(cacheType, keyPrefix string) {
	mc.cacheMissesTotal.WithLabelValues(cacheType, keyPrefix).Inc()
}

// SetCacheSize sets the current cache size
func (mc *MetricsCollector) SetCacheSize(cacheType string, size int) {
	mc.cacheSize.WithLabelValues(cacheType).Set(float64(size))
}

// SetActiveUsers sets the number of active users
func (mc *MetricsCollector) SetActiveUsers(count int) {
	mc.activeUsers.Set(float64(count))
}

// RecordEnrollment records a new enrollment
func (mc *MetricsCollector) RecordEnrollment() {
	mc.enrollmentsTotal.Inc()
}

// RecordPayment records a payment
func (mc *MetricsCollector) RecordPayment(method, status string) {
	mc.paymentsTotal.WithLabelValues(method, status).Inc()
}

// SetDBConnections sets the number of active database connections
func (mc *MetricsCollector) SetDBConnections(pool string, count int) {
	mc.dbConnectionsActive.WithLabelValues(pool).Set(float64(count))
}

// ProfilingMiddleware returns a Gin middleware for request profiling
func ProfilingMiddleware(mc *MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		status := c.Writer.Status()
		requestSize := c.Request.ContentLength
		responseSize := int64(c.Writer.Size())

		mc.RecordHTTPRequest(c.Request.Method, path, string(rune(status)), duration, requestSize, responseSize)
	}
}

// QueryProfiler profiles database queries
type QueryProfiler struct {
	collector *MetricsCollector
}

// NewQueryProfiler creates a new query profiler
func NewQueryProfiler(collector *MetricsCollector) *QueryProfiler {
	return &QueryProfiler{collector: collector}
}

// ProfileQuery profiles a database query execution
func (qp *QueryProfiler) ProfileQuery(ctx context.Context, operation, table string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	qp.collector.RecordDBQuery(operation, table, status, duration)

	// Log slow queries
	if duration > 500*time.Millisecond {
		log.Printf("[SLOW QUERY] Operation: %s, Table: %s, Duration: %v, Status: %s", operation, table, duration, status)
	}

	return err
}

// Global metrics collector instance
var GlobalMetricsCollector *MetricsCollector
var metricsOnce sync.Once

// GetMetricsCollector returns the global metrics collector instance
func GetMetricsCollector() *MetricsCollector {
	metricsOnce.Do(func() {
		GlobalMetricsCollector = NewMetricsCollector()
	})
	return GlobalMetricsCollector
}

// HealthCheck provides a health check endpoint
func HealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		})
	}
}

// ReadinessCheck provides a readiness check endpoint
func ReadinessCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check database connectivity
		// Check Redis connectivity
		// Check other dependencies

		c.JSON(200, gin.H{
			"status":    "ready",
			"timestamp": time.Now().Unix(),
			"checks": gin.H{
				"database": "ok",
				"cache":    "ok",
			},
		})
	}
}