package protected

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SearchResultItem represents a single unified search result returned to the frontend.
type SearchResultItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Relevance   int    `json:"relevance"`
	URL         string `json:"url"`
}

// UnifiedSearchResponse is the payload contract expected by the frontend
// AdvancedSearchSection. It is returned WITHOUT the { success, data } envelope
// because safeFetch on the client does not unwrap envelopes.
type UnifiedSearchResponse struct {
	Results []SearchResultItem `json:"results"`
	Total   int64              `json:"total"`
}

// PublicSearch performs a unified full-text search across courses (subjects),
// resources, teachers, and video lessons with server-side filtering and sorting.
//
// @Summary Unified public search
// @Description Search courses, resources, teachers, and videos with server-side filtering and relevance sorting
// @Tags search
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param limit query int false "Max results (default 10, max 50)"
// @Param type query string false "Filter by type (course,resource,teacher,video,all)"
// @Success 200 {object} UnifiedSearchResponse
// @Router /api/search [get]
func PublicSearch(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	// ── Parse query parameters ──────────────────────────────────────────
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, UnifiedSearchResponse{
			Results: []SearchResultItem{},
			Total:   0,
		})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 50 {
		limit = 50
	}

	typeFilter := strings.TrimSpace(strings.ToLower(c.Query("type")))
	if typeFilter == "" {
		typeFilter = "all"
	}

	// Valid type filters
	validTypes := map[string]bool{
		"all": true, "course": true, "resource": true,
		"teacher": true, "video": true,
	}
	if !validTypes[typeFilter] {
		typeFilter = "all"
	}

	// Build ILIKE pattern for PostgreSQL case-insensitive search
	pattern := "%" + q + "%"

	var results []SearchResultItem

	// ── Search Courses (Subject) ────────────────────────────────────────
	if typeFilter == "all" || typeFilter == "course" {
		var subjects []models.Subject
		query := database.Model(&models.Subject{}).
			Where("is_published = ? AND is_active = ?", true, true).
			Where("(name ILIKE ? OR description ILIKE ? OR short_description ILIKE ?)", pattern, pattern, pattern).
			Order("enrolled_count DESC, rating DESC, created_at DESC").
			Limit(limit)

		if err := query.Find(&subjects).Error; err == nil {
			for _, s := range subjects {
				desc := ""
				if s.ShortDescription != nil && *s.ShortDescription != "" {
					desc = *s.ShortDescription
				} else if s.Description != nil {
					desc = *s.Description
				}
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}

				category := "عام"
				if s.Level != "" {
					category = string(s.Level)
				}

				slug := ""
				if s.Slug != nil {
					slug = *s.Slug
				}
				url := fmt.Sprintf("/courses/%s", s.ID)
				if slug != "" {
					url = fmt.Sprintf("/courses/%s", slug)
				}

				relevance := computeRelevance(q, s.Name, desc)
				results = append(results, SearchResultItem{
					ID:          s.ID,
					Type:        "course",
					Title:       s.Name,
					Description: desc,
					Category:    category,
					Relevance:   relevance,
					URL:         url,
				})
			}
		}
	}

	// ── Search Resources ────────────────────────────────────────────────
	if typeFilter == "all" || typeFilter == "resource" {
		var resources []models.Resource
		query := database.Model(&models.Resource{}).
			Where("title ILIKE ? OR description ILIKE ?", pattern, pattern).
			Order("created_at DESC").
			Limit(limit)

		if err := query.Find(&resources).Error; err == nil {
			for _, r := range resources {
				desc := ""
				if r.Description != nil {
					desc = *r.Description
				}
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}

				relevance := computeRelevance(q, r.Title, desc)
				results = append(results, SearchResultItem{
					ID:          r.ID,
					Type:        "resource",
					Title:       r.Title,
					Description: desc,
					Category:    r.Type,
					Relevance:   relevance,
					URL:         fmt.Sprintf("/resources/%s", r.ID),
				})
			}
		}
	}

	// ── Search Teachers (User with role TEACHER) ────────────────────────
	if typeFilter == "all" || typeFilter == "teacher" {
		var teachers []models.User
		query := database.Model(&models.User{}).
			Where("role = ? AND status = ?", models.RoleTeacher, models.StatusActive).
			Where("name ILIKE ? OR bio ILIKE ?", pattern, pattern).
			Order("total_xp DESC, created_at DESC").
			Limit(limit)

		if err := query.Find(&teachers).Error; err == nil {
			for _, t := range teachers {
				name := t.GetName()
				desc := ""
				if t.Bio != nil {
					desc = *t.Bio
				}
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}

				category := "معلم"
				if t.InstructorStatus != "" {
					category = t.InstructorStatus
				}

				relevance := computeRelevance(q, name, desc)
				results = append(results, SearchResultItem{
					ID:          t.ID,
					Type:        "teacher",
					Title:       name,
					Description: desc,
					Category:    category,
					Relevance:   relevance,
					URL:         fmt.Sprintf("/teachers/%s", t.ID),
				})
			}
		}
	}

	// ── Search Videos (SubTopic with type VIDEO) ────────────────────────
	if typeFilter == "all" || typeFilter == "video" {
		var videos []models.SubTopic
		query := database.Model(&models.SubTopic{}).
			Where("type = ?", models.SubTopicVideo).
			Where("title ILIKE ? OR description ILIKE ?", pattern, pattern).
			Order("view_count DESC, created_at DESC").
			Limit(limit)

		if err := query.Find(&videos).Error; err == nil {
			for _, v := range videos {
				desc := ""
				if v.Description != nil {
					desc = *v.Description
				}
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}

				relevance := computeRelevance(q, v.Title, desc)
				results = append(results, SearchResultItem{
					ID:          v.ID,
					Type:        "video",
					Title:       v.Title,
					Description: desc,
					Category:    "فيديو",
					Relevance:   relevance,
					URL:         fmt.Sprintf("/lessons/%s", v.ID),
				})
			}
		}
	}

	// ── Sort by relevance (descending) and trim to limit ────────────────
	sortResultsByRelevance(results)
	if len(results) > limit {
		results = results[:limit]
	}

	total := int64(len(results))

	c.JSON(http.StatusOK, UnifiedSearchResponse{
		Results: results,
		Total:   total,
	})
}

