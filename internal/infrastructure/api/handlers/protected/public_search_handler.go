package protected

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
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

	// Build ILIKE pattern for PostgreSQL case-insensitive search. Escape
	// ILIKE's own wildcard characters (% and _) in the user's query so a
	// literal search for e.g. "100%" or "a_b" matches that exact text
	// instead of being interpreted as a pattern.
	pattern := "%" + escapeLikePattern(q) + "%"

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

// escapeLikePattern escapes the wildcard characters PostgreSQL's LIKE/ILIKE
// operator treats specially (%, _, and the escape character itself, \) so a
// user's literal search text is matched literally rather than as a pattern.
func escapeLikePattern(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)
	return replacer.Replace(s)
}
