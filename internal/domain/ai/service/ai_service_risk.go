package aiservice

import (
	"context"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	"time"
)

// AnalyzeRisk analyzes student risk based on activity
func (s *AIService) AnalyzeRisk(ctx context.Context, user models.User) (map[string]interface{}, error) {
	daysSinceUpdate := int(time.Since(user.UpdatedAt).Hours() / 24)

	riskScore := 60 + (daysSinceUpdate / 2)
	if riskScore > 98 {
		riskScore = 98
	}

	reasons := []string{}
	if daysSinceUpdate > 7 {
		reasons = append(reasons, fmt.Sprintf("انقطاع عن النشاط منذ %d يوم", daysSinceUpdate))
	}

	if user.CurrentStreak == 0 && daysSinceUpdate > 3 {
		reasons = append(reasons, "توقف سلسلة الحضور اليومي")
	}

	if user.TotalStudyTime < 60 && daysSinceUpdate > 14 {
		reasons = append(reasons, "وقت دراسة قليل جداً")
	}

	return map[string]interface{}{
		"riskScore": riskScore,
		"reasons":   reasons,
		"level":     getRiskLevel(riskScore),
	}, nil
}

func getRiskLevel(score int) string {
	if score >= 80 {
		return "high"
	} else if score >= 50 {
		return "medium"
	}
	return "low"
}
