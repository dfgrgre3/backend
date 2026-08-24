package admin

import (
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
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
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		if s.SubjectID != nil && *s.SubjectID != "" {
			ids = append(ids, *s.SubjectID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var subjects []models.Subject
	db.DB.Where(idInQuery, ids).Find(&subjects)
	for _, s := range subjects {
		summary.SubjectMap[s.ID] = s
	}
}
