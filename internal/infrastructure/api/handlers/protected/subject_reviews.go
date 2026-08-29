package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	gamificationservice "thanawy-backend/internal/domain/gamification/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func CreateCourseReview(c *gin.Context) {
	userIdVal, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userId, ok := userIdVal.(string)
	if !ok || userId == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	subjectId := c.Param("id")

	var subject models.Subject
	query := db.DB.Select("id")
	query = applyIDOrSlugQuery(query, subjectId)
	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, subjectId, err, "resolving subject for review creation")
		return
	}

	var review models.CourseReview
	if err := c.ShouldBindJSON(&review); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	review.UserID = userId
	review.SubjectID = subject.ID

	if err := SafeCreate(db.DB, &review); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create review")
		return
	}

	// Update subject rating (simplified calculation)
	var avg float64
	db.DB.Model(&models.CourseReview{}).Where(subjectIDQuery, subject.ID).Select("avg(rating)").Scan(&avg)
	db.DB.Model(&models.Subject{}).Where(idQuery, subject.ID).Update("rating", avg)

	// Award 10 XP points for submitting a course review
	xpAmount := 10
	gamificationService := gamificationservice.NewGamificationCommandService()
	_ = gamificationService.AwardXP(gamificationservice.AwardXPCommand{
		UserID:   review.UserID,
		XPType:   "quest",
		XPAmount: xpAmount,
		Source:   "course_review",
		SourceID: review.ID,
	})

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subject.ID)

	api_response.Success(c, gin.H{
		"review":    review,
		"xpAwarded": xpAmount,
	})
}

func GetCourseReviews(c *gin.Context) {
	id := c.Param("id")
	var reviews []models.CourseReview

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var subject models.Subject
	query := db.DB.Select("id").WithContext(c.Request.Context())
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "resolving subject for reviews")
		return
	}

	if err := db.DB.WithContext(c.Request.Context()).
		Preload("User").
		Where("subject_id = ? AND is_visible = ?", subject.ID, true).
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reviews")
		return
	}

	api_response.Success(c, reviews)
}
