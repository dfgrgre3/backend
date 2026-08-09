package protected

import (
	"fmt"
	"net/http"
	"os"
	"thanawy-backend/internal/application/services"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AdminAIGet returns the AI advisor snapshot: at-risk students and subjects.
func AdminAIGet(c *gin.Context) {
	var riskStudents []models.User
	db.DB.Where(statusQuery, models.StatusInactive).Limit(5).Find(&riskStudents)

	var subjects []models.Subject
	db.DB.Limit(10).Find(&subjects)

	riskItems := make([]gin.H, 0, len(riskStudents))
	for _, s := range riskStudents {
		daysSinceUpdate := int(time.Since(s.UpdatedAt).Hours() / 24)
		riskScore := 60 + (daysSinceUpdate / 2)
		if riskScore > 98 {
			riskScore = 98
		}

		riskItems = append(riskItems, gin.H{
			"id":         s.ID,
			"name":       firstNonEmpty(stringOrEmpty(s.Name), stringOrEmpty(s.Username), s.Email),
			"riskScore":  riskScore,
			"gradeLevel": s.GradeLevel,
			"reasons":    []string{"انقطاع عن النشاط", fmt.Sprintf("آخر تواجد منذ %d يوم", daysSinceUpdate)},
		})
	}

	subjectItems := make([]gin.H, 0, len(subjects))
	for _, s := range subjects {
		subjectItems = append(subjectItems, gin.H{
			"id":   s.ID,
			"name": s.Name,
		})
	}

	api_response.Success(c, gin.H{
		"riskStudents": riskItems,
		"subjects":     subjectItems,
		"summary": gin.H{
			"highRiskCount": len(riskItems),
		},
	})
}

// AdminResetCircuitBreaker resets all circuit breakers to closed state
func AdminResetCircuitBreaker(c *gin.Context) {
	svc := services.GetCircuitBreakerService()
	status := svc.GetStatus()

	for name := range status {
		svc.ResetCircuitBreaker(name)
	}

	api_response.Success(c, gin.H{
		"message": "All circuit breakers have been reset",
		"status":  svc.GetStatus(),
	})
}

func AdminAIPost(c *gin.Context) {
	var req struct {
		Action      string `json:"action"`
		Prompt      string `json:"prompt"`
		Title       string `json:"title"`
		ContentType string `json:"contentType"`
		SubjectId   string `json:"subjectId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request")
		return
	}

	switch req.Action {
	case "copilot":
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			api_response.Success(c, gin.H{
				"message": "أنا مستشار المملكة الذكي. هذه ميزة تجريبية وسأكون متاحاً قريباً بشكل كامل.",
			})
			return
		}

		var totalUsers int64
		var totalSubjects int64
		db.DB.Model(&models.User{}).Count(&totalUsers)
		db.DB.Model(&models.Subject{}).Count(&totalSubjects)

		systemPrompt := fmt.Sprintf(`أنت مستشار ذكي لإدارة منصة "ثانوي" التعليمية.
لديك حالياً %d مستخدم و %d مادة دراسية.
ساعد المدير في اتخاذ القرارات وتحليل البيانات وتقديم الاقتراحات.
أجب بالعربية بشكل مختصر ومفيد.`, totalUsers, totalSubjects)

		messages := []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Prompt},
		}

		aiHandler := NewAIHandler()
		reply, _, err := aiHandler.callAIWithRetryCustom(messages, "google/gemini-2.0-flash-001")
		if err != nil {
			api_response.Success(c, gin.H{
				"message": "عذراً، حدث خطأ في الاتصال بالذكاء الاصطناعي. يرجى المحاولة مرة أخرى.",
			})
			return
		}

		api_response.Success(c, gin.H{"message": reply})

	case "generate_content":
		systemPrompt := fmt.Sprintf("أنت مستشار ذكي لإنشاء محتوى من نوع %s. قم بإنشاء المحتوى المطلوب.", req.ContentType)
		messages := []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Prompt},
		}

		aiHandler := NewAIHandler()
		reply, _, err := aiHandler.callAIWithRetryCustom(messages, "google/gemini-2.0-flash-001")
		if err != nil {
			api_response.Error(c, http.StatusInternalServerError, "فشل توليد المحتوى، الرجاء المحاولة مرة أخرى.")
			return
		}
		api_response.Success(c, gin.H{"message": reply})

	default:
		api_response.Error(c, http.StatusBadRequest, "Unknown action")
	}
}

func GetAIRecommendations(c *gin.Context) {
	_, exists := c.Get("userId")
	if !exists {
		api_response.Success(c, gin.H{
			"recommendations": []interface{}{},
			"message":         "Please login to see personalized recommendations",
		})
		return
	}

	api_response.Success(c, gin.H{
		"recommendations": []interface{}{},
		"message":         "AI recommendations not yet implemented",
	})
}

func TrackAIRecommendation(c *gin.Context) {
	var req struct {
		RecommendationID string `json:"recommendationId" binding:"required"`
		Action           string `json:"action"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	api_response.Success(c, gin.H{"success": true})
}
