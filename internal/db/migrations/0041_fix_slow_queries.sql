-- ============================================================
-- Migration 0041: Fix remaining slow queries from performance logs
-- ============================================================
-- المشاكل:
-- 1. GET /api/users/billing-summary: 2.09s (Payment query بدون فهرس مناسب)
-- 2. GET /api/gamification/leaderboard: 1.0s (Full table scan)
-- 3. GET /api/gamification/achievements: 806ms (بدون فهرس)
-- 4. GET /api/exams: 561ms (بدون فهرس)
-- 5. GET /api/study-sessions: 569-811ms (تحسين caching)
-- 6. GET /api/recommendations: 540ms (تحسين caching)
-- 7. GET /api/notifications + /api/activities/recent polling كل 1 ثانية
-- ============================================================

BEGIN;

-- ============================================================
-- 1. Payment: تسريع استعلام billing-summary
-- ============================================================
-- الاستعلام يستخدم: WHERE user_id = ? ORDER BY created_at DESC LIMIT 10
-- مع soft delete (deleted_at IS NULL)
DROP INDEX IF EXISTS idx_payment_user_created;
CREATE INDEX IF NOT EXISTS idx_payment_user_created
    ON "Payment" (user_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- Covering index لنفس الاستعلام لتجنب heap fetches
DROP INDEX IF EXISTS idx_payment_user_created_covering;
CREATE INDEX IF NOT EXISTS idx_payment_user_created_covering
    ON "Payment" (user_id, created_at DESC)
    INCLUDE (amount, status)
    WHERE deleted_at IS NULL;

-- ============================================================
-- 2. Leaderboard: تسريع استعلام الـ Leaderboard
-- ============================================================
-- الاستعلام: ORDER BY total_xp DESC LIMIT ?
DROP INDEX IF EXISTS idx_user_leaderboard;
CREATE INDEX IF NOT EXISTS idx_user_leaderboard
    ON "User" (total_xp DESC)
    INCLUDE (id, name, role, level)
    WHERE status = 'ACTIVE';

-- فهرس مساعد لعد المستخدمين النشطين
DROP INDEX IF EXISTS idx_user_status_xp;
CREATE INDEX IF NOT EXISTS idx_user_status_xp
    ON "User" (status, total_xp DESC)
    WHERE status = 'ACTIVE';

-- ============================================================
-- 3. Achievements: تسريع استعلام الإنجازات
-- ============================================================
-- الاستعلام: WHERE user_id = ? ORDER BY unlocked_at DESC
DROP INDEX IF EXISTS idx_achievement_user_unlocked;
CREATE INDEX IF NOT EXISTS idx_achievement_user_unlocked
    ON "UserAchievement" (user_id, unlocked_at DESC)
    INCLUDE (achievement_id, "achievementKey")
    WHERE deleted_at IS NULL;

-- ============================================================
-- 4. Exams: تسريع استعلام نتائج الامتحانات للمستخدم
-- ============================================================
-- الاستعلام: WHERE user_id = ? ORDER BY taken_at DESC
DROP INDEX IF EXISTS idx_exam_result_user_taken;
CREATE INDEX IF NOT EXISTS idx_exam_result_user_taken
    ON "ExamResult" (user_id, taken_at DESC)
    WHERE deleted_at IS NULL;

-- Covering index لنتائج الامتحانات
DROP INDEX IF EXISTS idx_exam_result_user_taken_covering;
CREATE INDEX IF NOT EXISTS idx_exam_result_user_taken_covering
    ON "ExamResult" (user_id, taken_at DESC)
    INCLUDE (id, exam_id, score, passed)
    WHERE deleted_at IS NULL;

-- ============================================================
-- 5. Recommendations: تسريع استعلام التوصيات (تعطيل لعدم وجود جدول Recommendation)
-- ============================================================
-- الاستعلام: WHERE user_id = ? ORDER BY score DESC


-- ============================================================
-- 6. Subscription/Addons: تسريع استعلام الإضافات
-- ============================================================
DROP INDEX IF EXISTS idx_subscription_addons_user;
CREATE INDEX IF NOT EXISTS idx_subscription_addons_user
    ON "UserSubscription" (user_id, status, end_date DESC)
    INCLUDE (id, plan_id, start_date);

-- ============================================================
-- 7. Activities: تحسين إضافي لـ Activities query (عبر جدول Notification)
-- ============================================================
-- الاستعلامات المتكررة كل ثانية تستخدم WHERE user_id = ? ORDER BY created_at DESC
DROP INDEX IF EXISTS idx_notification_user_created_covering;
CREATE INDEX IF NOT EXISTS idx_notification_user_created_covering
    ON "Notification" (user_id, created_at DESC)
    INCLUDE (id, type, title, message, is_read)
    WHERE deleted_at IS NULL;

-- ============================================================
-- 8. Update planner statistics
-- ============================================================
ANALYZE "Payment";
ANALYZE "User";
ANALYZE "UserAchievement";
ANALYZE "Exam";

ANALYZE "UserSubscription";


COMMIT;