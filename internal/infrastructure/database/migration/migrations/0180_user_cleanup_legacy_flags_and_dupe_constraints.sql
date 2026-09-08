-- 0180_user_cleanup_legacy_flags_and_dupe_constraints.sql
--
-- Cleanup found during DB audit (2026-09-06):
--
-- 1. "User"."isDeleted" (legacy boolean) duplicates "User".deleted_at
--    (timestamptz, the actual soft-delete column GORM/application code uses).
--    "isDeleted" is not referenced anywhere in Go code (verified via repo-wide
--    grep) and has zero divergence from deleted_at on current data
--    (`count(*) FILTER (WHERE "isDeleted" != (deleted_at IS NOT NULL))` = 0).
--    Safe to drop as dead/redundant data.
--
-- 2. "User" has two CHECK constraints enforcing the same predicate under
--    different names (added by different migrations without dropping the
--    original): chk_user_ai_credits_nonneg / chk_user_ai_credits_nonnegative,
--    and chk_user_balance_nonneg / chk_user_balance_nonnegative. Drop the
--    older/shorter-named duplicate, keep the *_nonnegative naming (consistent
--    with chk_user_exam_credits_nonnegative, chk_user_streak_nonnegative,
--    chk_user_total_xp_nonnegative already using that suffix).

BEGIN;

ALTER TABLE public."User" DROP CONSTRAINT IF EXISTS chk_user_ai_credits_nonneg;
ALTER TABLE public."User" DROP CONSTRAINT IF EXISTS chk_user_balance_nonneg;

ALTER TABLE public."User" DROP COLUMN IF EXISTS "isDeleted";

COMMIT;
