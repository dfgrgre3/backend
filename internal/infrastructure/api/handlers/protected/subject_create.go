package protected

import (
	"log"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// Admin handlers
func CreateSubject(c *gin.Context) {
	// Use a flexible input struct to accept all field aliases from the frontend
	// without being constrained by Subject's binding tags (which have strict validators
	// like gt=0 on Price that block free courses).
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("CreateSubject: JSON Binding error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid input format: "+err.Error())
		return
	}

	subject, err := buildSubjectFromInput(input)
	if err != nil {
		log.Printf("CreateSubject: Build error: %v", err)
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if subject.Name == "" {
		api_response.Error(c, http.StatusBadRequest, "Course name (name) is required")
		return
	}

	normalizeSubjectFields(subject)

	log.Printf("Attempting to create subject: Name=%q, Code=%v, Slug=%v", subject.Name, subject.Code, subject.Slug)
	if err := getSubjectRepo().Create(subject); err != nil {
		log.Printf("CreateSubject: Repository error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, getCreateSubjectErrorMessage(err))
		return
	}

	LogAudit(c, "CREATE", "subject", subject.ID, subject)
	api_response.Created(c, gin.H{"course": subject, "id": subject.ID})
}

// buildSubjectFromInput maps a flexible JSON payload (with frontend field aliases) into a Subject model.
// It accepts both canonical field names (e.g. "name") and frontend aliases (e.g. "title", "nameAr").
func buildSubjectFromInput(input map[string]interface{}) (*models.Subject, error) {
	s := &models.Subject{}

	// name — accept "name", "title", "nameAr" as fallbacks
	if v, _ := input["name"].(string); v != "" {
		s.Name = strings.TrimSpace(v)
	} else if v, _ := input["title"].(string); v != "" {
		s.Name = strings.TrimSpace(v)
	}

	// nameAr
	if v, _ := input["nameAr"].(string); v != "" {
		s.NameAr = &v
	}

	// description — accept "description", "longDescription"
	if v, _ := input["description"].(string); v != "" {
		s.Description = &v
	} else if v, _ := input["longDescription"].(string); v != "" {
		s.Description = &v
	}

	// shortDescription
	if v, _ := input["shortDescription"].(string); v != "" {
		s.ShortDescription = &v
	} else if v, _ := input["simpleDescription"].(string); v != "" {
		s.ShortDescription = &v
	}

	// longDescription
	if v, _ := input["longDescription"].(string); v != "" {
		s.LongDescription = &v
	}

	// code
	if v, _ := input["code"].(string); v != "" {
		s.Code = &v
	}

	// slug
	if v, _ := input["slug"].(string); v != "" {
		s.Slug = &v
	}

	// level
	if v, _ := input["level"].(string); v != "" {
		s.Level = models.Level(v)
	} else {
		s.Level = models.LevelIntermediate
	}

	// language
	if v, _ := input["language"].(string); v != "" {
		s.Language = v
	} else {
		s.Language = "ar"
	}

	// price — accept 0 (free courses are valid)
	if v, ok := input["price"]; ok {
		switch pv := v.(type) {
		case float64:
			s.Price = decimal.NewFromFloat(pv)
		case string:
			if d, err := decimal.NewFromString(pv); err == nil {
				s.Price = d
			}
		}
	}

	// instructorId — accept "instructorId" and "primaryInstructorId"
	if v, _ := input["instructorId"].(string); v != "" {
		s.InstructorId = &v
	} else if v, _ := input["primaryInstructorId"].(string); v != "" {
		s.InstructorId = &v
	}

	// instructorName
	if v, _ := input["instructorName"].(string); v != "" {
		s.InstructorName = &v
	}

	// categoryId
	if v, _ := input["categoryId"].(string); v != "" {
		s.CategoryId = &v
	}

	// thumbnailUrl — accept "thumbnailUrl" and "coverImageUrl"
	if v, _ := input["thumbnailUrl"].(string); v != "" {
		s.ThumbnailUrl = &v
	} else if v, _ := input["coverImageUrl"].(string); v != "" {
		s.ThumbnailUrl = &v
	}

	// trailerUrl — accept "trailerUrl" and "promoVideoUrl"
	if v, _ := input["trailerUrl"].(string); v != "" {
		s.TrailerUrl = &v
	} else if v, _ := input["promoVideoUrl"].(string); v != "" {
		s.TrailerUrl = &v
	}

	// trailerDurationMinutes
	if v, ok := input["trailerDurationMinutes"]; ok {
		if fv, ok := v.(float64); ok {
			s.TrailerDurationMinutes = int(fv)
		}
	}

	// durationHours
	if v, ok := input["durationHours"]; ok {
		if fv, ok := v.(float64); ok {
			s.DurationHours = int(fv)
		}
	}

	// seoTitle
	if v, _ := input["seoTitle"].(string); v != "" {
		s.SeoTitle = &v
	}

	// seoDescription
	if v, _ := input["seoDescription"].(string); v != "" {
		s.SeoDescription = &v
	}

	// requirements
	if v, _ := input["requirements"].(string); v != "" {
		s.Requirements = &v
	} else if v, _ := input["prerequisitesText"].(string); v != "" {
		s.Requirements = &v
	}

	// learningObjectives
	if v, _ := input["learningObjectives"].(string); v != "" {
		s.LearningObjectives = &v
	}

	// boolean flags
	if v, ok := input["isActive"].(bool); ok {
		s.IsActive = v
	} else {
		s.IsActive = true // default
	}
	if v, ok := input["isPublished"].(bool); ok {
		s.IsPublished = v
	}
	if v, ok := input["isFeatured"].(bool); ok {
		s.IsFeatured = v
	}
	if v, ok := input["hasCertificate"].(bool); ok {
		s.HasCertificate = v
	}

	// maxStudents
	if v, ok := input["maxStudents"]; ok && v != nil {
		if fv, ok := v.(float64); ok {
			iv := int(fv)
			s.MaxStudents = &iv
		}
	}

	// targetAudience — accept string or array
	if v, _ := input["targetAudience"].(string); v != "" {
		s.TargetAudience = models.PGStringArray{v}
	} else if arr, ok := input["targetAudience"].([]interface{}); ok {
		for _, item := range arr {
			if str, ok := item.(string); ok && str != "" {
				s.TargetAudience = append(s.TargetAudience, str)
			}
		}
	}

	// whatYouLearn / learningOutcomes
	if v, _ := input["whatYouLearn"].(string); v != "" {
		s.WhatYouLearn = models.PGStringArray{v}
	} else if arr, ok := input["learningOutcomes"].([]interface{}); ok {
		for _, item := range arr {
			if str, ok := item.(string); ok && str != "" {
				s.WhatYouLearn = append(s.WhatYouLearn, str)
			}
		}
	}

	// coursePrerequisites
	if v, _ := input["coursePrerequisites"].(string); v != "" {
		s.CoursePrerequisites = models.PGStringArray{v}
	} else if arr, ok := input["coursePrerequisites"].([]interface{}); ok {
		for _, item := range arr {
			if str, ok := item.(string); ok && str != "" {
				s.CoursePrerequisites = append(s.CoursePrerequisites, str)
			}
		}
	}

	return s, nil
}

func normalizeSubjectFields(s *models.Subject) {
	if s.Code != nil && *s.Code == "" {
		s.Code = nil
	}
	if s.Slug != nil && *s.Slug == "" {
		s.Slug = nil
	}
	if s.InstructorId != nil && *s.InstructorId == "" {
		s.InstructorId = nil
	}
	if s.CategoryId != nil && *s.CategoryId == "" {
		s.CategoryId = nil
	}
}

func getCreateSubjectErrorMessage(err error) string {
	if !strings.Contains(err.Error(), "duplicate key") {
		return "Failed to create subject"
	}
	if strings.Contains(err.Error(), "Subject_name_key") {
		return "A course with this name already exists"
	}
	if strings.Contains(err.Error(), "Subject_code_key") {
		return "A course with this code already exists"
	}
	if strings.Contains(err.Error(), "Subject_slug_key") {
		return "A course with this slug already exists"
	}
	return "A duplicate entry was found"
}
