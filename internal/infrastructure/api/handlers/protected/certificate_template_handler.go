package protected

import (
	"net/http"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Certificate Template Endpoints (shared, global library)
// =============================================================

// CreateCertificateTemplateRequest represents the REST request body for
// creating a certificate template.
type CreateCertificateTemplateRequest struct {
	Name         string `json:"name" binding:"required"`
	TemplateHTML string `json:"templateHtml" binding:"required"`
	IsDefault    bool   `json:"isDefault"`
}

// ListCertificateTemplates lists every certificate template in the shared library.
func (h *CourseRESTHandler) ListCertificateTemplates(c *gin.Context) {
	templates, err := h.courseService.ListCertificateTemplates()
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to list certificate templates", err)
		return
	}

	api_response.Success(c, gin.H{"templates": templates})
}

// CreateCertificateTemplate adds a new certificate template to the shared library.
func (h *CourseRESTHandler) CreateCertificateTemplate(c *gin.Context) {
	var req CreateCertificateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	template, err := h.courseService.CreateCertificateTemplate(req.Name, req.TemplateHTML, req.IsDefault)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to create certificate template", err)
		return
	}

	api_response.Created(c, gin.H{"template": template})
}

// DeleteCertificateTemplate removes a certificate template from the shared library.
func (h *CourseRESTHandler) DeleteCertificateTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	parsedID, err := uuid.Parse(templateID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid template ID")
		return
	}

	if err := h.courseService.DeleteCertificateTemplate(parsedID); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to delete certificate template", err)
		return
	}

	api_response.Success(c, gin.H{"message": "Certificate template deleted successfully"})
}
