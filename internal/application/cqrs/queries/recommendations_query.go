package queries

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
)

// RecommendationReadModel is one actionable suggestion for a learner.
type RecommendationReadModel struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Priority      string `json:"priority"`
	Impact        int    `json:"impact"`
	EstimatedTime string `json:"estimatedTime"`
	Category      string `json:"category"`
	Icon          string `json:"icon"`
	ActionURL     string `json:"actionUrl"`
}

// RecommendationsQuerier defines the contract for fetching recommendations.
type RecommendationsQuerier interface {
	GetRecommendations(userID string) ([]RecommendationReadModel, error)
}

// RecommendationsQueryService builds suggestions from the learner's own records.
type RecommendationsQueryService struct{}

// Compile-time check to ensure RecommendationsQueryService implements RecommendationsQuerier.
var _ RecommendationsQuerier = (*RecommendationsQueryService)(nil)

// NewRecommendationsQueryService creates a new instance of RecommendationsQueryService.
func NewRecommendationsQueryService() *RecommendationsQueryService {
	return &RecommendationsQueryService{}
}

func (s *RecommendationsQueryService) readDB() *gorm.DB {
	return db.ReadDB()
}

// --- Constants & Configuration ---

const (
	// Limits & Thresholds
	maxRecommendationsPerCategory = 3
	passingScoreThreshold         = 70.0
	weakSubjectLookbackDays       = 60
	stalledCourseIdleDays         = 7
	targetWeeklyActiveDays        = 5

	// Impact Base Scores
	impactOverdueTaskBase = 90
	impactWeakSubjectBase = 70
	impactConsistencyBase = 70
	impactStalledBase     = 50
	impactMax             = 95 // Absolute cap for any recommendation

	// Priorities
	priorityHigh   = "high"
	priorityMedium = "medium"
	priorityLow    = "low"

	// Recommendation Types & Categories
	typeTask      = "task"
	typeExamPrep  = "exam_prep"
	typeStudyPlan = "study_plan"
	typeTip       = "tip"
)

/*
⚠️ REQUIRED DATABASE INDEXES FOR PERFORMANCE
Ensure the following composite indexes exist to prevent full table scans:
1. CREATE INDEX idx_tasks_user_status_due_at ON tasks(user_id, status, due_at);
2. CREATE INDEX idx_exam_results_user_taken_at ON exam_results(user_id, taken_at);
3. CREATE INDEX idx_subject_enrollments_user_updated_at ON subject_enrollments(user_id, updated_at, progress);
4. CREATE INDEX idx_study_sessions_user_start_time ON study_sessions(user_id, start_time);
*/

// --- Internal Row Structs for SQL Scanning ---

type weakSubjectRow struct {
	SubjectID   string  `gorm:"column:subject_id"`
	SubjectName string  `gorm:"column:subject_name"`
	AvgScore    float64 `gorm:"column:avg_score"`
	Attempts    int     `gorm:"column:attempts"`
}

type stalledCourseRow struct {
	SubjectID   string  `gorm:"column:subject_id"`
	SubjectName string  `gorm:"column:subject_name"`
	Progress    float64 `gorm:"column:progress"`
	DaysIdle    int     `gorm:"column:days_idle"`
}

// --- Main Orchestration Method ---

// GetRecommendations assembles suggestions across four evidence sources concurrently.
func (s *RecommendationsQueryService) GetRecommendations(userID string) ([]RecommendationReadModel, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}

	rdb := s.readDB()
	if rdb == nil {
		return nil, errors.New("database connection is not initialized")
	}

	now := time.Now().UTC()

	type recResult struct {
		items []RecommendationReadModel
		err   error
	}

	overdueCh := make(chan recResult, 1)
	weakCh := make(chan recResult, 1)
	stalledCh := make(chan recResult, 1)
	consistencyCh := make(chan recResult, 1)

	// Launch all 4 queries concurrently
	// Fix: Create separate sessions for each goroutine to avoid concurrent map writes
	go func() {
		defer func() {
			if r := recover(); r != nil {
				overdueCh <- recResult{err: fmt.Errorf("panic in overdueTaskRecommendations: %v", r)}
			}
		}()
		overdueRdb := rdb.Session(&gorm.Session{NewDB: true})
		items, err := s.overdueTaskRecommendations(overdueRdb, userID, now)
		overdueCh <- recResult{items: items, err: err}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				weakCh <- recResult{err: fmt.Errorf("panic in weakSubjectRecommendations: %v", r)}
			}
		}()
		weakRdb := rdb.Session(&gorm.Session{NewDB: true})
		items, err := s.weakSubjectRecommendations(weakRdb, userID, now)
		weakCh <- recResult{items: items, err: err}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stalledCh <- recResult{err: fmt.Errorf("panic in stalledCourseRecommendations: %v", r)}
			}
		}()
		stalledRdb := rdb.Session(&gorm.Session{NewDB: true})
		items, err := s.stalledCourseRecommendations(stalledRdb, userID, now)
		stalledCh <- recResult{items: items, err: err}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				consistencyCh <- recResult{err: fmt.Errorf("panic in consistencyRecommendations: %v", r)}
			}
		}()
		consistencyRdb := rdb.Session(&gorm.Session{NewDB: true})
		items, err := s.consistencyRecommendations(consistencyRdb, userID, now)
		consistencyCh <- recResult{items: items, err: err}
	}()

	var recommendations []RecommendationReadModel
	channels := []<-chan recResult{overdueCh, weakCh, stalledCh, consistencyCh}

	for _, ch := range channels {
		res := <-ch
		if res.err != nil {
			slog.Error("failed to fetch recommendations", "userID", userID, "error", res.err)
			return nil, res.err
		}
		recommendations = append(recommendations, res.items...)
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Impact > recommendations[j].Impact
	})

	return recommendations, nil
}

