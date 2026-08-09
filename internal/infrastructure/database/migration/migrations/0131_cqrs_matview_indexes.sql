-- Migration 0131: Optimize CQRS materialized view refresh queries
-- Adds composite indexes required by materialized view queries to eliminate slow sequential scans during view refresh.

-- 1. Index for StudySession filtering by user_id and start_time (used in mv_user_progress_summary & mv_user_weekly_analytics)
CREATE INDEX IF NOT EXISTS idx_studysession_user_starttime 
ON "StudySession" (user_id, start_time);

-- 2. Index for Task filtering by user_id and created_at/updated_at (used in mv_user_weekly_analytics)
CREATE INDEX IF NOT EXISTS idx_task_user_dates 
ON "Task" (user_id, created_at, updated_at);

-- 3. Partial Index for TopicProgress filtering by user_id and non-deleted (used in mv_user_progress_summary)
CREATE INDEX IF NOT EXISTS idx_topicprogress_user_active 
ON "TopicProgress" (user_id) 
WHERE deleted_at IS NULL;

-- 4. Index for ExamResult filtering by user_id (used in mv_user_progress_summary)
CREATE INDEX IF NOT EXISTS idx_examresult_user_passed 
ON "ExamResult" (user_id, passed);
