package queries

import "time"

/*
⚠️ DATABASE SCHEMA STANDARDIZATION NOTICE:
The underlying PostgreSQL Materialized Views currently use a mix of camelCase and snake_case.
While the GORM tags below map exactly to the existing DB columns to prevent runtime errors,
it is HIGHLY RECOMMENDED to run a database migration to standardize all columns to snake_case
for better consistency and maintainability.
*/

// UserProgressSummaryMV maps to the read-only materialized view mv_user_progress_summary.
// It provides a pre-calculated, high-level overview of a learner's lifetime and recent activity.
type UserProgressSummaryMV struct {
	// --- Identifiers ---
	UserID string `gorm:"column:user_id;primaryKey" json:"userId"`

	// --- Gamification & Engagement ---
	TotalXP       int `gorm:"column:totalXP" json:"totalXp"`
	Level         int `gorm:"column:level" json:"level"`
	CurrentStreak int `gorm:"column:currentStreak" json:"currentStreak"`
	LongestStreak int `gorm:"column:longestStreak" json:"longestStreak"`

	// --- Lifetime Activity Metrics ---

	// TotalStudyMinutes represents the cumulative active study session time (excluding paused/video-only time).
	// Maps to DB column 'totalStudyTime' (which is in minutes based on domain context).
	TotalStudyMinutes int `gorm:"column:totalStudyTime" json:"totalStudyMinutes"`

	// TotalWatchTimeSeconds represents the cumulative video playback time across all subjects.
	// Maps to DB column 'total_time_seconds'.
	TotalWatchTimeSeconds int `gorm:"column:total_time_seconds" json:"totalWatchTimeSeconds"`

	TasksCompleted   int `gorm:"column:tasksCompleted" json:"tasksCompleted"`
	LessonsCompleted int `gorm:"column:lessons_completed" json:"lessonsCompleted"`

	// InProgressCount represents the number of lessons/sub-topics currently marked as 'in_progress' (started but not completed).
	InProgressCount int `gorm:"column:in_progress_count" json:"inProgressCount"`

	// ActiveSubjects represents the count of distinct subjects the user has interacted with in the last 30 days.
	ActiveSubjects int `gorm:"column:active_subjects" json:"activeSubjects"`

	// --- Exam Metrics ---
	ExamsPassed      int `gorm:"column:examsPassed" json:"examsPassed"`
	TotalExamsTaken  int `gorm:"column:total_exams_taken" json:"totalExamsTaken"`
	TotalExamsPassed int `gorm:"column:total_exams_passed" json:"totalExamsPassed"`

	// --- Weekly Aggregations (Rolling 7 Days) ---
	WeeklyStudyMinutes int     `gorm:"column:weekly_study_minutes" json:"weeklyStudyMinutes"`
	WeeklySessions     int     `gorm:"column:weekly_sessions" json:"weeklySessions"`
	WeeklyAvgFocus     float64 `gorm:"column:weekly_avg_focus" json:"weeklyAvgFocus"`

	// --- Metadata ---
	ComputedAt time.Time `gorm:"column:computed_at" json:"computedAt"`
}

// TableName specifies the materialized view name for GORM.
func (UserProgressSummaryMV) TableName() string {
	return "mv_user_progress_summary"
}

// Backward compatibility alias for existing services (e.g., ProgressQueryService).
type UserProgressSummaryReadModel = UserProgressSummaryMV

// WeeklyAnalyticsMV maps to the read-only materialized view mv_user_weekly_analytics.
// It provides detailed metrics for the learner's activity over the last 7 days.
type WeeklyAnalyticsMV struct {
	// --- Identifiers ---
	UserID string `gorm:"column:user_id;primaryKey" json:"userId"`

	// --- Study Metrics ---
	TotalStudyMinutes int `gorm:"column:total_study_minutes" json:"totalStudyMinutes"`
	TotalSessions     int `gorm:"column:total_sessions" json:"totalSessions"`
	ActiveDays        int `gorm:"column:active_days" json:"activeDays"`

	// --- Task Metrics ---
	TotalTasks     int     `gorm:"column:total_tasks" json:"totalTasks"`
	CompletedTasks int     `gorm:"column:completed_tasks" json:"completedTasks"`
	CompletionRate float64 `gorm:"column:completion_rate" json:"completionRate"`

	// --- Focus & Gamification ---
	// WeeklyAvgFocus represents the average focus score across all study sessions in the week.
	// Note: Ensure the underlying MV query includes AVG(focus_score) for this column to be populated.
	WeeklyAvgFocus float64 `gorm:"column:weekly_avg_focus" json:"weeklyAvgFocus"`
	WeeklyXPEarned int     `gorm:"column:weekly_xp_earned" json:"weeklyXpEarned"`

	// --- Metadata ---
	ComputedAt time.Time `gorm:"column:computed_at" json:"computedAt"`
}

// TableName specifies the materialized view name for GORM.
func (WeeklyAnalyticsMV) TableName() string {
	return "mv_user_weekly_analytics"
}

// Backward compatibility alias for existing services.
type WeeklyAnalyticsReadModelV2 = WeeklyAnalyticsMV

// UserWatchTimeMV maps to the read-only materialized view mv_user_watch_time.
// It tracks video consumption and progress at the sub-topic level, with aggregated totals for parent topics and subjects.
type UserWatchTimeMV struct {
	// --- Identifiers (Composite Primary Key) ---
	UserID     string `gorm:"column:user_id;primaryKey" json:"userId"`
	SubjectID  string `gorm:"column:subject_id" json:"subjectId"`
	TopicID    string `gorm:"column:topic_id" json:"topicId"`
	SubTopicID string `gorm:"column:sub_topic_id;primaryKey" json:"subTopicId"`

	// --- Display Names ---
	SubjectName   string `gorm:"column:subject_name" json:"subjectName"`
	TopicTitle    string `gorm:"column:topic_title" json:"topicTitle"`
	SubTopicTitle string `gorm:"column:sub_topic_title" json:"subTopicTitle"`
	SubTopicType  string `gorm:"column:sub_topic_type" json:"subTopicType"`

	// --- Status & Progress ---
	Status    string `gorm:"column:status" json:"status"`
	Completed bool   `gorm:"column:completed" json:"completed"`

	// --- Time Metrics (Hierarchical Aggregation) ---

	// SubTopicTimeSpentSeconds is the exact time the user spent watching this specific sub-topic.
	SubTopicTimeSpentSeconds int `gorm:"column:time_spent_seconds" json:"subTopicTimeSpentSeconds"`

	// TopicAggregatedSeconds is the sum of time spent on ALL sub-topics within this parent topic.
	TopicAggregatedSeconds int `gorm:"column:total_topic_seconds" json:"topicAggregatedSeconds"`

	// SubjectAggregatedSeconds is the sum of time spent on ALL topics within this parent subject.
	SubjectAggregatedSeconds int `gorm:"column:total_subject_seconds" json:"subjectAggregatedSeconds"`

	LastWatchedPos int `gorm:"column:last_watched_position" json:"lastWatchedPosition"`

	// --- Metadata ---
	ComputedAt time.Time `gorm:"column:computed_at" json:"computedAt"`
}

// TableName specifies the materialized view name for GORM.
func (UserWatchTimeMV) TableName() string {
	return "mv_user_watch_time"
}
