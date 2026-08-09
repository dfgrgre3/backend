package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPercentile(t *testing.T) {
	samples := []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	assert.Equal(t, float64(50), percentile(samples, .50))
	assert.Equal(t, float64(100), percentile(samples, .95))
	assert.Equal(t, float64(100), percentile(samples, .99))
	assert.Zero(t, percentile(nil, .95))
}

func TestProcessCollectorSnapshotAndBounds(t *testing.T) {
	collector := &processCollector{buckets: make(map[time.Time]*processBucket)}
	now := time.Now().UTC().Truncate(time.Minute)
	for i, duration := range []int64{10, 20, 30, 40, 50} {
		status := 200
		if i == 4 {
			status = 500
		}
		collector.record(HTTPRequestMetric{Timestamp: now.Add(time.Duration(i) * time.Second), Status: status, DurationMS: duration})
	}

	points, summary := collector.snapshot(now.Add(-time.Minute), now.Add(time.Minute), time.Minute)
	assert.Len(t, points, 1)
	assert.Equal(t, int64(5), summary.RequestCount)
	assert.Equal(t, float64(30), summary.AvgResponseTime)
	assert.Equal(t, float64(50), summary.P95ResponseTime)
	assert.Equal(t, float64(0.2), summary.ErrorRate)
	assert.InDelta(t, 2.5, summary.RequestsPerMin, 0.001)

	bucket := collector.buckets[now]
	for i := 0; i < maxDurationsPerBucket+10; i++ {
		collector.record(HTTPRequestMetric{Timestamp: now, Status: 200, DurationMS: int64(i)})
	}
	assert.LessOrEqual(t, len(bucket.durations), maxDurationsPerBucket)
}

func TestMergeAndFinalizePoint(t *testing.T) {
	point := &PerformancePoint{Time: time.Now()}
	mergePoint(point, HTTPMetricBucket{RequestCount: 2, ErrorCount: 1, SlowCount: 1, DurationSum: 100, DurationMax: 70, P50: 40, P95: 70, P99: 70})
	mergePoint(point, HTTPMetricBucket{RequestCount: 2, DurationSum: 300, DurationMax: 200, P50: 100, P95: 200, P99: 200})
	finalizePoint(point)

	assert.Equal(t, int64(4), point.RequestCount)
	assert.Equal(t, int64(1), point.ErrorCount)
	assert.Equal(t, float64(100), point.AvgDurationMS)
	assert.Equal(t, float64(0.25), point.ErrorRate)
	assert.Equal(t, float64(70), point.P50ResponseTime)
	assert.Equal(t, int64(200), point.DurationMaxMS)
}
