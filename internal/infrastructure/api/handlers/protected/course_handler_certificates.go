package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// User-facing Certificate Endpoints (self-service, scoped to the
// authenticated user only - not to be confused with the admin
// certificate-template management endpoints in
// certificate_template_handler.go)
// =============================================================

// certificateResponse is the enriched shape returned to the frontend:
// the raw LmsCertificate row plus the course title, so the client does
// not need a second request per certificate.
type certificateResponse struct {
	ID            uuid.UUID `json:"id"`
	CourseID      uuid.UUID `json:"courseId"`
	CourseTitle   string    `json:"courseTitle"`
	UserID        uuid.UUID `json:"userId"`
	CertificateNo string    `json:"certificateNo"`
	QRCodeURL     *string   `json:"qrCodeUrl,omitempty"`
	PDFURL        string    `json:"pdfUrl"`
	IssuedAt      string    `json:"issuedAt"`
	CreatedAt     string    `json:"createdAt"`
}

// courseTitlesByID looks up course titles for a set of course IDs in a
// single query, returning a map for O(1) lookup while building responses.
func (h *CourseRESTHandler) courseTitlesByID(courseIDs []uuid.UUID) map[uuid.UUID]string {
	titles := make(map[uuid.UUID]string, len(courseIDs))
	if len(courseIDs) == 0 {
		return titles
	}

	var courses []models.LmsCourse
	if err := h.db.Model(&models.LmsCourse{}).
		Select("id", "title").
		Where("id IN ?", courseIDs).
		Find(&courses).Error; err != nil {
		return titles
	}

	for _, course := range courses {
		titles[course.ID] = course.Title
	}
	return titles
}

func toCertificateResponse(cert models.LmsCertificate, courseTitle string) certificateResponse {
	return certificateResponse{
		ID:            cert.ID,
		CourseID:      cert.CourseID,
		CourseTitle:   courseTitle,
		UserID:        cert.UserID,
		CertificateNo: cert.CertificateNo,
		QRCodeURL:     cert.QRCodeURL,
		PDFURL:        cert.PDFURL,
		IssuedAt:      cert.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:     cert.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ListMyCertificates returns every certificate earned by the authenticated
// user, enriched with the course title.
func (h *CourseRESTHandler) ListMyCertificates(c *gin.Context) {
	userIDRaw, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userIDStr, ok := userIDRaw.(string)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Invalid user ID in session")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		api_response.Error(c, http.StatusUnauthorized, "Invalid user ID in session")
		return
	}

	certs, err := h.courseService.ListUserCertificates(userID)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to list certificates", err)
		return
	}

	courseIDs := make([]uuid.UUID, 0, len(certs))
	for _, cert := range certs {
		courseIDs = append(courseIDs, cert.CourseID)
	}
	titles := h.courseTitlesByID(courseIDs)

	response := make([]certificateResponse, 0, len(certs))
	for _, cert := range certs {
		response = append(response, toCertificateResponse(cert, titles[cert.CourseID]))
	}

	api_response.Success(c, gin.H{"certificates": response})
}

// GetMyCertificate returns a single certificate for the authenticated user
// and the given course, or 404 if none has been issued.
func (h *CourseRESTHandler) GetMyCertificate(c *gin.Context) {
	userIDRaw, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userIDStr, ok := userIDRaw.(string)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Invalid user ID in session")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		api_response.Error(c, http.StatusUnauthorized, "Invalid user ID in session")
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	cert, err := h.courseService.GetCertificate(courseID, userID)
	if err != nil || cert == nil {
		api_response.Error(c, http.StatusNotFound, "Certificate not found")
		return
	}

	titles := h.courseTitlesByID([]uuid.UUID{cert.CourseID})
	api_response.Success(c, gin.H{"certificate": toCertificateResponse(*cert, titles[cert.CourseID])})
}
