package protected

import (
	"net/http"
	"time"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ExportJourneys exports journey data for analysis
// @Summary Export user journeys
// @Description Export user journey data to CSV or JSON
// @Tags admin,analytics
// @Accept json
// @Produce json
// @Param format query string false "Export format (csv|json)"
// @Param request body map[string]interface{} false "Filter parameters"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/analytics/journeys/export [post]
func ExportJourneys(c *gin.Context) {
	format := c.Query("format")
	if format == "" {
		format = "csv"
	}

	var filters struct {
		UserID string    `json:"userId"`
		From   time.Time `json:"from"`
		To     time.Time `json:"to"`
	}

	if err := c.ShouldBindJSON(&filters); err != nil {
		// Continue without filters
	}

	// Fetch journeys
	query := db.ReadDB().Model(&models.UserJourney{}).Preload("Steps")

	if filters.UserID != "" {
		query = query.Where("user_id = ?", filters.UserID)
	}

	if !filters.From.IsZero() {
		query = query.Where(startedAtGteQuery, filters.From)
	}

	if !filters.To.IsZero() {
		query = query.Where("started_at <= ?", filters.To)
	}

	var journeys []models.UserJourney
	if err := query.Find(&journeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch journeys"})
		return
	}

	// Export based on format
	switch format {
	case "json":
		c.JSON(http.StatusOK, gin.H{
			"journeys": journeys,
		})
	case "csv":
		csvData := generateJourneysCSV(journeys)
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=journeys.csv")
		c.String(http.StatusOK, csvData)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format"})
	}
}

// generateJourneysCSV converts a slice of journeys to CSV format.
func generateJourneysCSV(journeys []models.UserJourney) string {
	// Simple CSV generation - in production, use a proper CSV library
	var csv string
	csv += "journey_id,user_id,session_id,started_at,ended_at,duration,completed,goal,step_id,step_page,step_action,step_timestamp\n"

	for _, journey := range journeys {
		if len(journey.Steps) == 0 {
			csv += formatJourneyRow(journey, models.UserJourneyStep{}) + "\n"
		} else {
			for _, step := range journey.Steps {
				csv += formatJourneyRow(journey, step) + "\n"
			}
		}
	}

	return csv
}

// formatJourneyRow formats a single journey+step pair as a CSV row.
func formatJourneyRow(journey models.UserJourney, step models.UserJourneyStep) string {
	// Simplified — escape properly in production
	completedStr := "false"
	if journey.Completed {
		completedStr = "true"
	}
	return journey.ID + "," +
		journey.UserID + "," +
		journey.SessionID + "," +
		journey.StartedAt.Format(time.RFC3339) + "," +
		journey.EndedAt.Format(time.RFC3339) + "," +
		string(rune(journey.TotalDuration)) + "," +
		completedStr + "," +
		journey.ConversionGoal + "," +
		step.ID + "," +
		step.Page + "," +
		step.Action + "," +
		step.Timestamp.Format(time.RFC3339)
}
