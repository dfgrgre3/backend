package admin

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

type liveActivitySummary struct {
	SubjectMap map[string]models.Subject
	ExamMap    map[string]models.Exam
}

func buildLiveActivityMaps(examResults []models.ExamResult, studySessions []models.StudySession) liveActivitySummary {
	summary := liveActivitySummary{
		SubjectMap: make(map[string]models.Subject),
		ExamMap:    make(map[string]models.Exam),
	}

	populateExamMaps(examResults, &summary)
	populateSessionSubjectMap(studySessions, &summary)

	return summary
}

func populateExamMaps(results []models.ExamResult, summary *liveActivitySummary) {
	examIDs := make([]string, 0, len(results))
	for _, r := range results {
		examIDs = append(examIDs, r.ExamID)
	}

	if len(examIDs) == 0 {
		return
	}

	var exams []models.Exam
	db.DB.Where(idInQuery, examIDs).Find(&exams)

	subjectIDs := make([]string, 0, len(exams))
	for _, e := range exams {
		summary.ExamMap[e.ID] = e
		subjectIDs = append(subjectIDs, e.SubjectID)
	}

	if len(subjectIDs) > 0 {
		var subjects []models.Subject
		db.DB.Where(idInQuery, subjectIDs).Find(&subjects)
		for _, s := range subjects {
			summary.SubjectMap[s.ID] = s
		}
	}
}

func populateSessionSubjectMap(sessions []models.StudySession, summary *liveActivitySummary) {
	sessionSubjectIDs := make([]string, 0, len(sessions))
	for _, s := range sessions {
		if s.SubjectID != nil && *s.SubjectID != "" {
			sessionSubjectIDs = append(sessionSubjectIDs, *s.SubjectID)
		}
	}

	if len(sessionSubjectIDs) > 0 {
		var subjects []models.Subject
		db.DB.Where(idInQuery, sessionSubjectIDs).Find(&subjects)
		for _, s := range subjects {
			summary.SubjectMap[s.ID] = s
		}
	}
}

type liveUserActivity struct {
	Type    string
	Details interface{}
	Time    time.Time
}

func determineUserLiveActivity(user models.User, examResults []models.ExamResult, studySessions []models.StudySession, summary liveActivitySummary) liveUserActivity {
	// Check Exams first (higher priority activity)
	for _, result := range examResults {
		if result.UserID != user.ID {
			continue
		}

		activity := liveUserActivity{
			Type: "taking_exam",
			Time: result.TakenAt,
		}

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

	return liveUserActivity{
		Type: "online",
		Time: user.UpdatedAt,
	}
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

func GetAdminLive(c *gin.Context) {
	minutes, _ := strconv.Atoi(c.DefaultQuery("minutes", "5"))
	if minutes <= 0 {
		minutes = 5
	}
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)

	var users []models.User
	if err := db.DB.Where(statusQuery, models.StatusActive).Order("updated_at desc").Limit(200).Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch active users")
		return
	}

	var studySessions []models.StudySession
	_ = db.DB.Where("updated_at >= ? OR start_time >= ? OR end_time >= ?", cutoff, cutoff, cutoff).Find(&studySessions).Error

	var examResults []models.ExamResult
	_ = db.DB.Where("taken_at >= ?", cutoff).Find(&examResults).Error

	summary := buildLiveActivityMaps(examResults, studySessions)

	activeUsers := make([]gin.H, 0, len(users))
	stats := struct {
		Studying   int
		TakingExam int
		Online     int
		RoleStats  map[string]int
	}{
		RoleStats: map[string]int{"students": 0, "teachers": 0, "admins": 0},
	}

	for _, user := range users {
		switch user.Role {
		case models.RoleStudent:
			stats.RoleStats["students"]++
		case models.RoleTeacher:
			stats.RoleStats["teachers"]++
		case models.RoleAdmin:
			stats.RoleStats["admins"]++
		}

		activity := determineUserLiveActivity(user, examResults, studySessions, summary)

		switch activity.Type {
		case "taking_exam":
			stats.TakingExam++
		case "studying":
			stats.Studying++
		case "online":
			stats.Online++
		}

		activeUsers = append(activeUsers, gin.H{
			"userId": user.ID,
			"user": gin.H{
				"id":     user.ID,
				"name":   firstNonEmpty(stringOrEmpty(user.Name), stringOrEmpty(user.Username), user.Email),
				"email":  user.Email,
				"role":   user.Role,
				"avatar": user.Avatar,
			},
			"lastAccessed":    activity.Time.Format(time.RFC3339),
			"currentActivity": activity.Type,
			"activityDetails": activity.Details,
			"isActive":        true,
			"sessionId":       nil,
			"ip":              nil,
			"deviceInfo":      nil,
		})
	}

	filteredUsers := filterLiveUsers(activeUsers, c.DefaultQuery("type", "all"))

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"activeUsers": filteredUsers,
		"stats": gin.H{
			"totalActive": len(activeUsers),
			"studying":    stats.Studying,
			"takingExam":  stats.TakingExam,
			"online":      stats.Online,
			"byRole":      stats.RoleStats,
		},
	})
}
