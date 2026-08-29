package protected

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	courseservice "thanawy-backend/internal/domain/course/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UpdateCourse updates an existing course
func (h *CourseRESTHandler) UpdateCourse(c *gin.Context) {
	id := c.Param("id")

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if id == "" {
		var idBody struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(bodyBytes, &idBody); err == nil && idBody.ID != "" {
			id = idBody.ID
		}
	}

	var req UpdateCourseRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if req.Title == nil {
		if req.NameAr != nil && *req.NameAr != "" {
			req.Title = req.NameAr
		} else if req.Name != nil && *req.Name != "" {
			req.Title = req.Name
		}
	}
	if req.PrimaryInstructorID == nil && req.InstructorID != nil && *req.InstructorID != "" {
		req.PrimaryInstructorID = req.InstructorID
	}

	// A JSON `null` unmarshals into a nil *string identically to the key being
	// absent, so UpdateCourseCommand can't tell "leave it alone" apart from
	// "clear it" for CertificateTemplate. Detect an explicit `null` from the
	// raw body so removeCertificateTemplate() (which sends exactly that) can
	// actually clear the stored template instead of silently no-op'ing.
	clearCertificateTemplate := false
	if req.CertificateTemplate == nil {
		var presence map[string]json.RawMessage
		if err := json.Unmarshal(bodyBytes, &presence); err == nil {
			if raw, ok := presence["certificateTemplate"]; ok && string(raw) == "null" {
				clearCertificateTemplate = true
			}
		}
	}

	var courseID uuid.UUID
	if parsed, err := uuid.Parse(id); err == nil {
		courseID = parsed
	}

	cmd := courseservice.UpdateCourseCommand{
		ID:                       courseID,
		CourseID:                 id,
		Title:                    req.Title,
		Slug:                     req.Slug,
		ShortDescription:         req.ShortDescription,
		LongDescription:          req.LongDescription,
		CoverImageURL:            req.CoverImageURL,
		PromoVideoURL:            req.PromoVideoURL,
		Level:                    req.Level,
		Language:                 req.Language,
		EstimatedDurationMins:    req.EstimatedDurationMins,
		HasCertificate:           req.HasCertificate,
		CertificateTemplate:      req.CertificateTemplate,
		ClearCertificateTemplate: clearCertificateTemplate,
		MaxStudents:              req.MaxStudents,
		IsFeatured:               req.IsFeatured,
		IsTrending:               req.IsTrending,
		IsNew:                    req.IsNew,
		SEOTitle:                 req.SEOTitle,
		SEODescription:           req.SEODescription,
		SEOKeywords:              req.SEOKeywords,
		PrerequisitesText:        req.PrerequisitesText,
		TargetAudience:           req.TargetAudience,
		LearningOutcomes:         req.LearningOutcomes,
		PrimaryInstructorID:      req.PrimaryInstructorID,
		CategoryIDs:              req.CategoryIDs,
	}

	courseEntity, err := h.updateCourseHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		if errors.Is(err, courseservice.ErrCertificateTemplateNotFound) {
			api_response.Error(c, http.StatusBadRequest, "Certificate template not found")
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update course", err)
		return
	}

	api_response.Success(c, gin.H{"course": courseEntity})
}
