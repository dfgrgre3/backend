package queries

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"gorm.io/gorm"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
)

// TipReadModel is one piece of guidance rendered as a card in the UI.
type TipReadModel struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Href        string `json:"href"`
	Action      string `json:"action"`
	// ColorKey is a semantic identifier (e.g., "warning", "info").
	// The Frontend is responsible for mapping this to actual CSS/Tailwind classes. (Fix #3)
	ColorKey string `json:"colorKey"`
}

// TipsQuerier defines the contract for fetching study tips.
type TipsQuerier interface {
	GetTips(userID string) ([]TipReadModel, error)
}

// TipsQueryService derives study tips from the learner's measured habits.
type TipsQueryService struct{}

var _ TipsQuerier = (*TipsQueryService)(nil)

func NewTipsQueryService() *TipsQueryService {
	return &TipsQueryService{}
}

func (s *TipsQueryService) readDB() *gorm.DB {
	return db.ReadDB()
}

type habitSnapshot struct {
	AvgFocus       float64
	AvgSessionMin  float64
	ActiveDays     int
	PendingTasks   int64
	ScheduledTasks int64
}

const (
	// Time & Thresholds
	habitLookbackDays          = 14
	tipsLowActiveDaysThreshold = 4 // Fix #6: Less than 4 days out of 14 (<30% consistency) is genuinely low.
	longSessionThresholdMin    = 60.0
	lowFocusThresholdPct       = 60.0

	// Semantic Color Keys (Frontend maps these to CSS)
	ColorWarning = "warning"
	ColorInfo    = "info"
)

/*
⚠️ REQUIRED DATABASE INDEXES FOR PERFORMANCE
1. CREATE INDEX idx_study_sessions_user_start_time ON study_sessions(user_id, start_time);
2. CREATE INDEX idx_tasks_user_status_due_at ON tasks(user_id, status, due_at);
*/

// GetTips returns ONLY actionable tips. If a habit is excellent, no tip is shown for it. (Fix #2)
func (s *TipsQueryService) GetTips(userID string) ([]TipReadModel, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}

	rdb := s.readDB()
	if rdb == nil {
		return nil, errors.New("database connection is not initialized")
	}

	snapshot, err := s.collectHabits(rdb, userID)
	if err != nil {
		slog.Error("failed to collect habits for tips", "userID", userID, "error", err)
		return nil, err
	}

	// Collect tips, filtering out nils (excellent performance)
	var tips []TipReadModel

	if t := s.focusTip(snapshot); t != nil {
		tips = append(tips, *t)
	}
	if t := s.planningTip(snapshot); t != nil {
		tips = append(tips, *t)
	}
	if t := s.analysisTip(snapshot); t != nil {
		tips = append(tips, *t)
	}

	return tips, nil
}

func (s *TipsQueryService) collectHabits(rdb *gorm.DB, userID string) (habitSnapshot, error) {
	var snapshot habitSnapshot
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -habitLookbackDays)

	var sessionAgg struct {
		AvgFocus      float64 `gorm:"column:avg_focus"`
		AvgSessionMin float64 `gorm:"column:avg_session_min"`
		ActiveDays    int     `gorm:"column:active_days"`
	}

	var taskAgg struct {
		PendingTasks   int64 `gorm:"column:pending_tasks"`
		ScheduledTasks int64 `gorm:"column:scheduled_tasks"`
	}

	var wg sync.WaitGroup
	var errSession, errTask error

	wg.Add(2)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errSession = fmt.Errorf("panic in session aggregation: %v", r)
			}
			wg.Done()
		}()
		// Fix: Use separate session to avoid concurrent map writes
		sessionRdb := rdb.Session(&gorm.Session{NewDB: true})
		errSession = sessionRdb.Model(&models.StudySession{}).
			Where("user_id = ? AND start_time >= ?", userID, since).
			Select(`
				COALESCE(AVG(NULLIF(focus_score, 0)), 0) AS avg_focus,
				COALESCE(AVG(NULLIF(duration_min, 0)), 0) AS avg_session_min,
				COUNT(DISTINCT DATE(start_time)) AS active_days
			`).
			Scan(&sessionAgg).Error
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errTask = fmt.Errorf("panic in task aggregation: %v", r)
			}
			wg.Done()
		}()
		// Fix: Use separate session to avoid concurrent map writes
		taskRdb := rdb.Session(&gorm.Session{NewDB: true})
		errTask = taskRdb.Model(&models.Task{}).
			Where("user_id = ? AND status != ?", userID, models.TaskCompleted).
			Select(`
				COUNT(*) AS pending_tasks,
				COUNT(CASE WHEN due_at IS NOT NULL THEN 1 END) AS scheduled_tasks
			`).
			Scan(&taskAgg).Error
	}()

	wg.Wait()

	if errSession != nil {
		return snapshot, errSession
	}
	if errTask != nil {
		return snapshot, errTask
	}

	snapshot.AvgFocus = sessionAgg.AvgFocus
	snapshot.AvgSessionMin = sessionAgg.AvgSessionMin
	snapshot.ActiveDays = sessionAgg.ActiveDays
	snapshot.PendingTasks = taskAgg.PendingTasks
	snapshot.ScheduledTasks = taskAgg.ScheduledTasks

	return snapshot, nil
}

