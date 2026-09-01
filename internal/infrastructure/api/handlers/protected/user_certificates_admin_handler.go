package protected

import (
	"net/http"
	"strconv"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetUserCertificates returns the certificates issued to an arbitrary user,
// for the admin user-detail "academic" tab.
//
// The admin frontend (GET /api/admin/users/{id}/certificates?limit=N) expects
// { total, items: [{ id, title, courseName, issuedAt, grade, url }] } — see
// UserCertificateItem in src/lib/api/admin-users-api.ts. The LmsCertificate
// row has no title/grade of its own, so certificate_no is used as the title
// and grade is reported as null; courseName is resolved from LmsCourse in a
// single batched query instead of N+1 per row.
//
// This is the admin-scoped counterpart of ListMyCertificates
// (course_handler_certificates.go), which is restricted to the authenticated
// user's own certificates.
func GetUserCertificates(c *gin.Context) {
	userIDStr := c.Param("id")
	if userIDStr == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	limit := 50
	if q := c.Query("limit"); q != "" {
		if v, parseErr := strconv.Atoi(q); parseErr == nil && v > 0 {
			limit = v
		}
	}

	var total int64
	if err := db.DB.Model(&models.LmsCertificate{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to count certificates", err)
		return
	}

	var certs []models.LmsCertificate
	if err := db.DB.
		Where("user_id = ?", userID).
		Order("issued_at DESC").
		Limit(limit).
		Find(&certs).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch certificates", err)
		return
	}

	// Resolve course titles in one batched query (same approach as
	// courseTitlesByID in course_handler_certificates.go).
	courseIDs := make([]uuid.UUID, 0, len(certs))
	for _, cert := range certs {
		courseIDs = append(courseIDs, cert.CourseID)
	}
	titles := make(map[uuid.UUID]string, len(courseIDs))
	if len(courseIDs) > 0 {
		var courses []models.LmsCourse
		if err := db.DB.Model(&models.LmsCourse{}).
			Select("id", "title").
			Where("id IN ?", courseIDs).
			Find(&courses).Error; err == nil {
			for _, course := range courses {
				titles[course.ID] = course.Title
			}
		}
	}

	items := make([]gin.H, 0, len(certs))
	for _, cert := range certs {
		items = append(items, gin.H{
			"id":         cert.ID,
			"title":      cert.CertificateNo,
			"courseName": titles[cert.CourseID],
			"issuedAt":   cert.IssuedAt,
			"grade":      nil,
			"url":        cert.PDFURL,
		})
	}

	api_response.Success(c, gin.H{
		"userId": userIDStr,
		"total":  total,
		"items":  items,
	})
}
