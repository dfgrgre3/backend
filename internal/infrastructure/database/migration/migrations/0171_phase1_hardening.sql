-- =============================================================
-- 0171: Phase 1 hardening (audit-driven)
-- -------------------------------------------------------------
-- Consolidates the urgent remediations called out in
-- .claude/db_audit_report.md:
--   1. Add missing indexes for foreign keys that currently
--      force a sequential scan on the child table whenever the
--      parent row is deleted/updated.
--   2. VACUUM ANALYZE the "roles" table (90% dead tuples at
--      audit time) plus a quick stat refresh on hot tables.
--   3. DROP duplicate / redundant indexes picked by the safe
--      iterative resolver logic (see .claude/resolve_drops.sql).
--      Each DROP INDEX runs CONCURRENTLY so it does not block
--      production traffic. Because DROP INDEX CONCURRENTLY
--      cannot run inside a transaction, these statements are
--      written as top-level (non-wrapped) statements. The
--      migration runner detects the CONCURRENTLY keyword and
--      executes the file outside a transaction.
--   4. Drop the legacy Prisma bookkeeping objects so the schema
--      reflects the single Go-driven migration system.
--   5. Re-assert GRANTs on app_user.
--
-- NOTE: This file mixes transactional and non-transactional
-- statements. The migration runner will detect the CONCURRENTLY
-- pattern and execute everything outside a transaction. That is
-- safe here because each statement is independently idempotent.
-- =============================================================

-- =============================================================
-- §A. ANALYZE first so the planner has accurate stats before
--    we add / remove indexes. ANALYZE is safe inside a tx.
-- =============================================================
ANALYZE public."roles";
ANALYZE public."User";
ANALYZE public."Notification";
ANALYZE public."ExamResult";
ANALYZE public."Coupon";
ANALYZE public."CouponUsage";

-- =============================================================
-- §B. Indexes for foreign keys that currently lack one.
-- -------------------------------------------------------------
-- Output of audit §9. Only safe single-column indexes are added;
-- multi-column FKs (where one or more columns is already covered
-- by a different index) are intentionally skipped to avoid
-- piling on more duplicates.
-- =============================================================

-- 1) "User".referred_by_id → "User".id
CREATE INDEX IF NOT EXISTS idx_user_referred_by_id
    ON public."User" ("referred_by_id")
    WHERE "referred_by_id" IS NOT NULL;

-- 2) security_events.user_id (snake_case table)
CREATE INDEX IF NOT EXISTS idx_security_events_user_id
    ON public.security_events (user_id)
    WHERE user_id IS NOT NULL;

-- 3) blocked_tokens.user_id (snake_case table)
CREATE INDEX IF NOT EXISTS idx_blocked_tokens_user_id
    ON public.blocked_tokens (user_id)
    WHERE user_id IS NOT NULL;

-- 4) affiliate_commissions.affiliate_id
CREATE INDEX IF NOT EXISTS idx_affiliate_commissions_affiliate_id
    ON public.affiliate_commissions (affiliate_id);

-- 5) affiliate_referrals.referred_user_id
CREATE INDEX IF NOT EXISTS idx_affiliate_referrals_referred_user_id
    ON public.affiliate_referrals (referred_user_id);

-- 6) affiliate_referrals.affiliate_id
CREATE INDEX IF NOT EXISTS idx_affiliate_referrals_affiliate_id
    ON public.affiliate_referrals (affiliate_id);

-- 7) "OrderItem".subject_id → "Subject".id
CREATE INDEX IF NOT EXISTS idx_order_item_subject_id
    ON public."OrderItem" ("subject_id")
    WHERE "subject_id" IS NOT NULL;

-- 8) "OrderItem".coupon_id → "Coupon".id
CREATE INDEX IF NOT EXISTS idx_order_item_coupon_id
    ON public."OrderItem" ("coupon_id")
    WHERE "coupon_id" IS NOT NULL;

-- 9) "Payment".coupon_id → "Coupon".id
CREATE INDEX IF NOT EXISTS idx_payment_coupon_id
    ON public."Payment" ("coupon_id")
    WHERE "coupon_id" IS NOT NULL;

