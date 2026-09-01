package protected

import (
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// AdminGetAffiliateAnalytics returns aggregated stats for the dashboard.
func AdminGetAffiliateAnalytics(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	// Window: default last 30 days
	windowDays := 30
	if v := c.Query("days"); v != "" {
		var d int
		if _, err := time.ParseDuration(v + "h"); err == nil {
			// ignore: support numeric only
			d = atoiSafe(v)
		} else {
			d = atoiSafe(v)
		}
		if d > 0 && d <= 365 {
			windowDays = d
		}
	}

	since := time.Now().AddDate(0, 0, -windowDays)

	// Headline counts
	var (
		totalAffiliates    int64
		activeAffiliates   int64
		pendingAffiliates  int64
		totalClicks        int64
		totalConversions   int64
		totalCommission    float64
		pendingCommission  float64
		paidCommission     float64
		totalCampaigns     int64
		activeCampaigns    int64
		totalLinks         int64
		totalPayouts       int64
		pendingPayouts     int64
		paidPayouts        int64
	)

	database.Model(&models.Affiliate{}).Count(&totalAffiliates)
	database.Model(&models.Affiliate{}).Where("status = ?", "ACTIVE").Count(&activeAffiliates)
	database.Model(&models.Affiliate{}).Where("status = ?", "PENDING").Count(&pendingAffiliates)

	database.Model(&models.Affiliate{}).
		Select("COALESCE(SUM(clicks_count),0)").Scan(&totalClicks)
	database.Model(&models.Affiliate{}).
		Select("COALESCE(SUM(conversions_count),0)").Scan(&totalConversions)
	database.Model(&models.Affiliate{}).
		Select("COALESCE(SUM(total_earned),0)").Scan(&totalCommission)
	database.Model(&models.AffiliateReferral{}).
		Where("status = ?", "PENDING").Select("COALESCE(SUM(commission),0)").Scan(&pendingCommission)
	database.Model(&models.AffiliateReferral{}).
		Where("status = ?", "PAID").Select("COALESCE(SUM(commission),0)").Scan(&paidCommission)

	database.Model(&models.AffiliateCampaign{}).Count(&totalCampaigns)
	database.Model(&models.AffiliateCampaign{}).Where("status = ?", "ACTIVE").Count(&activeCampaigns)
	database.Model(&models.AffiliateLink{}).Count(&totalLinks)
	database.Model(&models.AffiliatePayout{}).Count(&totalPayouts)
	database.Model(&models.AffiliatePayout{}).Where("status = ?", "PENDING").Count(&pendingPayouts)
	database.Model(&models.AffiliatePayout{}).Where("status = ?", "PAID").Count(&paidPayouts)

	// Time series: clicks / conversions per day
	type dayRow struct {
		Day         string
		Clicks      int
		Conversions int
	}
	var clickRows []dayRow
	database.Raw(`
		SELECT TO_CHAR(date_trunc('day', created_at), 'YYYY-MM-DD') AS day,
		       COUNT(*) FILTER (WHERE converted = false) AS clicks,
		       COUNT(*) FILTER (WHERE converted = true) AS conversions
		FROM "AffiliateLinkClick"
		WHERE created_at >= ?
		GROUP BY day
		ORDER BY day ASC
	`, since).Scan(&clickRows)

	// Earnings per day (commissions)
	type earningRow struct {
		Day     string
		Pending float64
		Paid    float64
	}
	var earnRows []earningRow
	database.Raw(`
		SELECT TO_CHAR(date_trunc('day', created_at), 'YYYY-MM-DD') AS day,
		       COALESCE(SUM(commission) FILTER (WHERE status = 'PENDING'), 0) AS pending,
		       COALESCE(SUM(commission) FILTER (WHERE status = 'PAID'), 0)    AS paid
		FROM "AffiliateReferral"
		WHERE created_at >= ?
		GROUP BY day
		ORDER BY day ASC
	`, since).Scan(&earnRows)

	// Tier distribution
	type tierRow struct {
		Tier   string
		Count  int
		Total  float64
	}
	var tierRows []tierRow
	database.Raw(`
		SELECT tier AS tier, COUNT(*) AS count, COALESCE(SUM(total_earned),0) AS total
		FROM "Affiliate"
		GROUP BY tier
		ORDER BY total DESC
	`).Scan(&tierRows)

	// Status distribution
	type statusRow struct {
		Status string
		Count  int
	}
	var statusRows []statusRow
	database.Raw(`SELECT status, COUNT(*) AS count FROM "Affiliate" GROUP BY status`).Scan(&statusRows)

	// Top affiliates
	var topAffiliates []models.Affiliate
	database.Preload("User").
		Order("total_earned DESC").Limit(5).
		Find(&topAffiliates)

	// Top campaigns
	var topCampaigns []models.AffiliateCampaign
	database.Order("spent DESC, created_at DESC").Limit(5).Find(&topCampaigns)

	api_response.Success(c, gin.H{
		"windowDays": windowDays,
		"headline": gin.H{
			"totalAffiliates":   totalAffiliates,
			"activeAffiliates":  activeAffiliates,
			"pendingAffiliates": pendingAffiliates,
			"totalClicks":       totalClicks,
			"totalConversions":  totalConversions,
			"totalCommission":   totalCommission,
			"pendingCommission": pendingCommission,
			"paidCommission":    paidCommission,
			"totalCampaigns":    totalCampaigns,
			"activeCampaigns":   activeCampaigns,
			"totalLinks":        totalLinks,
			"totalPayouts":      totalPayouts,
			"pendingPayouts":    pendingPayouts,
			"paidPayouts":       paidPayouts,
		},
		"clicksSeries":  clickRows,
		"earningsSeries": earnRows,
		"tierDistribution": tierRows,
		"statusDistribution": statusRows,
		"topAffiliates": topAffiliates,
		"topCampaigns":  topCampaigns,
	})
}

// AdminListAffiliateAudits returns recent audit log entries.
func AdminListAffiliateAudits(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	limit := atoiSafe(strings.TrimSpace(c.Query("limit")))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var audits []models.AffiliateAudit
	if err := database.Order("created_at DESC").Limit(limit).Find(&audits).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch audit log")
		return
	}
	api_response.Success(c, audits)
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}