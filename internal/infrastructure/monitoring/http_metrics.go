package monitoring

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	db "thanawy-backend/internal/infrastructure/database"
)

const metricQueueSize = 4096
const inProcessHistory = 7 * 24 * time.Hour
const maxInProcessBuckets = 7 * 24 * 60
const maxDurationsPerBucket = 512

type HTTPRequestMetric struct {
	Timestamp  time.Time
	Route      string
	Method     string
	Status     int
	DurationMS int64
	Slow       bool
}

type PerformancePoint struct {
	Time            time.Time `json:"time"`
	RequestCount    int64     `json:"requestCount"`
	ErrorCount      int64     `json:"errorCount"`
	SlowCount       int64     `json:"slowCount"`
	DurationSumMS   int64     `json:"durationSumMs"`
	DurationMaxMS   int64     `json:"durationMaxMs"`
	AvgDurationMS   float64   `json:"avgDurationMs"`
	P50ResponseTime float64   `json:"p50ResponseTime"`
	P95ResponseTime float64   `json:"p95ResponseTime"`
	P99ResponseTime float64   `json:"p99ResponseTime"`
	ErrorRate       float64   `json:"errorRate"`
}

type PerformanceSummary struct {
	RequestCount    int64
	ErrorCount      int64
	SlowCount       int64
	AvgResponseTime float64
	P50ResponseTime float64
	P95ResponseTime float64
	P99ResponseTime float64
	ErrorRate       float64
	RequestsPerMin  float64
}

type processBucket struct {
	requestCount int64
	errorCount   int64
	slowCount    int64
	durationSum  int64
	durationMax  int64
	durations    []int64
}

type processCollector struct {
	mu      sync.RWMutex
	buckets map[time.Time]*processBucket
}

var processHistory = &processCollector{buckets: make(map[time.Time]*processBucket)}

func (p *processCollector) record(metric HTTPRequestMetric) {
	bucketTime := metric.Timestamp.UTC().Truncate(time.Minute)
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := bucketTime.Add(-inProcessHistory)
	for key := range p.buckets {
		if key.Before(cutoff) {
			delete(p.buckets, key)
		}
	}
	bucket := p.buckets[bucketTime]
	if bucket == nil {
		if len(p.buckets) >= maxInProcessBuckets {
			oldest := bucketTime
			for key := range p.buckets {
				if key.Before(oldest) {
					oldest = key
				}
			}
			delete(p.buckets, oldest)
		}
		bucket = &processBucket{}
		p.buckets[bucketTime] = bucket
	}
	bucket.requestCount++
	if metric.Status >= 400 {
		bucket.errorCount++
	}
	if metric.Slow {
		bucket.slowCount++
	}
	if metric.DurationMS > bucket.durationMax {
		bucket.durationMax = metric.DurationMS
	}
	bucket.durationSum += metric.DurationMS
	if len(bucket.durations) < maxDurationsPerBucket {
		bucket.durations = append(bucket.durations, metric.DurationMS)
	} else {
		// Deterministic bounded reservoir: replacing by position keeps memory fixed.
		index := int(bucket.requestCount % maxDurationsPerBucket)
		bucket.durations[index] = metric.DurationMS
	}
}

func (p *processCollector) snapshot(from, to time.Time, interval time.Duration) ([]PerformancePoint, PerformanceSummary) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if interval < time.Minute {
		interval = time.Minute
	}
	pointsByTime := make(map[time.Time]*PerformancePoint)
	var summary PerformanceSummary
	for bucketTime, bucket := range p.buckets {
		if bucketTime.Before(from.UTC()) || !bucketTime.Before(to.UTC()) {
			continue
		}
		key := bucketTime.Truncate(interval)
		point := pointsByTime[key]
		if point == nil {
			point = &PerformancePoint{Time: key}
			pointsByTime[key] = point
		}
		row := HTTPMetricBucket{RequestCount: bucket.requestCount, ErrorCount: bucket.errorCount, SlowCount: bucket.slowCount, DurationSum: bucket.durationSum, DurationMax: bucket.durationMax, P50: percentile(sortedCopy(bucket.durations), .50), P95: percentile(sortedCopy(bucket.durations), .95), P99: percentile(sortedCopy(bucket.durations), .99)}
		mergePoint(point, row)
		summary.RequestCount += row.RequestCount
		summary.ErrorCount += row.ErrorCount
		summary.SlowCount += row.SlowCount
		summary.AvgResponseTime += float64(row.DurationSum)
		summary.P50ResponseTime += row.P50 * float64(row.RequestCount)
		summary.P95ResponseTime += row.P95 * float64(row.RequestCount)
		summary.P99ResponseTime += row.P99 * float64(row.RequestCount)
	}
	points := make([]PerformancePoint, 0, len(pointsByTime))
	for _, point := range pointsByTime {
		finalizePoint(point)
		points = append(points, *point)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Time.Before(points[j].Time) })
	finalizeSummary(&summary, from, to)
	return points, summary
}

