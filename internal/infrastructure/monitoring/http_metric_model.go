package monitoring

import "time"

// HTTPMetricBucket stores aggregated request measurements for one route, method,
// status code, and minute. Percentiles are calculated from each asynchronous
// batch and merged as weighted approximations.
type HTTPMetricBucket struct {
	BucketStart  time.Time `gorm:"column:bucket_start;primaryKey" json:"bucketStart"`
	Route        string    `gorm:"column:route;primaryKey" json:"route"`
	Method       string    `gorm:"column:method;primaryKey" json:"method"`
	Status       int       `gorm:"column:status;primaryKey" json:"status"`
	RequestCount int64     `gorm:"column:request_count" json:"requestCount"`
	ErrorCount   int64     `gorm:"column:error_count" json:"errorCount"`
	SlowCount    int64     `gorm:"column:slow_count" json:"slowCount"`
	DurationSum  int64     `gorm:"column:duration_sum_ms" json:"durationSumMs"`
	DurationMax  int64     `gorm:"column:duration_max_ms" json:"durationMaxMs"`
	P50          float64   `gorm:"column:p50_ms" json:"p50Ms"`
	P95          float64   `gorm:"column:p95_ms" json:"p95Ms"`
	P99          float64   `gorm:"column:p99_ms" json:"p99Ms"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (HTTPMetricBucket) TableName() string { return "http_metric_buckets" }
