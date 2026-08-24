package admin

import (
	"time"
	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
)

type liveUserActivity struct {
	Type    string
	Details interface{}
	Time    time.Time
}

func determineUserLiveActivity(
	user models.User,
	examResults []models.ExamResult,
	studySessions []models.StudySession,
	summary liveActivitySummary,
) liveUserActivity {
	// Check Exams first (higher priority activity)
	for _, result := range examResults {
		if result.UserID != user.ID {
			continue
		}
		activity := liveUserActivity{Type: "taking_exam", Time: result.TakenAt}
		if exam, found := summary.ExamMap[result.ExamID]; found {
			subject := summary.SubjectMap[exam.SubjectID]
			activity.Details = gin.H{
				"type": "exam",
				"exam": gin.H{
					"id":    exam.ID,
					"title": exam.Title,
					"subject": gin.H{
						"name":   subject.Name,
						"nameAr": stringOrEmpty(subject.NameAr),
					},
				},
				"takenAt": result.TakenAt.Format(time.RFC3339),
				"score":   result.Score,
			}
		}
		return activity
	}

	// Check Study Sessions
	for _, session := range studySessions {
		if session.UserID != user.ID {
			continue
		}
		var subject models.Subject
		if session.SubjectID != nil && *session.SubjectID != "" {
			subject = summary.SubjectMap[*session.SubjectID]
		}
		return liveUserActivity{
			Type: "studying",
			Time: session.UpdatedAt,
			Details: gin.H{
				"type": "study",
				"subject": gin.H{
					"id":     subject.ID,
					"name":   subject.Name,
					"nameAr": stringOrEmpty(subject.NameAr),
				},
				"startTime": session.StartTime.Format(time.RFC3339),
				"duration":  session.DurationMin,
			},
		}
	}

	return liveUserActivity{Type: "online", Time: user.UpdatedAt}
}

func filterLiveUsers(users []gin.H, filterType string) []gin.H {
	if filterType == "all" {
		return users
	}
	filtered := make([]gin.H, 0)
	for _, user := range users {
		if (filterType == "exam" && user["currentActivity"] == "taking_exam") ||
			(filterType == "study" && user["currentActivity"] == "studying") ||
			(filterType == "online" && user["currentActivity"] == "online") {
			filtered = append(filtered, user)
		}
	}
	return filtered
}
