package protected

import (
	"net/http"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetUserJourneys returns user journeys with filtering
// @Summary Get user journeys
// @Description Get user journey data with filtering options
// @Tags admin,analytics
// @Accept json
// @Produce json
// @Param userId    query string false "Filter by user ID"
// @Param from      query string false "Filter from date (RFC3339)"
// @Param to        query string false "Filter to date (RFC3339)"
// @Param goal      query string false "Filter by conversion goal"
// @Param completed query bool   false "Filter by completion status"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/analytics/journeys [get]
func GetUserJourneys(c *gin.Context) {
	userID := c.Query("userId")
	from := c.Query("from")
	to := c.Query("to")
	goal := c.Query("goal")
	completed := c.Query("completed")

	query := db.ReadDB().Model(&models.UserJourney{}).Preload("Steps")

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if from != "" {
		if fromTime, err := time.Parse(time.RFC3339, from); err == nil {
			query = query.Where(startedAtGteQuery, fromTime)
		}
	}

	if to != "" {
		if toTime, err := time.Parse(time.RFC3339, to); err == nil {
			query = query.Where("started_at <= ?", toTime)
		}
	}

	if goal != "" {
		query = query.Where("conversion_goal = ?", goal)
	}

	if completed != "" {
		switch completed {
		case "true":
			query = query.Where("completed = ?", true)
		case "false":
			query = query.Where("completed = ?", false)
		}
	}

	query = query.Order("started_at DESC")

	var journeys []models.UserJourney
	if err := query.Find(&journeys).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch journeys")
		return
	}

	api_response.Success(c, gin.H{
		"journeys": journeys,
		"count":    len(journeys),
	})
}

// GetActivityMetrics returns aggregated activity metrics
// @Summary Get activity metrics
// @Description Get aggregated user activity metrics
// @Tags admin,analytics
// @Accept json
// @Produce json
// @Param from query string false "From date (RFC3339)"
// @Param to   query string false "To date (RFC3339)"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/analytics/metrics [get]
func GetActivityMetrics(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")

	var fromTime, toTime time.Time
	if from != "" {
		fromTime, _ = time.Parse(time.RFC3339, from)
	} else {
		fromTime = time.Now().AddDate(0, 0, -30) // Default to last 30 days
	}

	if to != "" {
		toTime, _ = time.Parse(time.RFC3339, to)
	} else {
		toTime = time.Now()
	}

	metrics := calculateActivityMetrics(fromTime, toTime)

	api_response.Success(c, metrics)
}

// calculateActivityMetrics calculates various activity metrics for the given time range.
func calculateActivityMetrics(from, to time.Time) ActivityMetrics {
	metrics := ActivityMetrics{
		TopPages:        []PageStats{},
		UserFlows:       []FlowStats{},
		ConversionRates: make(map[string]float64),
	}

	// Calculate DAU, WAU, MAU
	now := time.Now()
	db.ReadDB().Model(&models.UserJourney{}).
		Where(startedAtGteQuery, now.AddDate(0, 0, -1)).
		Select(countDistinctUserQuery).
		Scan(&metrics.DailyActiveUsers)

	db.ReadDB().Model(&models.UserJourney{}).
		Where(startedAtGteQuery, now.AddDate(0, 0, -7)).
		Select(countDistinctUserQuery).
		Scan(&metrics.WeeklyActiveUsers)

	db.ReadDB().Model(&models.UserJourney{}).
		Where(startedAtGteQuery, now.AddDate(0, 0, -30)).
		Select(countDistinctUserQuery).
		Scan(&metrics.MonthlyActiveUsers)

	// Calculate average session duration
	db.ReadDB().Model(&models.UserJourney{}).
		Where("started_at >= ? AND started_at <= ?", from, to).
		Select("COALESCE(AVG(total_duration), 0)").
		Scan(&metrics.AverageSessionDuration)

	// Calculate bounce rate (sessions with only 1 step)
	var totalSessions, bouncedSessions int64
	db.ReadDB().Model(&models.UserJourney{}).
		Where("started_at >= ? AND started_at <= ?", from, to).
		Count(&totalSessions)

	db.ReadDB().Raw(`
		SELECT COUNT(*) FROM user_journeys j
		JOIN user_journey_steps s ON s.journey_id = j.id AND s.deleted_at IS NULL
		WHERE j.started_at >= ? AND j.started_at <= ? AND j.deleted_at IS NULL
		GROUP BY j.id
		HAVING COUNT(s.id) = 1
	`, from, to).Scan(&bouncedSessions)

	if totalSessions > 0 {
		metrics.BounceRate = float64(bouncedSessions) / float64(totalSessions) * 100
	}

	// Top pages
	db.ReadDB().Raw(`
		SELECT page, COUNT(*) as views, COUNT(DISTINCT user_id) as unique_visitors
		FROM user_journey_steps
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY page
		ORDER BY views DESC
		LIMIT 10
	`, from, to).Scan(&metrics.TopPages)

	// User flows (page transitions)
	db.ReadDB().Raw(`
		WITH step_pairs AS (
			SELECT 
				l.page as from_page,
				LEAD(l.page) OVER (PARTITION BY l.session_id ORDER BY l.timestamp) as to_page
			FROM user_journey_steps l
			WHERE l.timestamp >= ? AND l.timestamp <= ?
		)
		SELECT from_page as from, to_page as to, COUNT(*) as count
		FROM step_pairs
		WHERE to_page IS NOT NULL
		GROUP BY from_page, to_page
		ORDER BY count DESC
		LIMIT 20
	`, from, to).Scan(&metrics.UserFlows)

	// Conversion rates by goal
	var goals []struct {
		Goal     string
		Total    int64
		Achieved int64
	}
	db.ReadDB().Raw(`
		SELECT 
			conversion_goal as goal,
			COUNT(*) as total,
			SUM(CASE WHEN completed THEN 1 ELSE 0 END) as achieved
		FROM user_journeys
		WHERE started_at >= ? AND started_at <= ? AND conversion_goal IS NOT NULL
		GROUP BY conversion_goal
	`, from, to).Scan(&goals)

	for _, g := range goals {
		if g.Total > 0 {
			metrics.ConversionRates[g.Goal] = float64(g.Achieved) / float64(g.Total) * 100
		}
	}

	return metrics
}
