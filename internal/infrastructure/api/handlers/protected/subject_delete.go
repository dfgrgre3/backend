package protected

import (
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func DeleteSubject(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		var input struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			log.Printf("[WARN] Failed to bind JSON input for subject deletion: %v", err)
		}
		id = input.ID
	}

	// Check admin permissions
	role, exists := c.Get("role")
	if !exists || (role != "ADMIN" && role != "SUPER_ADMIN") {
		api_response.Error(c, http.StatusForbidden, "Admin access required")
		return
	}

	log.Printf("Attempting to delete subject with ID: %q", id)

	// First, check if subject exists
	var subject models.Subject
	if err := db.DB.First(&subject, idQuery, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
			return
		}
		log.Printf("Error checking subject existence: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to verify subject")
		return
	}

	// Delete in transaction to ensure atomicity
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("[ERROR] Panic in subject deletion: %v", r)
			// Don't re-panic - just log and let the request fail gracefully
		}
	}()

	// Delete related records that don't have CASCADE constraints
	// Delete StudySessions referencing this subject
	if err := tx.Where(subjectIDQuery, id).Delete(&models.StudySession{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting study sessions for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete related study sessions")
		return
	}

	// Delete Books referencing this subject
	if err := tx.Where(subjectIDQuery, id).Delete(&models.Book{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting books for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete related books")
		return
	}

	// Delete Challenges referencing this subject
	if err := tx.Where(subjectIDQuery, id).Delete(&models.Challenge{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting challenges for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete related challenges")
		return
	}

	// Delete Payments referencing this subject (set to null instead of delete)
	if err := tx.Model(&models.Payment{}).Where(subjectIDQuery, id).Update("subject_id", nil).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating payments for subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to update related payments")
		return
	}

	// Now delete the subject (Topics, Enrollments will be cascade deleted)
	if err := tx.Delete(&subject).Error; err != nil {
		tx.Rollback()
		log.Printf("Error deleting subject %q: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete subject: "+err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction for subject %q deletion: %v", id, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to complete deletion")
		return
	}

	// Clear cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	LogAudit(c, "DELETE", "subject", id, nil)
	log.Printf("Successfully deleted subject: %q (%q)", id, subject.Name)
	api_response.Success(c, gin.H{"message": "Subject deleted successfully"})
}