// computeRelevance calculates a simple relevance score (0-100) based on
// how well the query matches the title and description.
// Title matches weigh more than description matches.
func computeRelevance(query, title, description string) int {
	if query == "" {
		return 0
	}

	queryLower := strings.ToLower(query)
	titleLower := strings.ToLower(title)
	descLower := strings.ToLower(description)

	score := 0

	// Exact title match → highest
	if titleLower == queryLower {
		score = 100
	} else if strings.Contains(titleLower, queryLower) {
		// Title starts with query → very high
		if strings.HasPrefix(titleLower, queryLower) {
			score = 90
		} else {
			score = 75
		}
	} else if strings.Contains(descLower, queryLower) {
		// Description match → medium
		score = 50
	} else {
		// Word-level matching for partial relevance
		queryWords := strings.Fields(queryLower)
		matchedWords := 0
		for _, word := range queryWords {
			if len(word) < 2 {
				continue
			}
			if strings.Contains(titleLower, word) {
				matchedWords++
			} else if strings.Contains(descLower, word) {
				matchedWords++
			}
		}
		if len(queryWords) > 0 {
			score = (matchedWords * 30) / len(queryWords)
		}
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	if score == 0 {
		// Ensure non-zero for display when we have a result
		score = 10
	}

	return score
}

// sortResultsByRelevance sorts results in descending order of relevance
// using a simple insertion sort (results slice is small, ≤ 50 items).
func sortResultsByRelevance(results []SearchResultItem) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Relevance > results[j-1].Relevance; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}

// Compile-time interface check to ensure the handler signature matches gin.HandlerFunc.
var _ gin.HandlerFunc = PublicSearch

// Ensure gorm is imported (used indirectly through models and db packages).
var _ = gorm.ErrRecordNotFound

