package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// TeachingGetAllReviews returns all reviews across all instructor's courses.
func TeachingGetAllReviews(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var reviews []models.CourseReview
	if err := database.
		Joins(`JOIN "Subject" ON "Subject".id = "CourseReview".subject_id`).
		Where(`"Subject".instructor_id = ? AND "CourseReview".is_visible = ?`, userID, true).
		Order(`"CourseReview".created_at DESC`).
		Preload("User").
		Find(&reviews).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reviews")
		return
	}

	// Fetch subject names for all reviewed courses
	subjectIDs := make([]string, 0, len(reviews))
	for _, r := range reviews {
		subjectIDs = append(subjectIDs, r.SubjectID)
	}
	subjectMap := make(map[string]string)
	if len(subjectIDs) > 0 {
		var subjects []models.Subject
		database.Where("id IN ?", subjectIDs).Find(&subjects)
		for _, s := range subjects {
			subjectMap[s.ID] = s.Name
		}
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
			CourseTitle:   subjectMap[r.SubjectID],
			Rating:        r.Rating,
			Comment:       r.Comment,
			Date:          r.CreatedAt.Format("2006-01-02"),
			Replies:       []ReplyItem{},
		})
	}

	api_response.Success(c, gin.H{"reviews": items})
}
