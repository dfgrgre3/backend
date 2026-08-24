package protected

import (
	"strings"
	"time"
	models "thanawy-backend/internal/domain/common"
)

// mapInputToSubjectUpdatesMap maps camelCase input keys to their DB snake_case columns.
func mapInputToSubjectUpdatesMap(input map[string]interface{}) (map[string]interface{}, error) {
	updates := make(map[string]interface{})

	strMappings := map[string]string{
		"name": "name", "nameAr": "name_ar", "title": "name_ar",
		"description": "description", "categoryId": "category_id",
		"color": "color", "image": "image", "code": "code", "icon": "icon",
		"instructorName": "instructor_name", "instructorId": "instructor_id",
		"primaryInstructorId": "instructor_id", "slug": "slug",
		"thumbnailUrl": "thumbnail_url", "coverImageUrl": "thumbnail_url",
		"trailerUrl": "trailer_url", "promoVideoUrl": "trailer_url",
		"prerequisitesText": "requirements", "seoTitle": "seo_title",
		"seoDescription": "seo_description", "level": "level",
		"language": "language", "type": "type",
		// Lifecycle fields
		"status": "status", "version": "version",
		"shortDescription": "short_description", "longDescription": "long_description",
	}
	for k, col := range strMappings {
		if val, exists := input[k]; exists {
			updates[col] = val
		}
	}

	numMappings := map[string]string{
		"price": "price", "durationHours": "duration_hours",
		"trailerDurationMinutes": "trailer_duration_minutes", "maxStudents": "max_students",
	}
	for k, col := range numMappings {
		if val, exists := input[k]; exists {
			updates[col] = val
		}
	}

	boolMappings := map[string]string{
		"isActive": "is_active", "isPublished": "is_published",
		"isFree": "is_free", "isFeatured": "is_featured",
		"isTrending": "is_trending", "isNew": "is_new",
		"hasCertificate": "has_certificate",
	}
	for k, col := range boolMappings {
		if val, exists := input[k]; exists {
			updates[col] = val
		}
	}

	// PGStringArray fields
	arrayFields := map[string]string{
		"coursePrerequisites": "course_prerequisites",
		"targetAudience":      "target_audience",
		"whatYouLearn":        "what_you_learn",
	}
	for k, col := range arrayFields {
		if val, exists := input[k]; exists {
			sa, err := parseStringArray(val)
			if err != nil {
				return nil, err
			}
			if sa != nil {
				updates[col] = *sa
			}
		}
	}

	// Date/time fields
	dateFields := map[string]string{
		"availableFrom": "available_from", "availableUntil": "available_until",
		"newUntil": "new_until",
	}
	for k, col := range dateFields {
		if val, exists := input[k]; exists {
			if val == nil {
				updates[col] = nil
			} else if str, ok := val.(string); ok && str != "" {
				if t, err := time.Parse(time.RFC3339, str); err == nil {
					updates[col] = t
				}
			}
		}
	}

	return updates, nil
}

// normalizeInputMap converts empty strings to nil for nullable pointer fields.
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

// getUpdateSubjectErrorMessage returns a user-friendly error message.
func getUpdateSubjectErrorMessage(err error) string {
	if strings.Contains(err.Error(), "duplicate key") {
		return "A duplicate entry was found (name, code, or slug already exists)"
	}
	return "Failed to update subject"
}

// syncSubjectStatusWithPublishFlag keeps the status column consistent with isPublished.
func syncSubjectStatusWithPublishFlag(updates map[string]interface{}, subject *models.Subject) {
	if _, hasStatus := updates["status"]; hasStatus {
		return
	}
	published, ok := updates["is_published"].(bool)
	if !ok {
		return
	}
	if published {
		updates["status"] = models.CourseStatusPublished
		if subject.PublishedAt == nil {
			updates["published_at"] = time.Now()
		}
		return
	}
	if subject.Status == models.CourseStatusPublished {
		updates["status"] = models.CourseStatusDraft
	}
}