// SearchHistoryRequest represents a search history tracking request
type SearchHistoryRequest struct {
	Query     string                 `json:"query" binding:"required"`
	Results   int                    `json:"results"`
	Type      string                 `json:"type"` // course, resource, teacher, video, all
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TrackSearchHistory tracks user search queries for analytics
// @Summary Track search history
// @Description Track user search queries for analytics and personalization
// @Tags search,analytics
// @Accept json
// @Produce json
// @Param request body SearchHistoryRequest true "Search history data"
// @Success 200 {object} map[string]bool
// @Router /api/ai/search/track [post]
func TrackSearchHistory(c *gin.Context) {
	var req SearchHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get user ID if authenticated (optional)
	var userID *string
	if uid, exists := c.Get("userId"); exists && uid != nil {
		if uidStr, ok := uid.(string); ok {
			userID = &uidStr
		}
	}

	// Build payload
	payload := models.JSONMap{
		"query":   req.Query,
		"results": req.Results,
		"type":    req.Type,
	}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			payload[k] = v
		}
	}

	// Capture request metadata for analytics
	if ip := c.ClientIP(); ip != "" {
		payload["ipAddress"] = ip
	}
	if ua := c.GetHeader("User-Agent"); ua != "" {
		payload["userAgent"] = ua
	}

	// Use client timestamp if provided, otherwise server time
	receivedAt := time.Now()
	if req.Timestamp > 0 {
		clientTime := time.UnixMilli(req.Timestamp)
		if clientTime.Before(receivedAt.Add(time.Hour)) {
			receivedAt = clientTime
		}
	}

	event := models.AnalyticsEvent{
		ID:         fmt.Sprintf("search-%s-%d", req.Query, time.Now().UnixNano()),
		EventType:  "search_query",
		UserID:     userID,
		Payload:    payload,
		Source:     "frontend",
		ReceivedAt: receivedAt,
	}

	// Fire-and-forget style: log but don't fail the request on DB error
	if err := db.DB.Create(&event).Error; err != nil {
		log.Printf("Failed to track search history: %v", err)
	}

	api_response.Success(c, gin.H{"success": true})
}

// GetUserSearchHistory returns the authenticated user's search history
// @Summary Get user search history
// @Description Get the authenticated user's recent search queries
// @Tags search,analytics
// @Produce json
// @Param limit query int false "Max results (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Router /api/ai/search/history [get]
func GetUserSearchHistory(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	var events []models.AnalyticsEvent
	query := db.DB.Where("event_type = ? AND user_id = ?", "search_query", userID).
		Order("received_at DESC").
		Limit(limit)

	if err := query.Find(&events).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch search history")
		return
	}

	// Extract relevant data from events
	history := make([]gin.H, 0, len(events))
	for _, event := range events {
		history = append(history, gin.H{
			"query":     event.Payload["query"],
			"results":   event.Payload["results"],
			"type":      event.Payload["type"],
			"timestamp": event.ReceivedAt,
		})
	}

	api_response.Success(c, gin.H{
		"history": history,
		"count":   len(history),
	})
}

// PromoEventRequest represents a promo event tracking request
type PromoEventRequest struct {
	PromoID   string                 `json:"promoId" binding:"required"`
	EventType string                 `json:"eventType" binding:"required"` // view, click, dismiss
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TrackPromoEvent tracks promo interaction events for analytics
// @Summary Track promo event
// @Description Track user interactions with promotional content
// @Tags analytics,promo
// @Accept json
// @Produce json
// @Param request body PromoEventRequest true "Promo event data"
// @Success 200 {object} map[string]bool
// @Router /api/analytics/promo [post]
func TrackPromoEvent(c *gin.Context) {
	var req PromoEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate event type
	validTypes := map[string]bool{"view": true, "click": true, "dismiss": true}
	if !validTypes[req.EventType] {
		api_response.Error(c, http.StatusBadRequest, "eventType must be 'view', 'click', or 'dismiss'")
		return
	}

	// Get user ID if authenticated (optional)
	var userID *string
	if uid, exists := c.Get("userId"); exists && uid != nil {
		if uidStr, ok := uid.(string); ok {
			userID = &uidStr
		}
	}

	// Build payload
	payload := models.JSONMap{
		"promoId":   req.PromoID,
		"eventType": req.EventType,
	}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			payload[k] = v
		}
	}

	// Capture request metadata for analytics
	if ip := c.ClientIP(); ip != "" {
		payload["ipAddress"] = ip
	}
	if ua := c.GetHeader("User-Agent"); ua != "" {
		payload["userAgent"] = ua
	}

	// Use client timestamp if provided, otherwise server time
	receivedAt := time.Now()
	if req.Timestamp > 0 {
		clientTime := time.UnixMilli(req.Timestamp)
		if clientTime.Before(receivedAt.Add(time.Hour)) {
			receivedAt = clientTime
		}
	}

	event := models.AnalyticsEvent{
		EventID:    fmt.Sprintf("promo-%s-%s-%d", req.PromoID, req.EventType, time.Now().UnixNano()),
		EventType:  "promo_" + req.EventType,
		UserID:     userID,
		Payload:    payload,
		Source:     "frontend",
		ReceivedAt: receivedAt,
	}

	// Fire-and-forget style: log but don't fail the request on DB error
	if err := db.DB.Create(&event).Error; err != nil {
		log.Printf("Failed to track promo event: %v", err)
	}

	api_response.Success(c, gin.H{"success": true})
}