// --- Evidence Source Methods ---

func (s *RecommendationsQueryService) overdueTaskRecommendations(rdb *gorm.DB, userID string, now time.Time) ([]RecommendationReadModel, error) {
	var tasks []models.Task
	if err := rdb.
		Where("user_id = ? AND status != ? AND due_at IS NOT NULL AND due_at < ?",
			userID, models.TaskCompleted, now).
		Order("due_at ASC").
		Limit(maxRecommendationsPerCategory).
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	out := make([]RecommendationReadModel, 0, len(tasks))
	for _, task := range tasks {
		// Fix #5: Defensive check against corrupted in-memory data
		if task.DueAt == nil {
			continue
		}

		daysLate := int(math.Floor(now.Sub(*task.DueAt).Hours() / 24))

		// Fix #1: Dynamic Impact based on days late
		impact := impactOverdueTaskBase + daysLate
		if impact > impactMax {
			impact = impactMax
		}

		out = append(out, RecommendationReadModel{
			ID:            "task-overdue-" + task.ID,
			Type:          typeTask,
			Title:         "مهمة متأخرة: " + task.Title,
			Description:   fmt.Sprintf("تأخرت %d يوم عن موعد هذه المهمة، أنجزها اليوم لتستعيد جدولك", daysLate),
			Priority:      priorityHigh,
			Impact:        impact,
			EstimatedTime: formatMinutes(task.EstimatedTime),
			Category:      typeTask,
			Icon:          "target",
			ActionURL:     "/tasks",
		})
	}
	return out, nil
}

func (s *RecommendationsQueryService) weakSubjectRecommendations(rdb *gorm.DB, userID string, now time.Time) ([]RecommendationReadModel, error) {
	var rows []weakSubjectRow
	if err := rdb.Table(`"ExamResult" er`).
		Joins(`JOIN "Exam" e ON e.id = er.exam_id AND e.deleted_at IS NULL`).
		Joins(`JOIN "Subject" s ON s.id = e.subject_id AND s.deleted_at IS NULL`).
		Where("er.user_id = ? AND er.deleted_at IS NULL AND er.taken_at >= ?", userID, now.AddDate(0, 0, -weakSubjectLookbackDays)).
		Group("s.id, s.name, s.name_ar").
		Having("AVG(er.score) < ?", passingScoreThreshold).
		Order("AVG(er.score) ASC").
		Limit(maxRecommendationsPerCategory).
		Select(`s.id AS subject_id,
			COALESCE(NULLIF(s.name_ar, ''), s.name) AS subject_name,
			AVG(er.score) AS avg_score,
			COUNT(*) AS attempts`).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]RecommendationReadModel, 0, len(rows))
	for _, row := range rows {
		// Fix #1: Dynamic Impact based on the deficit from the passing score
		// The further below 70 they are, the higher the impact (max 95).
		deficit := passingScoreThreshold - row.AvgScore
		impact := impactWeakSubjectBase + int(deficit*0.8)
		if impact > impactMax {
			impact = impactMax
		}

		out = append(out, RecommendationReadModel{
			ID:          "weak-subject-" + row.SubjectID,
			Type:        typeExamPrep,
			Title:       fmt.Sprintf("قوِّ مستواك في %s", row.SubjectName),
			Description: fmt.Sprintf("متوسط درجاتك %d%% في %d امتحان، راجع الدروس ثم أعد الاختبار", int(math.Round(row.AvgScore)), row.Attempts),
			Priority:    priorityHigh,
			Impact:      impact,
			// Fix #2: Dynamic Estimated Time based on the score deficit
			EstimatedTime: estimateWeakSubjectTime(row.AvgScore),
			Category:      typeExamPrep,
			Icon:          "book",
			ActionURL:     "/courses/" + row.SubjectID,
		})
	}
	return out, nil
}

