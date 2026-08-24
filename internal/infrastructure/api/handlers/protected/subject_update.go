package protected

import (
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func UpdateSubject(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("UpdateSubject: JSON Binding error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid input format: "+err.Error())
		return
	}

	id, ok := input["id"].(string)
	if !ok || id == "" {
		api_response.Error(c, http.StatusBadRequest, "Subject ID is required")
		return
	}

	var subject models.Subject
	if err := db.DB.First(&subject, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
		return
	}

	normalizeInputMap(input)
	updates, err := mapInputToSubjectUpdatesMap(input)
	if err != nil {
		log.Printf("UpdateSubject: mapping error: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Invalid array input format: "+err.Error())
		return
	}
	syncSubjectStatusWithPublishFlag(updates, &subject)

	if err := db.DB.Model(&models.Subject{}).Where(idQuery, subject.ID).
		Updates(updates).Error; err != nil {
		log.Printf("UpdateSubject: Database error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, getUpdateSubjectErrorMessage(err))
		return
	}

	// Refresh from DB to get all fields
	db.DB.First(&subject, idQuery, id)
	getSubjectRepo().Update(&subject) // Update cache
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	LogAudit(c, "UPDATE", "subject", id, input)
	api_response.Success(c, gin.H{"course": subject})
}
