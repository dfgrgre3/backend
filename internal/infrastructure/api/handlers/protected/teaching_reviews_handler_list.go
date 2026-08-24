package protected

import (
	"errors"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TeachingListReviews returns reviews for a course.
func TeachingListReviews(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Verify ownership
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", courseID, userID).First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	var reviews []models.CourseReview
	if err := database.
		Where(`subject_id = ? AND is_visible = ?`, courseID, true).
		Order(`created_at DESC`).
		Preload("User").
		Find(&reviews).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reviews")
		return
	}

	type ReplyItem struct {
		ID     string `json:"id"`
		Author string `json:"author"`
		Text   string `json:"text"`
		Date   string `json:"date"`
	}

	type ReviewItem struct {
		ID            string      `json:"id"`
		StudentName   string      `json:"studentName"`
		StudentAvatar string      `json:"studentAvatar"`
		CourseTitle   string      `json:"courseTitle"`
		Rating        int         `json:"rating"`
		Comment       string      `json:"comment"`
		Date          string      `json:"date"`
		Replies       []ReplyItem `json:"replies"`
	}

	items := make([]ReviewItem, 0, len(reviews))
	for _, r := range reviews {
		name := ""
		avatar := ""
		if r.User.ID != "" {
			name = stringPtrToString(r.User.Name)
			if name == "" {
				name = r.User.Email
			}
			avatar = stringPtrToString(r.User.Avatar)
		}

		items = append(items, ReviewItem{
			ID:            r.ID,
			StudentName:   name,
			StudentAvatar: avatar,
			CourseTitle:   subject.Name,
			Rating:        r.Rating,
			Comment:       r.Comment,
			Date:          r.CreatedAt.Format("2006-01-02"),
			Replies:       []ReplyItem{}, // Replies not stored yet
		})
	}

	api_response.Success(c, gin.H{"reviews": items})
}