func (s *RecommendationsQueryService) stalledCourseRecommendations(rdb *gorm.DB, userID string, now time.Time) ([]RecommendationReadModel, error) {
	var rows []stalledCourseRow

	if err := rdb.Table(`"SubjectEnrollment" se`).
		Joins(`JOIN "Subject" s ON s.id = se.subject_id AND s.deleted_at IS NULL`).
		Where("se.user_id = ? AND se.deleted_at IS NULL AND se.progress < 100 AND se.updated_at < ?",
			userID, now.AddDate(0, 0, -stalledCourseIdleDays)).
		Order("se.updated_at ASC").
		Limit(maxRecommendationsPerCategory).
		Select(`s.id AS subject_id,
			COALESCE(NULLIF(s.name_ar, ''), s.name) AS subject_name,
			se.progress AS progress,
			EXTRACT(DAY FROM ? - se.updated_at)::int AS days_idle`, now).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]RecommendationReadModel, 0, len(rows))
	for _, row := range rows {
		out = append(out, RecommendationReadModel{
			ID:          "stalled-course-" + row.SubjectID,
			Type:        typeStudyPlan,
			Title:       fmt.Sprintf("أكمل كورس %s", row.SubjectName),
			Description: fmt.Sprintf("توقفت عند %d%% منذ %d يوم، أكمل الدرس التالي للحفاظ على تقدمك", int(math.Round(row.Progress)), row.DaysIdle),
			Priority:    priorityFromIdleDays(row.DaysIdle),
			Impact:      impactFromIdleDays(row.DaysIdle),
			// Fix #2: Dynamic Estimated Time based on remaining progress
			EstimatedTime: estimateStalledCourseTime(row.Progress),
			Category:      typeStudyPlan,
			Icon:          "book",
			ActionURL:     "/courses/" + row.SubjectID,
		})
	}
	return out, nil
}

func (s *RecommendationsQueryService) consistencyRecommendations(rdb *gorm.DB, userID string, now time.Time) ([]RecommendationReadModel, error) {
	// Fix #4: Use sql.NullInt64 to definitively distinguish between 0 active days and a query error
	var activeDaysResult sql.NullInt64
	if err := rdb.Model(&models.StudySession{}).
		Where("user_id = ? AND start_time >= ?", userID, now.AddDate(0, 0, -7)).
		Select("COUNT(DISTINCT DATE(start_time))").
		Scan(&activeDaysResult).Error; err != nil {
		return nil, err
	}

	activeDays := 0
	if activeDaysResult.Valid {
		activeDays = int(activeDaysResult.Int64)
	}

	if activeDays >= targetWeeklyActiveDays {
		return nil, nil
	}

	// Fix #1: Dynamic Impact based on how inconsistent the user is
	deficit := targetWeeklyActiveDays - activeDays
	impact := impactConsistencyBase + (deficit * 5)
	if impact > impactMax {
		impact = impactMax
	}

	return []RecommendationReadModel{{
		ID:          "consistency-weekly",
		Type:        typeTip,
		Title:       "ارفع انتظامك الأسبوعي",
		Description: fmt.Sprintf("ذاكرت %d يوم فقط هذا الأسبوع، جلسة قصيرة يومياً أفضل من جلسة طويلة متقطعة", activeDays),
		Priority:    priorityMedium,
		Impact:      impact,
		// Fix #2: Dynamic Estimated Time based on the deficit in active days
		EstimatedTime: estimateConsistencyTime(activeDays),
		Category:      typeTip,
		Icon:          "lightbulb",
		ActionURL:     "/schedule",
	}}, nil
}

// --- Smart Estimation & Utility Functions ---

// estimateWeakSubjectTime calculates study time based on the gap from the passing score.
func estimateWeakSubjectTime(avgScore float64) string {
	deficit := passingScoreThreshold - avgScore
	if deficit <= 0 {
		return formatMinutes(15)
	}
	// 2 minutes of study for every 1% deficit
	minutes := 15 + int(deficit*2.0)
	return formatMinutes(minutes)
}

// estimateStalledCourseTime calculates study time based on remaining progress.
func estimateStalledCourseTime(progress float64) string {
	remaining := 100.0 - progress
	if remaining <= 0 {
		return formatMinutes(15)
	}
	// 1.5 minutes for every 1% remaining
	minutes := 15 + int(remaining*1.5)
	return formatMinutes(minutes)
}

// estimateConsistencyTime calculates study time based on missing active days.
func estimateConsistencyTime(activeDays int) string {
	deficit := targetWeeklyActiveDays - activeDays
	if deficit <= 0 {
		return formatMinutes(15)
	}
	// 10 minutes for every missing day
	minutes := 15 + (deficit * 10)
	return formatMinutes(minutes)
}

func formatMinutes(minutes int) string {
	if minutes <= 0 {
		return "غير محدد"
	}
	return fmt.Sprintf("%d دقيقة", minutes)
}

func priorityFromIdleDays(days int) string {
	if days >= 21 {
		return priorityHigh
	}
	if days >= 14 {
		return priorityMedium
	}
	return priorityLow
}

func impactFromIdleDays(days int) int {
	impact := impactStalledBase + days
	if impact > impactMax {
		return impactMax
	}
	return impact
}
