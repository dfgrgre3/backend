package queries

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"gorm.io/gorm"

	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"
)

// ProgressQuerier defines the contract for fetching progress data.
type ProgressQuerier interface {
	GetSummary(userID string) (*ProgressSummaryReadModel, error)
	GetWeeklyAnalytics(userID string) (*WeeklyAnalyticsReadModel, error)
}

// ProgressQueryService provides progress and analytics data with multi-tier caching.
type ProgressQueryService struct {
	l1SummaryCache *sync.Map
	l1WeeklyCache  *sync.Map
}

// Compile-time check to ensure ProgressQueryService implements ProgressQuerier.
var _ ProgressQuerier = (*ProgressQueryService)(nil)

// Cache TTLs and Timeouts
const (
	l1CacheTTL        = 15 * time.Second
	redisCacheTTL     = 3 * time.Minute
	redisReadTimeout  = 500 * time.Millisecond // Fix #6: Increased from 200ms to prevent false misses
	redisWriteTimeout = 2 * time.Second
	l1CleanupInterval = 1 * time.Minute
	l1CleanupSample   = 100 // Fix #5: Max entries to scan per tick to save CPU

	targetWeeklyMinutes = 210
	whereUserID         = "user_id = ?"
)

// l1Entry represents a generic cached item with an expiration time.
type l1Entry struct {
	data      any
	expiresAt time.Time
}

// IsExpired checks if the cache entry has passed its TTL.
func (e *l1Entry) IsExpired(now time.Time) bool {
	return now.After(e.expiresAt)
}

// ProgressSummaryReadModel represents a high-level summary of a learner's progress.
type ProgressSummaryReadModel struct {
	TotalMinutes   int     `json:"totalMinutes"`
	AverageFocus   float64 `json:"averageFocus"`
	TasksCompleted int64   `json:"tasksCompleted"`
	StreakDays     int     `json:"streakDays"`
}

// WeeklyAnalyticsReadModel represents weekly study analytics.
type WeeklyAnalyticsReadModel struct {
	ProgressRate   int             `json:"progressRate"`
	SkillsAcquired int             `json:"skillsAcquired"`
	StudyHours     int             `json:"studyHours"`
	DailyProgress  []DailyProgress `json:"dailyProgress"`
	Timestamp      time.Time       `json:"timestamp"`
}

// DailyProgress represents study progress for a single day.
type DailyProgress struct {
	Day      string `json:"day"`
	Progress int    `json:"progress"`
}

var cleanerOnce sync.Once

// NewProgressQueryService creates a new instance with isolated caches for testing. (Fix #2)
func NewProgressQueryService() *ProgressQueryService {
	s := &ProgressQueryService{
		l1SummaryCache: &sync.Map{},
		l1WeeklyCache:  &sync.Map{},
	}

	// Start the background cleaner only once, even if the service is instantiated multiple times.
	cleanerOnce.Do(func() {
		go s.startCacheCleaner()
	})

	return s
}

// startCacheCleaner runs in the background to evict expired L1 cache entries.
func (s *ProgressQueryService) startCacheCleaner() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in L1 cache cleanup goroutine", "error", r)
		}
	}()

	ticker := time.NewTicker(l1CleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.cleanupL1Cache()
	}
}

// cleanupL1Cache uses random sampling to prevent CPU spikes on large caches. (Fix #5)
func (s *ProgressQueryService) cleanupL1Cache() {
	now := time.Now()
	s.sampleAndClean(s.l1SummaryCache, now)
	s.sampleAndClean(s.l1WeeklyCache, now)
}

func (s *ProgressQueryService) sampleAndClean(m *sync.Map, now time.Time) {
	count := 0
	m.Range(func(key, value any) bool {
		if count >= l1CleanupSample {
			return false // Stop scanning to save CPU
		}
		count++
		if entry, ok := value.(*l1Entry); ok {
			if entry.IsExpired(now) {
				m.Delete(key)
			}
		}
		return true
	})
}

func (s *ProgressQueryService) readDBOrFallback() *gorm.DB {
	return db.ReadDB()
}

