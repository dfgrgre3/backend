package http

import (
	"net/http"
	"strconv"

	"thanawy-backend/internal/domain/certificate"

	"github.com/gin-gonic/gin"
)

type CertificateHandler struct {
	service *certificate.Service
}

func NewCertificateHandler(service *certificate.Service) *CertificateHandler {
	return &CertificateHandler{service: service}
}

// GetCertificate returns a certificate by ID
// GET /api/certificates/:id
func (h *CertificateHandler) GetCertificate(c *gin.Context) {
	certID := c.Param("id")

	details, err := h.service.GetCertificate(c.Request.Context(), certID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Certificate not found"})
		return
	}

	// Check ownership
	userID := c.GetString("userId")
	if userID != "" && details.User.ID != userID {
		// Allow admins to view any certificate
		role := c.GetString("role")
		if role != "ADMIN" && role != "SUPER_ADMIN" && role != "MODERATOR" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this certificate"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"certificate": details,
	})
}

// ListMyCertificates returns certificates for the authenticated user
// GET /api/certificates
func (h *CertificateHandler) ListMyCertificates(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	result, err := h.service.ListUserCertificates(c.Request.Context(), userID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch certificates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"certificates": result.Certificates,
		"pagination": gin.H{
			"page":       result.Page,
			"limit":      result.Limit,
			"total":      result.Total,
			"totalPages": result.TotalPages,
		},
	})
}

// GetCertificateByCourse returns a certificate for a specific course
// GET /api/courses/:id/certificate
func (h *CertificateHandler) GetCertificateByCourse(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	courseID := c.Param("id")

	cert, err := h.service.GetUserCertificate(c.Request.Context(), userID, courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"certificate": nil,
			"hasCertificate": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"certificate":    cert,
		"hasCertificate": true,
	})
}

// IssueCertificate manually issues a certificate (admin only)
// POST /api/admin/certificates
func (h *CertificateHandler) IssueCertificate(c *gin.Context) {
	var input struct {
		UserID    string `json:"userId" binding:"required"`
		CourseID  string `json:"courseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId and courseId are required"})
		return
	}

	cert, err := h.service.IssueCertificate(c.Request.Context(), certificate.IssueCertificateInput{
		UserID:    input.UserID,
		SubjectID: input.CourseID,
	})
	if err != nil {
		switch err {
		case certificate.ErrAlreadyExists:
			c.JSON(http.StatusConflict, gin.H{"error": "Certificate already issued"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to issue certificate"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"certificate": cert,
		"message":     "Certificate issued successfully",
	})
}

// AdminListCertificates returns all certificates with filtering (admin only)
// GET /api/admin/certificates
func (h *CertificateHandler) AdminListCertificates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	userID := c.Query("userId")
	subjectID := c.Query("courseId")

	// If userID provided, use list by user
	if userID != "" {
		result, err := h.service.ListUserCertificates(c.Request.Context(), userID, page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list certificates"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"certificates": result.Certificates,
			"pagination": gin.H{
				"page":       result.Page,
				"limit":      result.Limit,
				"total":      result.Total,
				"totalPages": result.TotalPages,
			},
		})
		return
	}

	// Fallback: list by subject or all
	if subjectID != "" {
		result, err := h.service.ListUserCertificates(c.Request.Context(), subjectID, page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list certificates"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"certificates": result.Certificates,
			"pagination": gin.H{
				"page":       result.Page,
				"limit":      result.Limit,
				"total":      result.Total,
				"totalPages": result.TotalPages,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"certificates": []certificate.Certificate{},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      0,
			"totalPages": 0,
		},
	})
}
