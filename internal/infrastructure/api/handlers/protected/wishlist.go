package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetWishlist returns the authenticated user's saved courses.
func GetWishlist(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var items []models.Wishlist
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", userId).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch wishlist")
		return
	}

	api_response.Success(c, gin.H{"items": items})
}

// AddToWishlist saves a course to the authenticated user's wishlist.
// Duplicate adds are a no-op thanks to the unique (user_id, subject_id) index.
func AddToWishlist(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	courseId := c.Param("id")
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "resolving subject for wishlist")
		return
	}

	if isAlreadyEnrolled(userId, subject.ID) {
		api_response.Error(c, http.StatusConflict, "أنت مسجّل بالفعل في هذه الدورة")
		return
	}

	item := models.Wishlist{UserID: userId, SubjectID: subject.ID}
	if err := db.DB.WithContext(c.Request.Context()).
		Where("user_id = ? AND subject_id = ?", userId, subject.ID).
		FirstOrCreate(&item).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add to wishlist")
		return
	}

	api_response.Success(c, gin.H{"item": item})
}

// RemoveFromWishlist removes a course from the authenticated user's wishlist.
func RemoveFromWishlist(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	courseId := c.Param("id")
	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "resolving subject for wishlist removal")
		return
	}

	if err := db.DB.WithContext(c.Request.Context()).
		Where("user_id = ? AND subject_id = ?", userId, subject.ID).
		Delete(&models.Wishlist{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove from wishlist")
		return
	}

	api_response.Success(c, gin.H{"removed": true})
}