// GetSummary returns the learner's progress summary, utilizing L1 and L2 caches.
func (s *ProgressQueryService) GetSummary(userID string) (*ProgressSummaryReadModel, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}

	// Try L1 cache first (Unified logic)
	if val, ok := s.getFromL1(s.l1SummaryCache, userID); ok {
		return val.(*ProgressSummaryReadModel), nil
	}

	cacheKey := fmt.Sprintf("user_summary:%s", userID)

	// Try Redis cache next (Unified logic)
	var cachedSummary ProgressSummaryReadModel
	if s.getFromRedis(cacheKey, &cachedSummary) {
		s.warmL1(s.l1SummaryCache, userID, &cachedSummary)
		return &cachedSummary, nil
	}

	rdb := s.readDBOrFallback()
	if rdb == nil {
		return s.getSummaryFallback(userID)
	}

	var summary *ProgressSummaryReadModel
	var err error

	// Read from materialized view
	var mv UserProgressSummaryReadModel
	if err = rdb.Where(whereUserID, userID).Take(&mv).Error; err != nil {
		summary, err = s.getSummaryFallback(userID)
	} else {
		summary = &ProgressSummaryReadModel{
			TotalMinutes:   mv.WeeklyStudyMinutes,
			AverageFocus:   mv.WeeklyAvgFocus,
			TasksCompleted: int64(mv.TasksCompleted),
			StreakDays:     mv.CurrentStreak,
		}
	}

	if err == nil && summary != nil {
		s.warmCache(s.l1SummaryCache, userID, cacheKey, summary)
	}

	return summary, err
}

// GetWeeklyAnalytics returns the learner's weekly analytics, utilizing multi-tier caching.
func (s *ProgressQueryService) GetWeeklyAnalytics(userID string) (*WeeklyAnalyticsReadModel, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}

	if val, ok := s.getFromL1(s.l1WeeklyCache, userID); ok {
		return val.(*WeeklyAnalyticsReadModel), nil
	}

	cacheKey := fmt.Sprintf("weekly_analytics:%s", userID)

	var cachedAnalytics WeeklyAnalyticsReadModel
	if s.getFromRedis(cacheKey, &cachedAnalytics) {
		s.warmL1(s.l1WeeklyCache, userID, &cachedAnalytics)
		return &cachedAnalytics, nil
	}

	rdb := s.readDBOrFallback()
	if rdb == nil {
		return s.getWeeklyAnalyticsFallback(userID)
	}

	var summary *WeeklyAnalyticsReadModel
	var err error

	var mv WeeklyAnalyticsReadModelV2
	if err = rdb.Where(whereUserID, userID).Take(&mv).Error; err != nil {
		summary, err = s.getWeeklyAnalyticsFallback(userID)
	} else {
		summary = weeklyAnalyticsFromView(mv)
	}

	if err == nil && summary != nil {
		s.warmCache(s.l1WeeklyCache, userID, cacheKey, summary)
	}

	return summary, err
}

// --- Unified Caching Helpers (Fix #3: Eliminated 80% code duplication) ---

func (s *ProgressQueryService) getFromL1(m *sync.Map, userID string) (any, bool) {
	if val, ok := m.Load(userID); ok {
		entry := val.(*l1Entry)
		if !entry.IsExpired(time.Now()) {
			return entry.data, true
		}
		m.Delete(userID)
	}
	return nil, false
}

func (s *ProgressQueryService) getFromRedis(cacheKey string, target any) bool {
	if cache.Redis == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisReadTimeout)
	defer cancel()

	cachedVal, err := cache.Redis.Get(ctx, cacheKey).Result()
	if err != nil {
		return false
	}
	return json.Unmarshal([]byte(cachedVal), target) == nil
}

func (s *ProgressQueryService) warmL1(m *sync.Map, userID string, data any) {
	// Fix #9: LoadOrStore prevents race conditions during cache stampedes
	entry := &l1Entry{data: data, expiresAt: time.Now().Add(l1CacheTTL)}
	m.LoadOrStore(userID, entry)
}

func (s *ProgressQueryService) warmCache(m *sync.Map, userID, cacheKey string, data any) {
	s.warmL1(m, userID, data)

	if cache.Redis != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in cache warming", "error", r)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), redisWriteTimeout)
			defer cancel()
			if cacheBytes, err := json.Marshal(data); err == nil {
				// Fix #7: Log Redis write failures instead of failing silently
				if err := cache.Redis.Set(ctx, cacheKey, cacheBytes, redisCacheTTL).Err(); err != nil {
					slog.Error("failed to write to Redis cache", "key", cacheKey, "error", err)
				}
			}
		}()
	}
}

// --- Fallback Logic ---

