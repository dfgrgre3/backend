package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetExamResults(c *gin.Context) {
	// SECURITY: always use the authenticated session's own user ID. This
	// previously trusted a client-supplied ?userId= query parameter over the
	// session and was registered on the public (unauthenticated) router,
	// letting anyone read any other user's exam scores and answers just by
	// passing ?userId=<victim-id> — a full auth bypass. There is no
	// teacher/admin "view another user's results" use case wired up
	// elsewhere in the codebase, so the query override is dropped entirely
	// rather than gated behind a permission check.
	val, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userId, ok := val.(string)
	if !ok || userId == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var results []models.ExamResult
	if err := db.ReadDB(c.Request.Context()).
		Preload("Exam.Subject").
		Preload("Exam.Questions").
		Where("user_id = ?", userId).
		Order("taken_at desc").
		Limit(limit).
		Offset(offset).
		Find(&results).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch results")
		return
	}

	api_response.Success(c, results)
}