func sortedCopy(values []int64) []int64 {
	result := append([]int64(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

var (
	metricQueue = make(chan HTTPRequestMetric, metricQueueSize)
	writerOnce  sync.Once
)

func RecordHTTPRequest(metric HTTPRequestMetric) {
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now().UTC()
	}
	processHistory.record(metric)
	writerOnce.Do(func() { go metricWriter() })
	select {
	case metricQueue <- metric:
	default:
	}
}

func metricWriter() {
	// Use a smaller batch to keep each multi-row upsert well under the
	// 500ms SLOW SQL threshold. A batch of 500 metrics can produce a
	// single INSERT with hundreds of rows (one per unique route/status
	// bucket), which was observed taking ~4.7s in production.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]HTTPRequestMetric, 0, 200)
	for {
		select {
		case metric := <-metricQueue:
			batch = append(batch, metric)
			if len(batch) >= 200 {
				persistMetricBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				persistMetricBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

type metricKey struct {
	bucket        time.Time
	route, method string
	status        int
}

func persistMetricBatch(metrics []HTTPRequestMetric) {
	if db.RawWriteDB() == nil || len(metrics) == 0 {
		return
	}
	grouped := make(map[metricKey][]HTTPRequestMetric)
	for _, metric := range metrics {
		key := metricKey{metric.Timestamp.UTC().Truncate(time.Minute), metric.Route, metric.Method, metric.Status}
		grouped[key] = append(grouped[key], metric)
	}

	if len(grouped) == 0 {
		return
	}

	var valueStrings []string
	var valueArgs []interface{}

	for key, samples := range grouped {
		durations := make([]int64, 0, len(samples))
		var errors, slow, sum, maximum int64
		for _, sample := range samples {
			durations = append(durations, sample.DurationMS)
			sum += sample.DurationMS
			if sample.DurationMS > maximum {
				maximum = sample.DurationMS
			}
			if sample.Status >= 400 {
				errors++
			}
			if sample.Slow {
				slow++
			}
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		count := int64(len(samples))

		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())")
		valueArgs = append(valueArgs, key.bucket, key.route, key.method, key.status, count, errors, slow, sum, maximum, percentile(durations, .50), percentile(durations, .95), percentile(durations, .99))
	}

	query := fmt.Sprintf(`INSERT INTO http_metric_buckets (bucket_start, route, method, status, request_count, error_count, slow_count, duration_sum_ms, duration_max_ms, p50_ms, p95_ms, p99_ms, updated_at) VALUES %s ON CONFLICT (bucket_start, route, method, status) DO UPDATE SET p50_ms = ((http_metric_buckets.p50_ms * http_metric_buckets.request_count) + (EXCLUDED.p50_ms * EXCLUDED.request_count)) / (http_metric_buckets.request_count + EXCLUDED.request_count), p95_ms = ((http_metric_buckets.p95_ms * http_metric_buckets.request_count) + (EXCLUDED.p95_ms * EXCLUDED.request_count)) / (http_metric_buckets.request_count + EXCLUDED.request_count), p99_ms = ((http_metric_buckets.p99_ms * http_metric_buckets.request_count) + (EXCLUDED.p99_ms * EXCLUDED.request_count)) / (http_metric_buckets.request_count + EXCLUDED.request_count), request_count = http_metric_buckets.request_count + EXCLUDED.request_count, error_count = http_metric_buckets.error_count + EXCLUDED.error_count, slow_count = http_metric_buckets.slow_count + EXCLUDED.slow_count, duration_sum_ms = http_metric_buckets.duration_sum_ms + EXCLUDED.duration_sum_ms, duration_max_ms = GREATEST(http_metric_buckets.duration_max_ms, EXCLUDED.duration_max_ms), updated_at = NOW()`, strings.Join(valueStrings, ", "))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.RawWriteDB(ctx).Exec(query, valueArgs...).Error; err != nil {
		log.Printf("[PERF] failed to persist HTTP metric bucket batch: %v", err)
	}
}

func percentile(sorted []int64, quantile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(quantile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index])
}

func QueryPerformance(ctx context.Context, from, to time.Time, interval time.Duration) ([]PerformancePoint, PerformanceSummary, error) {
	if interval < time.Minute {
		interval = time.Minute
	}
	currentMinute := time.Now().UTC().Truncate(time.Minute)
	dbTo := to.UTC()
	if dbTo.After(currentMinute) {
		dbTo = currentMinute
	}
	var rows []HTTPMetricBucket
	if !dbTo.Before(from.UTC()) {
		if err := db.RawReadDB(ctx).Where("bucket_start >= ? AND bucket_start < ?", from.UTC(), dbTo).Order("bucket_start asc").Find(&rows).Error; err != nil {
			return nil, PerformanceSummary{}, err
		}
	}
	pointMap := make(map[time.Time]*PerformancePoint)
	var summary PerformanceSummary
	addRow := func(row HTTPMetricBucket) {
		key := row.BucketStart.UTC().Truncate(interval)
		point := pointMap[key]
		if point == nil {
			point = &PerformancePoint{Time: key}
			pointMap[key] = point
		}
		mergePoint(point, row)
		summary.RequestCount += row.RequestCount
		summary.ErrorCount += row.ErrorCount
		summary.SlowCount += row.SlowCount
		summary.AvgResponseTime += float64(row.DurationSum)
		summary.P50ResponseTime += row.P50 * float64(row.RequestCount)
		summary.P95ResponseTime += row.P95 * float64(row.RequestCount)
		summary.P99ResponseTime += row.P99 * float64(row.RequestCount)
	}
	for _, row := range rows {
		addRow(row)
	}
	memoryFrom := from.UTC()
	if memoryFrom.Before(currentMinute) {
		memoryFrom = currentMinute
	}
	if memoryFrom.Before(to.UTC()) {
		memoryPoints, memorySummary := processHistory.snapshot(memoryFrom, to.UTC(), interval)
		for _, point := range memoryPoints {
			addRow(HTTPMetricBucket{BucketStart: point.Time, RequestCount: point.RequestCount, ErrorCount: point.ErrorCount, SlowCount: point.SlowCount, DurationSum: point.DurationSumMS, DurationMax: point.DurationMaxMS, P50: point.P50ResponseTime, P95: point.P95ResponseTime, P99: point.P99ResponseTime})
		}
		_ = memorySummary
	}
	points := make([]PerformancePoint, 0, len(pointMap))
	for _, point := range pointMap {
		finalizePoint(point)
		points = append(points, *point)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Time.Before(points[j].Time) })
	finalizeSummary(&summary, from, to)
	return points, summary, nil
}

func finalizeSummary(summary *PerformanceSummary, from, to time.Time) {
	if summary.RequestCount > 0 {
		count := float64(summary.RequestCount)
		summary.AvgResponseTime /= count
		summary.P50ResponseTime /= count
		summary.P95ResponseTime /= count
		summary.P99ResponseTime /= count
		summary.ErrorRate = float64(summary.ErrorCount) / count
	}
	if minutes := to.Sub(from).Minutes(); minutes > 0 {
		summary.RequestsPerMin = float64(summary.RequestCount) / minutes
	}
}
func mergePoint(point *PerformancePoint, row HTTPMetricBucket) {
	oldCount := point.RequestCount
	point.RequestCount += row.RequestCount
	point.ErrorCount += row.ErrorCount
	point.SlowCount += row.SlowCount
	point.DurationSumMS += row.DurationSum
	if row.DurationMax > point.DurationMaxMS {
		point.DurationMaxMS = row.DurationMax
	}
	point.P50ResponseTime = weightedMerge(point.P50ResponseTime, oldCount, row.P50, row.RequestCount)
	point.P95ResponseTime = weightedMerge(point.P95ResponseTime, oldCount, row.P95, row.RequestCount)
	point.P99ResponseTime = weightedMerge(point.P99ResponseTime, oldCount, row.P99, row.RequestCount)
}
func weightedMerge(current float64, currentCount int64, next float64, nextCount int64) float64 {
	total := currentCount + nextCount
	if total == 0 {
		return 0
	}
	return (current*float64(currentCount) + next*float64(nextCount)) / float64(total)
}
func finalizePoint(point *PerformancePoint) {
	if point.RequestCount > 0 {
		point.AvgDurationMS = float64(point.DurationSumMS) / float64(point.RequestCount)
		point.ErrorRate = float64(point.ErrorCount) / float64(point.RequestCount)
	}
}
