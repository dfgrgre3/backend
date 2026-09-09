package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetCourseEnrollmentEligibility answers the permission question without
// mutating enrollment or payment state. Checkout and enrollment remain
// separate commands.
func GetCourseEnrollmentEligibility(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.ReadDB(), c.Param("id")).First(&subject).Error; err != nil {
		handleSubjectError(c, c.Param("id"), err, "checking course enrollment eligibility")
		return
	}

	var enrollment models.Enrollment
	isEnrolled := db.ReadDB().Where("user_id = ? AND subject_id = ?", userID, subject.ID).First(&enrollment).Error == nil
	price, _ := subject.Price.Float64()
	requiresPayment := price > 0 && !hasPaidForSubject(userID, subject.ID)

	api_response.Success(c, gin.H{
		"courseId":        subject.ID,
		"isEnrolled":      isEnrolled,
		"eligible":        isEnrolled || !requiresPayment,
		"requiresPayment": requiresPayment,
		"price":           price,
	})
}