// focusTip returns nil if the user's focus and session lengths are optimal.
func (s *TipsQueryService) focusTip(snapshot habitSnapshot) *TipReadModel {
	tip := &TipReadModel{
		ID:       "focus",
		Icon:     "focus",
		Href:     "/time",
		Action:   "بدء المؤقت",
		ColorKey: ColorWarning,
	}

	switch {
	case snapshot.AvgSessionMin == 0:
		tip.Title = "ابدأ أول جلسة مذاكرة"
		tip.Description = "لم تسجل أي جلسة مذاكرة بعد. ابدأ بجلسة 25 دقيقة لقياس مستوى تركيزك."

	case snapshot.AvgFocus > 0 && snapshot.AvgFocus < lowFocusThresholdPct:
		tip.Title = "ارفع درجة تركيزك"
		tip.Description = fmt.Sprintf("متوسط تركيزك %d%%. أغلق الإشعارات وذاكر في مكان هادئ لرفع هذه النسبة.",
			int(math.Round(snapshot.AvgFocus)))

	case snapshot.AvgSessionMin > longSessionThresholdMin:
		// Fix #5: Handle edge case where session is long but focus tracking is disabled (0)
		if snapshot.AvgFocus == 0 {
			tip.Title = "قسّم جلساتك وفعّل قياس التركيز"
			tip.Description = fmt.Sprintf("متوسط جلستك %d دقيقة لكنك لا تقيس تركيزك. قسّمها إلى 25 دقيقة وفعّل أداة التركيز لضمان الاستفادة.",
				int(math.Round(snapshot.AvgSessionMin)))
		} else {
			tip.Title = "قسّم جلساتك الطويلة"
			tip.Description = fmt.Sprintf("متوسط جلستك %d دقيقة، وهو أطول من طاقة التركيز المعتادة. قسّمها إلى فترات 25 دقيقة مع راحة 5 دقائق.",
				int(math.Round(snapshot.AvgSessionMin)))
		}

	default:
		// Performance is excellent (normal sessions, good focus). Hide the tip.
		return nil
	}

	return tip
}

// planningTip returns nil if all tasks are properly scheduled.
func (s *TipsQueryService) planningTip(snapshot habitSnapshot) *TipReadModel {
	tip := &TipReadModel{
		ID:       "planning",
		Icon:     "planning",
		Href:     "/schedule",
		Action:   "تجهيز الخطة",
		ColorKey: ColorInfo,
	}

	// Fix #1: Protect against negative unscheduled counts due to data anomalies
	unscheduled := snapshot.PendingTasks - snapshot.ScheduledTasks
	if unscheduled < 0 {
		unscheduled = 0
	}

	switch {
	case snapshot.PendingTasks == 0:
		tip.Title = "أضف مهامك القادمة"
		tip.Description = "لا توجد مهام معلقة حالياً. أضف مهام الأسبوع القادم لتحافظ على تقدمك."

	case unscheduled > 0:
		tip.Title = "حدد مواعيد مهامك"
		desc := fmt.Sprintf("لديك %d مهمة بدون موعد نهائي من أصل %d مهمة معلقة. تحديد المواعيد يرفع نسبة الإنجاز.",
			unscheduled, snapshot.PendingTasks)

		// Fix #4: Context-aware planning based on focus levels
		if snapshot.AvgFocus > 0 && snapshot.AvgFocus < lowFocusThresholdPct {
			desc += " ونظراً لانخفاض تركيزك مؤخراً، خطط لمهام قصيرة (15 دقيقة) لتجنب التشتت."
		}
		tip.Description = desc

	default:
		// All tasks are scheduled. Excellent planning. Hide the tip.
		return nil
	}

	return tip
}

// analysisTip returns nil if the user's consistency is strong.
func (s *TipsQueryService) analysisTip(snapshot habitSnapshot) *TipReadModel {
	tip := &TipReadModel{
		ID:       "analysis",
		Icon:     "analysis",
		Href:     "/settings/progress",
		Action:   "تحليل الأداء",
		ColorKey: ColorWarning,
	}

	switch {
	case snapshot.ActiveDays == 0:
		tip.Title = "ابدأ بتسجيل نشاطك"
		tip.Description = "لا توجد أيام نشطة خلال أسبوعين. سجّل جلساتك ليتمكن النظام من تحليل أدائك."

	case snapshot.ActiveDays < tipsLowActiveDaysThreshold:
		tip.Title = "ارفع عدد أيامك النشطة"
		tip.Description = fmt.Sprintf("ذاكرت %d يوم فقط خلال آخر 14 يوم. استهدف %d أيام على الأقل لتثبيت المعلومات.",
			snapshot.ActiveDays, tipsLowActiveDaysThreshold+3) // Target slightly higher than threshold

	default:
		// Consistency is good. Hide the tip to reduce UI noise.
		return nil
	}

	return tip
}
