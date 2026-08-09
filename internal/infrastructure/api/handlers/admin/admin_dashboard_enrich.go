package admin

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// This file enriches the raw rows loaded for the admin dashboard with the
// aggregate/related values the admin UI displays (question counts, participant
// counts, subject names, submission counts, host names...).
//
// Without this step the UI would receive rows that simply lack those fields and
// would render permanent zeros/UUIDs in their place.

// subjectNameLookup resolves subject IDs to their display names (Arabic when
// available) in a single query.
func subjectNameLookup(subjectIDs []string) map[string]string {
	names := make(map[string]string, len(subjectIDs))
	unique := dedupeNonEmpty(subjectIDs)
	if len(unique) == 0 {
		return names
	}

	var subjects []models.Subject
	readDashboardDB().Where(idInQuery, unique).Find(&subjects)
	for _, s := range subjects {
		names[s.ID] = firstNonEmpty(stringOrEmpty(s.NameAr), s.Name)
	}
	return names
}

// userNameLookup resolves user IDs to a human readable display name.
func userNameLookup(userIDs []string) map[string]string {
	names := make(map[string]string, len(userIDs))
	unique := dedupeNonEmpty(userIDs)
	if len(unique) == 0 {
		return names
	}

	var users []models.User
	readDashboardDB().Where(idInQuery, unique).Find(&users)
	for _, u := range users {
		names[u.ID] = firstNonEmpty(stringOrEmpty(u.Name), stringOrEmpty(u.Username), u.Email)
	}
	return names
}

// countsByKey runs a grouped COUNT query and returns the result keyed by the
// grouping column.
func countsByKey(model interface{}, groupColumn string, keys []string) map[string]int64 {
	counts := make(map[string]int64, len(keys))
	unique := dedupeNonEmpty(keys)
	if len(unique) == 0 {
		return counts
	}

	type row struct {
		Key   string `gorm:"column:key"`
		Total int64  `gorm:"column:total"`
	}
	var rows []row
	readDashboardDB().Model(model).
		Select(groupColumn+" as key, COUNT(*) as total").
		Where(groupColumn+" IN ?", unique).
		Group(groupColumn).
		Scan(&rows)

	for _, r := range rows {
		counts[r.Key] = r.Total
	}
	return counts
}

// buildDashboardExams turns exam rows into the shape the admin UI renders,
// resolving subject names and the real question/participant counts.
func buildDashboardExams(exams []models.Exam) []gin.H {
	examIDs := make([]string, 0, len(exams))
	subjectIDs := make([]string, 0, len(exams))
	for _, e := range exams {
		examIDs = append(examIDs, e.ID)
		subjectIDs = append(subjectIDs, e.SubjectID)
	}

	questionCounts := countsByKey(&models.Question{}, "exam_id", examIDs)
	participantCounts := countsByKey(&models.ExamResult{}, "exam_id", examIDs)
	subjectNames := subjectNameLookup(subjectIDs)

	items := make([]gin.H, 0, len(exams))
	for _, e := range exams {
		items = append(items, gin.H{
			"id":               e.ID,
			"title":            e.Title,
			"subject":          firstNonEmpty(subjectNames[e.SubjectID], "—"),
			"questionCount":    questionCounts[e.ID],
			"participantCount": participantCounts[e.ID],
			"status":           dashboardExamStatus(e, participantCounts[e.ID]),
			"createdAt":        e.CreatedAt,
			"durationMin":      e.Duration,
		})
	}
	return items
}

// dashboardExamStatus derives a display status from the exam's real state
// instead of labelling every exam as "scheduled".
func dashboardExamStatus(exam models.Exam, participants int64) string {
	if !exam.IsActive {
		if participants > 0 {
			return "completed"
		}
		return "draft"
	}
	if participants > 0 {
		return "active"
	}
	return "scheduled"
}

// buildDashboardAssignments turns task rows into the assignment shape the UI
// renders, resolving subject names and the real submission counts.
func buildDashboardAssignments(tasks []models.Task) []gin.H {
	subjectIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if t.SubjectID != nil {
			subjectIDs = append(subjectIDs, *t.SubjectID)
		}
	}
	subjectNames := subjectNameLookup(subjectIDs)

	// A task is one student's assignment instance. "Submissions" is how many of
	// the sibling tasks sharing the same title are already completed, and
	// "totalStudents" is how many exist in total.
	titles := make([]string, 0, len(tasks))
	for _, t := range tasks {
		titles = append(titles, t.Title)
	}
	submitted, total := assignmentProgressByTitle(titles)

	items := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		subject := "—"
		if t.SubjectID != nil {
			subject = firstNonEmpty(subjectNames[*t.SubjectID], "—")
		}

		item := gin.H{
			"id":            t.ID,
			"title":         t.Title,
			"subject":       subject,
			"submissions":   submitted[t.Title],
			"totalStudents": total[t.Title],
			"status":        string(t.Status),
			"createdAt":     t.CreatedAt,
		}
		if t.DueAt != nil {
			item["dueDate"] = *t.DueAt
		}
		items = append(items, item)
	}
	return items
}