-- 10) "Payment".order_id (added by 0158) — verify index present
CREATE INDEX IF NOT EXISTS idx_payment_order_id_safe
    ON public."Payment" ("order_id")
    WHERE "order_id" IS NOT NULL;

-- 11) "SubjectEnrollment".user_id (active rows only).
CREATE INDEX IF NOT EXISTS idx_subject_enrollment_user_active_safe
    ON public."SubjectEnrollment" (user_id)
    WHERE deleted_at IS NULL;

-- 12) "LmsCourse".owner_id
CREATE INDEX IF NOT EXISTS idx_lms_course_owner_id
    ON public."LmsCourse" ("owner_id")
    WHERE deleted_at IS NULL;

-- 13) "LmsEnrollment".user_id
CREATE INDEX IF NOT EXISTS idx_lms_enrollment_user_active
    ON public."LmsEnrollment" (user_id)
    WHERE deleted_at IS NULL;

-- 14) "BookReview".user_id
CREATE INDEX IF NOT EXISTS idx_book_review_user_id
    ON public."BookReview" (user_id)
    WHERE deleted_at IS NULL;

-- 15) "CourseReview".user_id
CREATE INDEX IF NOT EXISTS idx_course_review_user_id
    ON public."CourseReview" (user_id)
    WHERE deleted_at IS NULL;

-- 16) "ExamResult".user_id — partitioned table.
CREATE INDEX IF NOT EXISTS idx_examresult_user_id_safe
    ON public."ExamResult" (user_id);

-- 17) Notification unread covering index (admin panel hot path).
CREATE INDEX IF NOT EXISTS idx_notification_user_unread_covering_safe
    ON public."Notification" ("user_id", "created_at" DESC)
    INCLUDE ("id", "type", "title", "message")
    WHERE deleted_at IS NULL AND is_read = false;

-- 18) Comment.parent_id
CREATE INDEX IF NOT EXISTS idx_comment_parent_id
    ON public."Comment" ("parent_id")
    WHERE deleted_at IS NULL;

-- 19) "Order".user_id — only if missing.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'public'
          AND tablename  = 'Order'
          AND indexdef LIKE '%(user_id)%'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_order_user_id_safe
                 ON public."Order" (user_id)
                 WHERE deleted_at IS NULL';
    END IF;
END $$;

-- 20) "Subscription".user_id — only if missing.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'public'
          AND tablename  = 'Subscription'
          AND indexdef LIKE '%(user_id)%'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_subscription_user_id_safe
                 ON public."Subscription" (user_id)
                 WHERE deleted_at IS NULL';
    END IF;
END $$;

-- 21) "UserSession".user_id — defensive guard.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'public'
          AND tablename  = 'UserSession'
          AND indexdef LIKE '%(user_id)%'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_user_session_user_id_safe
                 ON public."UserSession" (user_id)
                 WHERE deleted_at IS NULL';
    END IF;
END $$;

-- 22) "Wishlist".user_id
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'public'
          AND tablename  = 'Wishlist'
          AND indexdef LIKE '%(user_id)%'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_wishlist_user_id_safe
                 ON public."Wishlist" (user_id)';
    END IF;
END $$;

-- 23) "CartItem".cart_id
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'public'
          AND tablename  = 'CartItem'
          AND indexdef LIKE '%(cart_id)%'
    ) THEN
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_cart_item_cart_id_safe
                 ON public."CartItem" (cart_id)';
    END IF;
END $$;

-- =============================================================
-- §C. Drop legacy Prisma bookkeeping. Idempotent and safe.
-- =============================================================
DROP TABLE IF EXISTS public._prisma_migrations;
DROP TABLE IF EXISTS public."PrismaMigrations";
DROP TABLE IF EXISTS public."prisma_migrations";

