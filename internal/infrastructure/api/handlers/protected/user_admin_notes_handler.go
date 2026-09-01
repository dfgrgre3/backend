package protected

import (
	"net/http"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// noteResponse renders a UserAdminNote in the shape the admin frontend expects:
// { id, content, createdBy (display name), createdAt, updatedAt }.
func noteResponse(n models.UserAdminNote) gin.H {
	createdBy := n.CreatedBy
	if n.Creator != nil {
		createdBy = n.Creator.GetName()
	}
	return gin.H{
		"id":        n.ID,
		"content":   n.Content,
		"createdBy": createdBy,
		"createdAt": n.CreatedAt,
		"updatedAt": n.UpdatedAt,
	}
}

// GetUserAdminNotes lists internal admin notes for a user (newest first).
func GetUserAdminNotes(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var notes []models.UserAdminNote
	if err := db.DB.Preload("Creator").Where("user_id = ?", userID).Order("created_at DESC").Find(&notes).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch notes")
		return
	}
	items := make([]gin.H, 0, len(notes))
	for _, n := range notes {
		items = append(items, noteResponse(n))
	}
	api_response.Success(c, items)
}

// CreateUserAdminNote adds a new internal admin note for a user.
func CreateUserAdminNote(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var req struct {
		Content string `json:"content" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "content is required")
		return
	}
	actorID, ok := getAuthenticatedUserID(c)
	if !ok || actorID == "" {
		api_response.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}
	note := models.UserAdminNote{
		UserID:    userID,
		Content:   req.Content,
		CreatedBy: actorID,
	}
	if err := db.DB.Create(&note).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create note")
		return
	}
	db.DB.Preload("Creator").First(&note, "id = ?", note.ID)
	LogAudit(c, "CREATE_USER_NOTE", "user", userID, gin.H{"noteId": note.ID})
	api_response.Success(c, noteResponse(note))
}

// UpdateUserAdminNote edits the content of an existing note.
func UpdateUserAdminNote(c *gin.Context) {
	userID := c.Param("id")
	noteID := c.Param("noteId")
	if userID == "" || noteID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id and note id are required")
		return
	}
	var req struct {
		Content string `json:"content" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "content is required")
		return
	}
	actorID, _ := getAuthenticatedUserID(c)
	var note models.UserAdminNote
	if err := db.DB.Where("id = ? AND user_id = ?", noteID, userID).First(&note).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Note not found")
		return
	}
	updates := map[string]any{
		"content":    req.Content,
		"updated_by": actorID,
	}
	if err := db.DB.Model(&note).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update note")
		return
	}
	db.DB.Preload("Creator").First(&note, "id = ?", note.ID)
	LogAudit(c, "UPDATE_USER_NOTE", "user", userID, gin.H{"noteId": note.ID})
	api_response.Success(c, noteResponse(note))
}

// DeleteUserAdminNote soft-deletes a note.
func DeleteUserAdminNote(c *gin.Context) {
	userID := c.Param("id")
	noteID := c.Param("noteId")
	if userID == "" || noteID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id and note id are required")
		return
	}
	if err := db.DB.Where("id = ? AND user_id = ?", noteID, userID).Delete(&models.UserAdminNote{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete note")
		return
	}
	LogAudit(c, "DELETE_USER_NOTE", "user", userID, gin.H{"noteId": noteID})
	api_response.Success(c, gin.H{"success": true})
}