// assignmentProgressByTitle counts, per assignment title, how many task
// instances exist and how many are completed.
func assignmentProgressByTitle(titles []string) (submitted map[string]int64, total map[string]int64) {
	submitted = make(map[string]int64, len(titles))
	total = make(map[string]int64, len(titles))
	unique := dedupeNonEmpty(titles)
	if len(unique) == 0 {
		return submitted, total
	}

	type row struct {
		Title     string `gorm:"column:title"`
		Total     int64  `gorm:"column:total"`
		Completed int64  `gorm:"column:completed"`
	}
	var rows []row
	readDashboardDB().Model(&models.Task{}).
		Select("title, COUNT(*) as total, COUNT(*) FILTER (WHERE status = ?) as completed", models.TaskCompleted).
		Where("title IN ? AND deleted_at IS NULL", unique).
		Group("title").
		Scan(&rows)

	for _, r := range rows {
		total[r.Title] = r.Total
		submitted[r.Title] = r.Completed
	}
	return submitted, total
}

// buildDashboardLiveClasses resolves the host's display name and the real
// number of enrolled viewers for each live session.
func buildDashboardLiveClasses(sessions []models.LiveSession) []gin.H {
	hostEmails := make([]string, 0, len(sessions))
	subjectIDs := make([]string, 0, len(sessions))
	for _, s := range sessions {
		hostEmails = append(hostEmails, s.HostEmail)
		if s.SubjectID != nil {
			subjectIDs = append(subjectIDs, s.SubjectID.String())
		}
	}

	hostNames := userNamesByEmail(hostEmails)
	subjectNames := subjectNameLookup(subjectIDs)
	viewers := liveSessionViewerCounts(subjectIDs)

	items := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		subject := "مباشر"
		viewerCount := int64(0)
		if s.SubjectID != nil {
			key := s.SubjectID.String()
			subject = firstNonEmpty(subjectNames[key], s.Provider)
			viewerCount = viewers[key]
		}

		items = append(items, gin.H{
			"id":          s.ID,
			"title":       s.Title,
			"instructor":  firstNonEmpty(hostNames[s.HostEmail], s.HostEmail),
			"subject":     subject,
			"viewers":     viewerCount,
			"durationMin": s.DurationMin,
			"status":      s.Status,
			"scheduledAt": s.ScheduledAt,
		})
	}
	return items
}

// userNamesByEmail maps email addresses to display names.
func userNamesByEmail(emails []string) map[string]string {
	names := make(map[string]string, len(emails))
	unique := dedupeNonEmpty(emails)
	if len(unique) == 0 {
		return names
	}

	var users []models.User
	readDashboardDB().Where("email IN ?", unique).Find(&users)
	for _, u := range users {
		names[u.Email] = firstNonEmpty(stringOrEmpty(u.Name), stringOrEmpty(u.Username), u.Email)
	}
	return names
}

// liveSessionViewerCounts approximates the live audience by the number of
// students currently enrolled in the session's subject.
func liveSessionViewerCounts(subjectIDs []string) map[string]int64 {
	return countsByKey(&models.Enrollment{}, "subject_id", subjectIDs)
}

// buildDashboardSecurityAlerts resolves the acting user for each security log
// entry so the UI can show a name instead of a bare UUID.
func buildDashboardSecurityAlerts(logs []models.SecurityLog) []gin.H {
	userIDs := make([]string, 0, len(logs))
	for _, l := range logs {
		if l.UserID != nil {
			userIDs = append(userIDs, *l.UserID)
		}
	}
	userNames := userNameLookup(userIDs)

	items := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		source := l.IP
		if l.UserID != nil {
			if name, ok := userNames[*l.UserID]; ok {
				source = name
			}
		}

		items = append(items, gin.H{
			"id":        l.ID,
			"eventType": l.EventType,
			"ip":        l.IP,
			"source":    source,
			"location":  stringOrEmpty(l.Location),
			"metadata":  stringOrEmpty(l.Metadata),
			"createdAt": l.CreatedAt,
		})
	}
	return items
}

// buildDashboardAnnouncements resolves the author name for each announcement.
func buildDashboardAnnouncements(notifications []models.Notification) []gin.H {
	userIDs := make([]string, 0, len(notifications))
	for _, n := range notifications {
		userIDs = append(userIDs, n.UserID)
	}
	userNames := userNameLookup(userIDs)

	items := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, gin.H{
			"id":        n.ID,
			"title":     n.Title,
			"message":   n.Message,
			"type":      n.Type,
			"priority":  n.Priority,
			"author":    userNames[n.UserID],
			"createdAt": n.CreatedAt,
		})
	}
	return items
}

// dedupeNonEmpty removes blank values and duplicates while preserving order.
func dedupeNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" || v == uuid.Nil.String() {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
