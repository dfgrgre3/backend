package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AdminGetResources(c *gin.Context) {
	listResources(c, true)
}

func AdminCreateResource(c *gin.Context) {
	var input resourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if input.Title == "" || input.URL == "" || input.SubjectID == "" {
		api_response.Error(c, http.StatusBadRequest, "title, url, and subjectId are required")
		return
	}
	free := true
	if input.Free != nil {
		free = *input.Free
	}
	resourceType := input.Type
	if resourceType == "" {
		resourceType = "link"
	}

	resource := models.Resource{
		Title:       input.Title,
		Description: input.Description,
		URL:         input.URL,
		Type:        resourceType,
		Source:      input.Source,
		Free:        free,
		SubjectID:   input.SubjectID,
	}
	if err := SafeCreate(db.DB, &resource); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create resource")
		return
	}
	LogAudit(c, "CREATE", "resource", resource.ID, resource)
	InvalidateResourcesCache()
	api_response.Created(c, resource)
}

func AdminUpdateResource(c *gin.Context) {
	var input resourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	ids := collectResourceIDs(input)
	if len(ids) == 0 {
		api_response.Error(c, http.StatusBadRequest, "id or ids is required")
		return
	}

	updates := buildResourceUpdates(input)
	if !updates.hasUpdates {
		api_response.Error(c, http.StatusBadRequest, "no updates provided")
		return
	}

	if err := db.DB.Model(&models.Resource{}).Where("id IN ?", ids).
		Updates(&updates.structVal).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update resource")
		return
	}
	LogAudit(c, "UPDATE", "resource", input.ID, updates)
	InvalidateResourcesCache()
	api_response.Success(c, gin.H{"updated": len(ids)})
}

func AdminDeleteResource(c *gin.Context) {
	var input resourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ids := collectResourceIDs(input)
	if len(ids) == 0 {
		api_response.Error(c, http.StatusBadRequest, "id or ids is required")
		return
	}
	if err := db.DB.Delete(&models.Resource{}, "id IN ?", ids).Error; err != nil && err != gorm.ErrRecordNotFound {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete resource")
		return
	}
	LogAudit(c, "DELETE", "resource", input.ID, gin.H{"ids": ids})
	InvalidateResourcesCache()
	api_response.Success(c, gin.H{"deleted": len(ids)})
}
