package protected

import (
	"net/http"
	courseservice "thanawy-backend/internal/domain/course/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// CreateCourse creates a new course
func (h *CourseRESTHandler) CreateCourse(c *gin.Context) {
	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if req.Title == "" {
		if req.NameAr != "" {
			req.Title = req.NameAr
		} else if req.Name != "" {
			req.Title = req.Name
		} else {
			req.Title = "كورس جديد"
		}
	}
	if req.PrimaryInstructorID == "" {
		if req.InstructorID != "" {
			req.PrimaryInstructorID = req.InstructorID
		} else if userIdVal, exists := c.Get("userId"); exists {
			if uid, ok := userIdVal.(string); ok && uid != "" {
				req.PrimaryInstructorID = uid
			}
		}
		if req.PrimaryInstructorID == "" {
			req.PrimaryInstructorID = "00000000-0000-0000-0000-000000000000"
		}
	}

	cmd := courseservice.CreateCourseCommand{
		Title:                 req.Title,
		Slug:                  req.Slug,
		ShortDescription:      req.ShortDescription,
		LongDescription:       req.LongDescription,
		CoverImageURL:         req.CoverImageURL,
		PromoVideoURL:         req.PromoVideoURL,
		Level:                 req.Level,
		Language:              req.Language,
		EstimatedDurationMins: req.EstimatedDurationMins,
		HasCertificate:        req.HasCertificate,
		CertificateTemplate:   req.CertificateTemplate,
		MaxStudents:           req.MaxStudents,
		SEOTitle:              req.SEOTitle,
		SEODescription:        req.SEODescription,
		SEOKeywords:           req.SEOKeywords,
		PrerequisitesText:     req.PrerequisitesText,
		TargetAudience:        req.TargetAudience,
		LearningOutcomes:      req.LearningOutcomes,
		PrimaryInstructorID:   req.PrimaryInstructorID,
		CategoryIDs:           req.CategoryIDs,
	}

	courseEntity, err := h.createCourseHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create course: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"course": courseEntity})
}