func (s *ProgressQueryService) getSummaryFallback(userID string) (*ProgressSummaryReadModel, error) {
	rdb := s.readDBOrFallback()
	if rdb == nil {
		return &ProgressSummaryReadModel{}, errors.New("database connection is not initialized")
	}

	summary := &ProgressSummaryReadModel{}

	type studyStats struct {
		TotalMinutes int
		AvgFocus     float64
	}
	var stats studyStats
	if err := rdb.Model(&models.StudySession{}).
		Where(whereUserID, userID).
		Select("COALESCE(SUM(duration_min), 0) as total_minutes, COALESCE(AVG(focus_score), 0) as avg_focus").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	summary.TotalMinutes = stats.TotalMinutes
	summary.AverageFocus = stats.AvgFocus

	if err := rdb.Model(&models.Task{}).
		Where("user_id = ? AND status = ?", userID, "COMPLETED").
		Count(&summary.TasksCompleted).Error; err != nil {
		slog.Error("failed to count completed tasks in fallback", "userID", userID, "error", err)
	}

	// Fix #4: Handle streak calculation errors explicitly
	streak, err := s.calculateStreakDays(userID)
	if err != nil {
		slog.Error("failed to calculate streak days", "userID", userID, "error", err)
	}
	summary.StreakDays = streak

	return summary, nil
}

// calculateStreakDays computes the current consecutive days of study.
func (s *ProgressQueryService) calculateStreakDays(userID string) (int, error) { // Fix #4: Returns error
	rdb := s.readDBOrFallback()
	if rdb == nil {
		return 0, errors.New("database connection is not initialized")
	}

	type dayResult struct {
		Day string
	}
	var days []dayResult

	oneYearAgo := time.Now().UTC().AddDate(-1, 0, 0)

	if err := rdb.Model(&models.StudySession{}).
		Select("DISTINCT DATE(start_time) as day").
		Where("user_id = ? AND start_time >= ?", userID, oneYearAgo).
		Order("day DESC").
		Scan(&days).Error; err != nil {
		return 0, err
	}

	if len(days) == 0 {
		return 0, nil
	}

	streak := 0
	currentDate := time.Now().UTC()
	daySet := make(map[string]bool, len(days))
	for _, d := range days {
		daySet[d.Day] = true
	}

	for {
		dayStr := currentDate.Format("2006-01-02")
		if daySet[dayStr] {
			streak++
			currentDate = currentDate.AddDate(0, 0, -1)
		} else {
			break
		}
	}
	return streak, nil
}

func (s *ProgressQueryService) getWeeklyAnalyticsFallback(userID string) (*WeeklyAnalyticsReadModel, error) {
	rdb := s.readDBOrFallback()
	if rdb == nil {
		return &WeeklyAnalyticsReadModel{Timestamp: time.Now().UTC()}, errors.New("database connection is not initialized")
	}

	sevenDaysAgo := time.Now().UTC().AddDate(0, 0, -7)
	var sessions []models.StudySession
	if err := rdb.Where("user_id = ? AND start_time >= ?", userID, sevenDaysAgo).
		Order("start_time asc").Find(&sessions).Error; err != nil {
		return nil, err
	}

	dailyProgress := make(map[string]int)
	totalStudyMinutes := 0
	for _, session := range sessions {
		day := session.StartTime.Format("Mon")
		dailyProgress[day] += session.DurationMin
		totalStudyMinutes += session.DurationMin
	}

	var dailyProgressArr []DailyProgress
	// Fix #8: Start week from Sunday to align with standard Middle Eastern/Global formats
	days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	for _, day := range days {
		dailyProgressArr = append(dailyProgressArr, DailyProgress{Day: day, Progress: dailyProgress[day]})
	}

	progressRate := weeklyProgressRate(totalStudyMinutes)

	var skillsAcquired int64
	if err := rdb.Model(&models.Task{}).
		Where("user_id = ? AND status = ?", userID, "COMPLETED").
		Count(&skillsAcquired).Error; err != nil {
		slog.Error("failed to count completed tasks for weekly analytics", "userID", userID, "error", err)
	}

	return &WeeklyAnalyticsReadModel{
		ProgressRate:   progressRate,
		SkillsAcquired: int(skillsAcquired),
		StudyHours:     totalStudyMinutes / 60,
		DailyProgress:  dailyProgressArr,
		Timestamp:      time.Now().UTC(),
	}, nil
}

// --- Analytics Helpers ---

func weeklyAnalyticsFromView(mv WeeklyAnalyticsReadModelV2) *WeeklyAnalyticsReadModel {
	return &WeeklyAnalyticsReadModel{
		ProgressRate:   weeklyProgressRate(mv.TotalStudyMinutes),
		SkillsAcquired: mv.CompletedTasks,
		StudyHours:     mv.TotalStudyMinutes / 60,
		DailyProgress:  nil, // MV doesn't store daily breakdowns
		Timestamp:      mv.ComputedAt,
	}
}

// weeklyProgressRate calculates progress without capping at 100% to show overachievement. (Fix #10)
func weeklyProgressRate(totalStudyMinutes int) int {
	if totalStudyMinutes <= 0 {
		return 0
	}
	return int(math.Round(float64(totalStudyMinutes) / float64(targetWeeklyMinutes) * 100))
}