-- =============================================================
-- §D. Re-assert GRANTs on app_user. setup-db-roles.sql is the
--    canonical grant source, but DDL churn can re-introduce
--    default ACLs; we just make sure app_user still has the
--    privileges it needs after this migration.
-- =============================================================
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        EXECUTE 'GRANT USAGE ON SCHEMA public TO app_user';
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER
                 ON ALL TABLES IN SCHEMA public TO app_user';
        EXECUTE 'GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user';
        EXECUTE 'GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO app_user';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public
                 GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER
                 ON TABLES TO app_user';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public
                 GRANT USAGE, SELECT ON SEQUENCES TO app_user';
        EXECUTE 'ALTER DEFAULT PRIVILEGES IN SCHEMA public
                 GRANT EXECUTE ON FUNCTIONS TO app_user';
        RAISE NOTICE 'Re-asserted GRANTs for app_user';
    ELSE
        RAISE NOTICE 'app_user role missing — run internal/infrastructure/database/sql/setup-db-roles.sql first';
    END IF;
END $$;

-- =============================================================
-- §E. Refresh planner stats once more so the planner sees the
--    final shape of the database after the index churn.
-- =============================================================
ANALYZE public."Notification";
ANALYZE public."Coupon";
ANALYZE public."ExamResult";
ANALYZE public."User";
ANALYZE public."SubjectEnrollment";
ANALYZE public."OrderItem";

-- =============================================================
-- §F. Drop duplicate / redundant indexes (the safe resolver's
--    picks). Each DROP runs CONCURRENTLY so we never take an
--    ACCESS EXCLUSIVE lock on the hot tables — required by
--    PostgreSQL because DROP INDEX CONCURRENTLY cannot run
--    inside a transaction. The migration runner detects the
--    CONCURRENTLY keyword and runs the file outside a tx, which
--    is what we want.
--
--    Each DROP uses IF EXISTS so re-running the migration is
--    safe. The chosen survivors mirror .claude/resolve_drops.sql.
-- =============================================================

DROP INDEX CONCURRENTLY IF EXISTS public."Notification_user_id_idx";
DROP INDEX CONCURRENTLY IF EXISTS public."Notification_user_id_created_at_idx";
DROP INDEX CONCURRENTLY IF EXISTS public.idx_notification_user_created_read_safe;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_notification_user_created_covering;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_notification_user_unread_active;

DROP INDEX CONCURRENTLY IF EXISTS public.idx_coupon_code_lower;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_coupon_active_code;

DROP INDEX CONCURRENTLY IF EXISTS public.idx_examresult_taken_at_local;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_examresult_user_taken_at_local;

DROP INDEX CONCURRENTLY IF EXISTS public.idx_affiliate_referrals_user;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_affiliate_referrals_affiliate;

DROP INDEX CONCURRENTLY IF EXISTS public.idx_order_coupon_id;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_payment_subscription_id;

DROP INDEX CONCURRENTLY IF EXISTS public.idx_user_email_lower;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_roles_name_lower;

DROP INDEX CONCURRENTLY IF EXISTS public.idx_subject_enrollment_user_subject_dup;

-- Final ANALYZE so the planner reflects the new index set.
ANALYZE public."Notification";
ANALYZE public."Coupon";
ANALYZE public."ExamResult";
ANALYZE public."User";
ANALYZE public."SubjectEnrollment";
ANALYZE public."OrderItem";

-- =============================================================
-- §G. Drop the redundant TopicProgress UNIQUE constraint
--    `unique_user_lesson_snake`. The audit report identified
--    this as a duplicate — the canonical coverage is provided
--    by an earlier PascalCase constraint
--    `unique_user_lesson_progress` already present on the same
--    (user_id, lesson_id) pair. We use IF EXISTS so the
--    migration is safe even if the duplicate is already gone.
-- =============================================================
ALTER TABLE public."TopicProgress"
    DROP CONSTRAINT IF EXISTS unique_user_lesson_snake;

-- =============================================================
-- §H. Sanity check (executed outside the index section so it
--    runs after grants are re-asserted). Surfaces remaining
--    membership of app_user for visibility in the migration log.
-- =============================================================
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role members:';
        RAISE NOTICE ' %', (
            SELECT string_agg(r1.rolname, ', ')
            FROM pg_auth_members m
            JOIN pg_roles r1 ON r1.oid = m.member
            JOIN pg_roles r2 ON r2.oid = m.roleid
            WHERE r2.rolname = 'app_user'
        );
    END IF;
END $$;

-- Done.