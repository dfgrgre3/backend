package protected

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

func UpdateSubject(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("UpdateSubject: JSON Binding error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid input format: "+err.Error())
		return
	}

	id, ok := input["id"].(string)
	if !ok || id == "" {
		api_response.Error(c, http.StatusBadRequest, "Subject ID is required")
		return
	}

	var subject models.Subject
	if err := db.DB.First(&subject, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
		return
	}

	normalizeInputMap(input)
	updates, err := mapInputToSubjectUpdatesMap(input)
	if err != nil {
		log.Printf("UpdateSubject: mapping error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid array input format: "+err.Error())
		return
	}

	if err := db.DB.Model(&models.Subject{}).Where(idQuery, subject.ID).
		Updates(updates).Error; err != nil {
		log.Printf("UpdateSubject: Database error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, getUpdateSubjectErrorMessage(err))
		return
	}

	// Refresh from DB to get all fields
	db.DB.First(&subject, idQuery, id)
	getSubjectRepo().Update(&subject) // Update cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	LogAudit(c, "UPDATE", "subject", id, input)
	api_response.Success(c, gin.H{"course": subject})
}

func mapInputToSubjectUpdatesMap(input map[string]interface{}) (map[string]interface{}, error) {
	updates := make(map[string]interface{})

	// Field mapping from input key (camelCase) to db column (snake_case)
	mappings := map[string]string{
		"name":                "name",
		"nameAr":              "name_ar",
		"title":               "name_ar",
		"description":         "description",
		"categoryId":          "category_id",
		"color":               "color",
		"image":               "image",
		"code":                "code",
		"icon":                "icon",
		"instructorName":      "instructor_name",
		"instructorId":        "instructor_id",
		"primaryInstructorId": "instructor_id",
		"slug":                "slug",
		"thumbnailUrl":        "thumbnail_url",
		"coverImageUrl":       "thumbnail_url",
		"trailerUrl":          "trailer_url",
		"promoVideoUrl":       "trailer_url",
		"prerequisitesText":   "requirements",
		"seoTitle":            "seo_title",
		"seoDescription":      "seo_description",
		"level":               "level",
		"language":            "language",
		"type":                "type",
		// Lifecycle fields (migration 0064)
		"status":           "status",
		"version":          "version",
		"shortDescription": "short_description",
		"longDescription":  "long_description",
	}

	for inputKey, dbCol := range mappings {
		if val, exists := input[inputKey]; exists {
			updates[dbCol] = val
		}
	}

	// Boolean & numeric type-safe conversions or direct mappings
	numMappings := map[string]string{
		"price":                  "price",
		"durationHours":          "duration_hours",
		"trailerDurationMinutes": "trailer_duration_minutes",
		"maxStudents":            "max_students",
	}
	for inputKey, dbCol := range numMappings {
		if val, exists := input[inputKey]; exists {
			updates[dbCol] = val
		}
	}

	boolMappings := map[string]string{
		"isActive":       "is_active",
		"isPublished":    "is_published",
		"isFree":         "is_free",
		"isFeatured":     "is_featured",
		"isTrending":     "is_trending",
		"isNew":          "is_new",
		"hasCertificate": "has_certificate",
	}
	for inputKey, dbCol := range boolMappings {
		if val, exists := input[inputKey]; exists {
			updates[dbCol] = val
		}
	}

	// PGStringArray fields
	arrayFields := map[string]string{
		"coursePrerequisites": "course_prerequisites",
		"targetAudience":      "target_audience",
		"whatYouLearn":        "what_you_learn",
	}
	for inputKey, dbCol := range arrayFields {
		if val, exists := input[inputKey]; exists {
			sa, err := parseStringArray(val)
			if err != nil {
				return nil, err
			}
			if sa != nil {
				updates[dbCol] = *sa
			}
		}
	}

	// Date/time fields (accept ISO 8601 strings or null)
	dateFields := map[string]string{
		"availableFrom":  "available_from",
		"availableUntil": "available_until",
		"newUntil":       "new_until",
	}
	for inputKey, dbCol := range dateFields {
		if val, exists := input[inputKey]; exists {
			if val == nil {
				updates[dbCol] = nil
			} else if str, ok := val.(string); ok && str != "" {
				if t, err := time.Parse(time.RFC3339, str); err == nil {
					updates[dbCol] = t
				}
			}
		}
	}

	return updates, nil
}

func parseStringArray(val interface{}) (*models.PGStringArray, error) {
	if val == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	var sa models.PGStringArray
	if err := json.Unmarshal(bytes, &sa); err != nil {
		return nil, err
	}
	return &sa, nil
}

func normalizeInputMap(input map[string]interface{}) {
	pointerFields := []string{
		"code", "slug", "instructorId", "categoryId",
		"thumbnailUrl", "trailerUrl", "nameAr", "description",
		"icon", "instructorName", "seoTitle", "seoDescription",
		"shortDescription", "longDescription",
	}
	for _, field := range pointerFields {
		if val, exists := input[field]; exists {
			if str, ok := val.(string); ok && str == "" {
				input[field] = nil
			}
		}
	}
}

func getUpdateSubjectErrorMessage(err error) string {
	if strings.Contains(err.Error(), "duplicate key") {
		return "A duplicate entry was found (name, code, or slug already exists)"
	}
	return "Failed to update subject"
}
