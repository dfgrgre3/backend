package admin

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AdminListExamResults returns exam attempts/results across all users
// (admin-only — the self-scoped GetExamResults deliberately does not allow
// this). Backed by the real "ExamResult" table. Powers both the "exam
// attempts" and "exam results" admin pages, and the per-student "exams" tab
// via the optional userId filter.
func AdminListExamResults(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	search := strings.TrimSpace(c.Query("search"))
	userID := c.Query("userId")
	examID := c.Query("examId")
	passedFilter := c.Query("passed") // "true" | "false" | ""
	offset := (page - 1) * limit

	query := db.DB.Model(&models.ExamResult{})
	if userID != "" {
		query = query.Where("\"ExamResult\".user_id = ?", userID)
	}
	if examID != "" {
		query = query.Where("\"ExamResult\".exam_id = ?", examID)
	}
	if passedFilter == "true" {
		query = query.Where("\"ExamResult\".passed = ?", true)
	} else if passedFilter == "false" {
		query = query.Where("\"ExamResult\".passed = ?", false)
	}
	if search != "" {
		query = query.Joins("LEFT JOIN \"User\" ON \"ExamResult\".user_id = \"User\".id").
			Joins("LEFT JOIN \"Exam\" ON \"ExamResult\".exam_id = \"Exam\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"Exam\".title ILIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var results []models.ExamResult
	if err := query.
		Preload("User").
		Preload("Exam.Subject").
		Order("\"ExamResult\".taken_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&results).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch exam results")
		return
	}

	items := make([]gin.H, 0, len(results))
	for _, r := range results {
		userName := ""
		if r.User.Name != nil {
			userName = *r.User.Name
		}
		items = append(items, gin.H{
			"id":          r.ID,
			"examId":      r.ExamID,
			"examTitle":   r.Exam.Title,
			"subjectName": r.Exam.Subject.Name,
			"userId":      r.UserID,
			"score":       r.Score,
			"maxScore":    r.Exam.MaxScore,
			"passed":      r.Passed,
			"takenAt":     r.TakenAt,
			"user": gin.H{
				"id":    r.User.ID,
				"name":  userName,
				"email": r.User.Email,
			},
		})
	}

	var totalAttempts, passedCount, failedCount int64
	var avgScore float64
	db.DB.Model(&models.ExamResult{}).Count(&totalAttempts)
	db.DB.Model(&models.ExamResult{}).Where("passed = ?", true).Count(&passedCount)
	db.DB.Model(&models.ExamResult{}).Where("passed = ?", false).Count(&failedCount)
	db.DB.Model(&models.ExamResult{}).Select("COALESCE(AVG(score), 0)").Scan(&avgScore)

	api_response.Success(c, gin.H{
		"results": items,
		"summary": gin.H{
			"totalAttempts": totalAttempts,
			"passedCount":   passedCount,
			"failedCount":   failedCount,
			"avgScore":      avgScore,
		},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}
