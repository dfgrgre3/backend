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
	"gorm.io/gorm"
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

// @Summary Get course reviews
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/courses/{id}/reviews [get]
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
		Preload("Comments", func(q *gorm.DB) *gorm.DB { return q.Order("created_at ASC") }).
		Preload("Comments.User").
		Where("subject_id = ? AND is_visible = ?", subject.ID, true).
		Limit(limit).
		Offset(offset).
		Find(&reviews).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reviews")
		return
	}

	api_response.Success(c, reviews)
}

func CreateCourseReviewComment(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}
	var review models.CourseReview
	if err := db.DB.WithContext(c.Request.Context()).Where(idQuery, c.Param("reviewId")).First(&review).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Review not found")
		return
	}
	if !isEnrolledInSubject(userID, review.SubjectID) && !isReviewManagerOrInstructor(c, userID, review.SubjectID) {
		api_response.Error(c, http.StatusForbidden, "Course enrollment is required")
		return
	}
	var body struct {
		Comment string `json:"comment" binding:"required,min=1,max=2000"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}
	comment := models.StudentReviewComment{ReviewID: review.ID, UserID: userID, Comment: body.Comment}
	if err := SafeCreate(db.DB, &comment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create review comment")
		return
	}
	api_response.Created(c, gin.H{"comment": comment})
}

func DeleteCourseReviewComment(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}
	var comment models.StudentReviewComment
	if err := db.DB.Where(idQuery, c.Param("commentId")).First(&comment).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Review comment not found")
		return
	}
	var review models.CourseReview
	if err := db.DB.Select("subject_id").Where(idQuery, comment.ReviewID).First(&review).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Review not found")
		return
	}
	if comment.UserID != userID && !isReviewManagerOrInstructor(c, userID, review.SubjectID) {
		api_response.Error(c, http.StatusForbidden, "You are not allowed to delete this comment")
		return
	}
	if err := db.DB.Delete(&comment).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete review comment")
		return
	}
	api_response.Success(c, gin.H{"deleted": true})
}

func isReviewManagerOrInstructor(c *gin.Context, userID, subjectID string) bool {
	if isAdminRole(c) || c.GetString("role") == "MODERATOR" {
		return true
	}
	var subject models.Subject
	return db.DB.Select("instructor_id").Where(idQuery, subjectID).First(&subject).Error == nil && subject.InstructorId != nil && *subject.InstructorId == userID
}
