package admin

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// GetDashboardTimeSeries returns a bucketed series for one registered metric.
// It reuses the same metric registry as the summary endpoint, so a metric can
// never be charted without also being permission-gated.
//
// GET /api/admin/dashboard/time-series
func GetDashboardTimeSeries(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardAccess) {
		return
	}
	readDB, abort := safeReadDB(c)
	if abort {
		return
	}

	metricKey := strings.TrimSpace(c.Query("metricKey"))
	if metricKey == "" {
		api_response.Error(c, http.StatusBadRequest, "metricKey is required")
		return
	}

	var spec *dashboardMetricSpec
	specs := dashboardMetricSpecs()
	for i := range specs {
		if specs[i].Key == metricKey {
			spec = &specs[i]
			break
		}
	}
	if spec == nil {
		api_response.Error(c, http.StatusNotFound, fmt.Sprintf("Unknown metricKey '%s'", metricKey))
		return
	}
	if !dashboardRequire(c, spec.Permission) {
		return
	}

	if strings.TrimSpace(c.Query("startDate")) == "" || strings.TrimSpace(c.Query("endDate")) == "" {
		api_response.Error(c, http.StatusBadRequest, "startDate and endDate are required")
		return
	}
	filters, ok := parseDashboardFilters(c)
	if !ok {
		return
	}

	buckets := dashboardBuckets(filters.StartDate, filters.EndDate, filters.Granularity)
	if len(buckets) > dashboardMaxSeriesPoints {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf(
			"granularity '%s' produces %d points for this range; use a coarser granularity (max %d)",
			filters.Granularity, len(buckets), dashboardMaxSeriesPoints))
		return
	}

	// The previous-period comparison shifts each bucket back by the full window
	// so points line up one-to-one with the current series.
	offset := filters.EndDate.Sub(filters.StartDate)

	// Resolve the whole range up front. This used to call spec.Compute once per
	// bucket (and twice per bucket when comparing), so a 200-point daily chart
	// issued up to 400 sequential queries. Now it is at most two.
	var current, previous map[string]float64
	pointInTimeValue := float64(0)

	switch {
	case spec.PointInTime:
		// No historical dimension: the same present-moment value applies to
		// every bucket rather than being re-queried per point.
		pointInTimeValue = spec.Compute(readDB, filters.StartDate, filters.EndDate, filters)
	case spec.Series != nil:
		current = spec.Series(readDB, filters.StartDate, filters.EndDate, filters.Granularity, filters)
		if filters.Compare {
			previous = spec.Series(readDB,
				filters.StartDate.Add(-offset), filters.EndDate.Add(-offset), filters.Granularity, filters)
		}
	}

	points := make([]gin.H, 0, len(buckets))
	now := time.Now()
	for _, b := range buckets {
		bucketKey := truncateToGranularity(b.Start, filters.Granularity).Format("2006-01-02")

		var value float64
		switch {
		case spec.PointInTime:
			value = pointInTimeValue
		case current != nil:
			value = current[bucketKey]
		default:
			// No batched resolver registered: fall back to the per-bucket path
			// so a metric can never silently chart as flat zero.
			value = spec.Compute(readDB, b.Start, b.End, filters)
		}

		point := gin.H{
			"periodStart":     b.Start,
			"periodEnd":       b.End,
			"value":           value,
			"isPartialPeriod": b.End.After(now),
		}

		switch {
		case !filters.Compare:
			point["previousValue"] = nil
		case spec.PointInTime:
			// A point-in-time metric has no meaningful previous value.
			point["previousValue"] = nil
		case previous != nil:
			prevKey := truncateToGranularity(b.Start.Add(-offset), filters.Granularity).Format("2006-01-02")
			point["previousValue"] = previous[prevKey]
		default:
			point["previousValue"] = spec.Compute(readDB, b.Start.Add(-offset), b.End.Add(-offset), filters)
		}

		points = append(points, point)
	}

	api_response.Success(c, gin.H{
		"metricKey":   spec.Key,
		"title":       spec.Title,
		"unit":        spec.Unit,
		"granularity": filters.Granularity,
		"startDate":   filters.StartDate,
		"endDate":     filters.EndDate,
		"points":      points,
	})
}

// dashboardBucket is a half-open [Start, End) interval.
type dashboardBucket struct {
	Start time.Time
	End   time.Time
}

// dashboardBuckets splits the window into contiguous, non-overlapping buckets.
// The final bucket is clamped to endDate so a partial period is not overcounted.
func dashboardBuckets(start, end time.Time, granularity string) []dashboardBucket {
	buckets := make([]dashboardBucket, 0, 64)
	cursor := truncateToGranularity(start, granularity)

	for cursor.Before(end) {
		next := advanceGranularity(cursor, granularity)
		bucketStart := cursor
		if bucketStart.Before(start) {
			bucketStart = start
		}
		bucketEnd := next
		if bucketEnd.After(end) {
			bucketEnd = end
		}
		buckets = append(buckets, dashboardBucket{Start: bucketStart, End: bucketEnd})
		cursor = next

		if len(buckets) > dashboardMaxSeriesPoints {
			break
		}
	}
	return buckets
}

func truncateToGranularity(t time.Time, granularity string) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	switch granularity {
	case "week":
		// Align to Monday.
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return day
	}
}

func advanceGranularity(t time.Time, granularity string) time.Time {
	switch granularity {
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}
