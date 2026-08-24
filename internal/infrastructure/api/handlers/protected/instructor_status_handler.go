package protected

import (
	"errors"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ApproveInstructor(c *gin.Context) {
	changeInstructorStatus(c, instructorStatusApproved)
}

func RejectInstructor(c *gin.Context) {
	changeInstructorStatus(c, instructorStatusRejected)
}

func SuspendInstructor(c *gin.Context) {
	changeInstructorStatus(c, instructorStatusSuspended)
}

func changeInstructorStatus(c *gin.Context, status string) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	normalizedStatus := normalizeInstructorStatusDefault(status)

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}

		apiresponse.Error(c, http.StatusInternalServerError, "Failed to update instructor status")
		return
	}

	if user.InstructorStatus != normalizedStatus {
		if err := database.Model(&user).Update("instructor_status", normalizedStatus).Error; err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to update instructor status")
			return
		}
	}

	apiresponse.Success(c, gin.H{"status": normalizedStatus})
}

func GetInstructorStatistics(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	baseQuery := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher)
	summary, err := instructorStatusSummary(baseQuery)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor statistics")
		return
	}

	apiresponse.Success(c, summary)
}

func GetInstructorDocuments(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor documents")
		return
	}

	documents := []gin.H{
		{"documentId": "identity", "name": "Identity Document", "status": "PENDING"},
		{"documentId": "qualification", "name": "Qualification Certificate", "status": "PENDING"},
		{"documentId": "portfolio", "name": "Teaching Portfolio", "status": "PENDING"},
	}

	apiresponse.Success(c, gin.H{"documents": documents})
}

func ReviewInstructorDocument(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	documentID := strings.TrimSpace(c.Param("documentId"))
	if id == "" || documentID == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id and document id are required")
		return
	}

	var input instructorReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to review instructor document")
		return
	}

	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "APPROVED"
	}

	logEntry := models.AuditLog{
		UserID:     &user.ID,
		EventType:  "instructor_document_review",
		Action:     "review",
		Resource:   "instructor_document",
		ResourceID: documentID,
		Changes:    input.Notes,
		Metadata:   `{"status":"` + status + `"}`,
		CreatedAt:  time.Now(),
	}
	if err := SafeCreate(database, &logEntry); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to record document review")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"documentId":   documentID,
		"instructorId": user.ID,
		"status":       status,
		"notes":        input.Notes,
		"reviewedAt":   logEntry.CreatedAt,
	})
}

func GetInstructorContracts(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor contracts")
		return
	}

	contracts := []gin.H{{
		"id":           "contract-" + user.ID,
		"instructorId": user.ID,
		"title":        "Standard Instructor Contract",
		"status":       "DRAFT",
		"createdAt":    time.Now(),
	}}

	apiresponse.Success(c, gin.H{"contracts": contracts})
}

func CreateInstructorContract(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var input instructorContractInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor contract")
		return
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Standard Instructor Contract"
	}

	apiresponse.Success(c, gin.H{
		"id":           "contract-" + user.ID,
		"instructorId": user.ID,
		"title":        title,
		"notes":        input.Notes,
		"status":       "DRAFT",
		"createdAt":    time.Now(),
	})
}

func GetInstructorPerformance(c *gin.Context) {
	apiresponse.Success(c, gin.H{"performance": []gin.H{}})
}

func GetInstructorViolations(c *gin.Context) {
	apiresponse.Success(c, gin.H{"violations": []gin.H{}})
}

func CreateInstructorViolation(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var input instructorViolationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor violation")
		return
	}

	typeValue := strings.ToLower(strings.TrimSpace(input.Type))
	if typeValue == "" {
		typeValue = "policy"
	}
	severity := strings.ToLower(strings.TrimSpace(input.Severity))
	if severity == "" {
		severity = "medium"
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = "No details provided"
	}

	logEntry := models.AuditLog{
		UserID:     &user.ID,
		EventType:  "instructor_violation",
		Action:     "create",
		Resource:   "instructor_violation",
		ResourceID: user.ID,
		Changes:    description,
		Metadata:   `{"type":"` + typeValue + `","severity":"` + severity + `"}`,
		CreatedAt:  time.Now(),
	}
	if err := SafeCreate(database, &logEntry); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to record instructor violation")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":      true,
		"id":           logEntry.ID,
		"instructorId": user.ID,
		"type":         typeValue,
		"description":  description,
		"severity":     severity,
		"status":       "OPEN",
		"createdAt":    logEntry.CreatedAt,
	})
}

func ResolveInstructorViolation(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to resolve instructor violation")
		return
	}

	apiresponse.Success(c, gin.H{
		"instructorId": user.ID,
		"status":       "RESOLVED",
		"resolvedAt":   time.Now(),
	})
}
