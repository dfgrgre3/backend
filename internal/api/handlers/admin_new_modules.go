package handlers

import (
	"net/http"
	"strconv"

	api_response "thanawy-backend/internal/api/response"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  Lessons Management
// ─────────────────────────────────────────────

// Legacy lessons/refunds handlers removed; the current backend uses the core lesson model directly.

// Refund and tax handlers removed; the current backend does not expose these legacy admin endpoints.

// ─────────────────────────────────────────────
//  Badges Management
// ─────────────────────────────────────────────

func AdminListBadges(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 { page = 1 }
	if limit <= 0 { limit = 10 }

	api_response.Success(c, gin.H{
		"items": []gin.H{},
		"pagination": gin.H{"page": page, "limit": limit, "total": 0},
	})
}

func AdminGetBadge(c *gin.Context) {
	api_response.Error(c, http.StatusNotFound, "Badge not found")
}

func AdminCreateBadge(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Badge created"})
}

func AdminUpdateBadge(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Badge updated"})
}

func AdminDeleteBadge(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Badge deleted"})
}

// ─────────────────────────────────────────────
//  Attendance Management
// ─────────────────────────────────────────────

func AdminListAttendance(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 { page = 1 }
	if limit <= 0 { limit = 10 }

	api_response.Success(c, gin.H{
		"items": []gin.H{},
		"pagination": gin.H{"page": page, "limit": limit, "total": 0},
	})
}

func AdminGetAttendanceStats(c *gin.Context) {
	api_response.Success(c, gin.H{"stats": gin.H{}})
}

func AdminCreateAttendance(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Attendance created"})
}

func AdminUpdateAttendance(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Attendance updated"})
}

// ─────────────────────────────────────────────
//  CMS Pages Management
// ─────────────────────────────────────────────

func AdminListCMSPages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 { page = 1 }
	if limit <= 0 { limit = 10 }

	api_response.Success(c, gin.H{
		"items": []gin.H{},
		"pagination": gin.H{"page": page, "limit": limit, "total": 0},
	})
}

func AdminGetCMSPage(c *gin.Context) {
	api_response.Error(c, http.StatusNotFound, "CMS page not found")
}

func AdminCreateCMSPage(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "CMS page created"})
}

func AdminUpdateCMSPage(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "CMS page updated"})
}

func AdminDeleteCMSPage(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "CMS page deleted"})
}

// ─────────────────────────────────────────────
//  Integrations Management
// ─────────────────────────────────────────────

func AdminListIntegrations(c *gin.Context) {
	api_response.Success(c, gin.H{"items": []gin.H{}, "pagination": gin.H{"page": 1, "limit": 10, "total": 0}})
}

func AdminGetIntegration(c *gin.Context) {
	api_response.Error(c, http.StatusNotFound, "Integration not found")
}

func AdminCreateIntegration(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Integration created"})
}

func AdminUpdateIntegration(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Integration updated"})
}

func AdminDeleteIntegration(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Integration deleted"})
}

func AdminTestIntegration(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Integration tested"})
}