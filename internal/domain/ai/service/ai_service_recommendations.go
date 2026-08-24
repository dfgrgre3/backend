package aiservice

import (
	"context"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	database "thanawy-backend/internal/infrastructure/database"
)

// GetStudyRecommendations provides personalized study recommendations using AI or fallback
func (s *AIService) GetStudyRecommendations(ctx context.Context, user models.User) ([]map[string]interface{}, error) {
	if !s.enabled {
		// Return rule-based recommendations when AI is not enabled
		return s.getFallbackRecommendations(user), nil
	}

	// Try to get AI-powered recommendations
	prompt := fmt.Sprintf(`بناءً على بيانات الطالب التالية:
- المستوى الدراسي: %s
- نوع التعليم: %s
- الشعبة: %s
- إجمالي النقاط: %d
- المستوى الحالي: %d
- سلسلة الحضور اليومي: %d أيام
- آخر نشاط: %s

اقترح 5-7 دورات دراسية مناسبة لهذا الطالب مع الأسباب.`,
		safeString(user.GradeLevel), safeString(user.EducationType), safeString(user.Section),
		user.TotalXP, user.Level, user.CurrentStreak, user.UpdatedAt.Format("2006-01-02"))

	systemPrompt := "أنت مستشار تعليمي خبير. اقترح دورات دراسية محددة ومفصلة تناسب مستوى الطالب واهتماماته، مع تبرير كل اقتراح."

	aiResponse, err := s.callAI(ctx, systemPrompt, prompt, 0.7, 1200)
	if err != nil {
		// Fallback to rule-based recommendations
		return s.getFallbackRecommendations(user), nil
	}

	// Parse AI response and combine with database courses
	aiBasedRecs := s.parseAIRecommendations(aiResponse)

	// Get some actual courses from database to complement AI suggestions
	dbCourses := s.getDatabaseCourseRecommendations(3)

	// Combine both recommendations
	recommendations := append(aiBasedRecs, dbCourses...)

	return recommendations, nil
}

// parseAIRecommendations parses AI response into structured recommendations
func (s *AIService) parseAIRecommendations(aiResponse string) []map[string]interface{} {
	// For now, return generic recommendations based on AI response
	// In production, this would parse the AI response more intelligently
	reasonText := aiResponse
	if len(aiResponse) > 100 {
		reasonText = aiResponse[:100]
	}

	return []map[string]interface{}{
		{
			"type":      "subject",
			"subjectId": "", // Would be filled by matching with database
			"title":     "دورة مراجعة شاملة",
			"reason":    reasonText,
			"priority":  "high",
		},
	}
}

// getDatabaseCourseRecommendations gets course recommendations from database
func (s *AIService) getDatabaseCourseRecommendations(limit int) []map[string]interface{} {
	var subjects []models.Subject

	query := database.DB.Model(&models.Subject{}).
		Where("is_published = ? AND is_active = ?", true, true).
		Where("status = ?", "PUBLISHED").
		Order("is_featured DESC").
		Order("rating DESC").
		Limit(limit)

	if err := query.Find(&subjects).Error; err != nil {
		return []map[string]interface{}{}
	}

	recommendations := make([]map[string]interface{}, 0, len(subjects))
	for _, subject := range subjects {
		recommendations = append(recommendations, map[string]interface{}{
			"type":      "subject",
			"subjectId": subject.ID,
			"title":     getDisplayName(subject),
			"reason":    "دورة عالية الجودة مناسبة لمستواك",
			"priority":  "medium",
		})
	}

	return recommendations
}

// getFallbackRecommendations provides rule-based recommendations when AI is unavailable
func (s *AIService) getFallbackRecommendations(user models.User) []map[string]interface{} {
	recommendations := []map[string]interface{}{}

	// Get top courses from database as fallback
	var subjects []models.Subject
	database.DB.Model(&models.Subject{}).
		Where("is_published = ? AND is_active = ?", true, true).
		Where("status = ?", "PUBLISHED").
		Order("is_featured DESC").
		Order("rating DESC").
		Limit(5).
		Find(&subjects)

	for _, subject := range subjects {
		recommendations = append(recommendations, map[string]interface{}{
			"type":      "subject",
			"subjectId": subject.ID,
			"title":     getDisplayName(subject),
			"reason":    "دورة مميزة موصى بها",
			"priority":  "high",
		})
	}

	// Add habit-based recommendations
	if user.CurrentStreak == 0 {
		recommendations = append(recommendations, map[string]interface{}{
			"type":     "habit",
			"title":    "ابدأ سلسلة حضور يومي",
			"reason":   "الانتظام في الدراسة مهم للنجاح",
			"priority": "high",
		})
	}

	if user.TotalXP < 100 {
		recommendations = append(recommendations, map[string]interface{}{
			"type":     "practice",
			"title":    "تدريبات أساسية",
			"reason":   "تحتاج لتعزيز نقاطك",
			"priority": "medium",
		})
	}

	return recommendations
}

// getDisplayName returns Arabic name if available, otherwise English
func getDisplayName(subject models.Subject) string {
	if subject.NameAr != nil && *subject.NameAr != "" {
		return *subject.NameAr
	}
	return subject.Name
}
