package protected

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PublicAffiliateRedirect resolves /r/:code/:slug, records a click and returns
// the destination URL plus cookie metadata. The Next.js edge route at
// `/r/[code]/[slug]/page.tsx` calls this and performs the actual 302 redirect
// to keep the public landing URL short.
func PublicAffiliateRedirect(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	code := strings.TrimSpace(c.Param("code"))
	slug := strings.TrimSpace(c.Param("slug"))
	if code == "" || slug == "" {
		api_response.Error(c, http.StatusBadRequest, "Missing affiliate code or link slug")
		return
	}

	// Find the affiliate by code
	var affiliate models.Affiliate
	if err := database.Where("code = ?", code).First(&affiliate).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Affiliate not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to resolve affiliate")
		return
	}

	// Find the link scoped to the affiliate
	var link models.AffiliateLink
	if err := database.Where("affiliate_id = ? AND slug = ?", affiliate.ID, slug).First(&link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Affiliate link not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to resolve link")
		return
	}

	if !link.Active {
		api_response.Error(c, http.StatusGone, "Affiliate link is disabled")
		return
	}
	if affiliate.Status != "ACTIVE" {
		api_response.Error(c, http.StatusForbidden, "Affiliate is not active")
		return
	}

	// Record the click — same logic as the admin endpoint, but no auth required
	ip := strings.TrimSpace(c.ClientIP())
	ipHash := ""
	if ip != "" {
		sum := sha256.Sum256([]byte(ip + "|aff-salt"))
		ipHash = hex.EncodeToString(sum[:])
	}
	userAgent := strings.TrimSpace(c.GetHeader("User-Agent"))
	referer := strings.TrimSpace(c.GetHeader("Referer"))
	country := strings.TrimSpace(c.GetHeader("CF-IPCountry"))

	device := detectDevice(userAgent)

	click := models.AffiliateLinkClick{
		LinkID:      link.ID,
		AffiliateID: affiliate.ID,
		IPHash:      &ipHash,
		UserAgent:   strOrNil(userAgent),
		Referer:     strOrNil(referer),
		Country:     strOrNil(country),
		Device:      device,
		Converted:   false,
	}
	if err := SafeCreate(database, &click); err != nil {
		// Non-fatal: log but continue to redirect
		fmt.Printf("[affiliate-redirect] click record failed: %v\n", err)
	} else {
		database.Model(&models.AffiliateLink{}).Where(queryID, link.ID).Updates(map[string]interface{}{
			"clicks_count": gorm.Expr("clicks_count + 1"),
		})
		database.Model(&models.Affiliate{}).Where(queryID, affiliate.ID).Updates(map[string]interface{}{
			"clicks_count":       gorm.Expr("clicks_count + 1"),
			"last_activity_at":   time.Now().UTC(),
		})
	}

	// Build the destination URL with any utm_* appended
	dest := link.DestinationURL
	if dest == "" {
		api_response.Error(c, http.StatusFailedDependency, "Link has no destination")
		return
	}

	// Settings cookie window — read settings if present, else default 30 days
	cookieDays := 30
	var setting models.AffiliateSetting
	if err := database.First(&setting).Error; err == nil && setting.CookieDays > 0 {
		cookieDays = setting.CookieDays
	}

	api_response.Success(c, gin.H{
		"destinationUrl":   dest,
		"affiliateCode":    affiliate.Code,
		"linkId":           link.ID,
		"affiliateId":      affiliate.ID,
		"campaignId":       link.CampaignID,
		"cookieName":       "aff_ref",
		"cookieDays":       cookieDays,
		"utm": gin.H{
			"source":   link.UTMSource,
			"medium":   link.UTMMedium,
			"campaign": link.UTMCampaign,
		},
		"recorded": true,
	})
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func detectDevice(ua string) *string {
	if ua == "" {
		return nil
	}
	low := strings.ToLower(ua)
	switch {
	case strings.Contains(low, "ipad"), strings.Contains(low, "tablet"):
		return strPtr("tablet")
	case strings.Contains(low, "android"), strings.Contains(low, "iphone"), strings.Contains(low, "mobile"):
		return strPtr("mobile")
	default:
		return strPtr("desktop")
	}
}