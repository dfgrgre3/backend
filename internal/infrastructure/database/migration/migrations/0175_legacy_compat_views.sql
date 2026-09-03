-- =============================================================
-- 0175: Compatibility views for PascalCase core tables
-- -------------------------------------------------------------
-- Audit §8: PascalCase tables (User, Order, Payment, Subscription,
-- Notification, Book, AuditLog) co-exist with snake_case tables.
-- We create snake_case VIEWs over the PascalCase rows so the
-- admin panel + SQL tooling can address them by either casing.
--
-- IMPORTANT: column naming in the source schema is INCONSISTENT:
--   • User, Order, ExamResult, Book, AuditLog → snake_case columns
--     (after 0113 renamed Book.createdAt/updatedAt → snake_case)
--   • Payment                                → mixed (mostly snake_case,
--     but legacy discount/promo fields are PascalCase: couponId,
--     discountAmount, paymentData, errorMessage, referralRewardId,
--     creditAmount, balanceUsed, promoDiscount, prorationDiscount,
--     archiveReason)
--   • Notification                           → mixed (snake_case after
--     0022 renamed isRead/isDeleted/actionUrl/deletedAt, and 0022
--     added category/priority/status/channels/broadcast_id/actions;
--     0161 added is_active)
--   • Subscription                           → ALL PascalCase columns
--     (legacy Prisma-style schema, never renamed)
--
-- Every view below exposes the ACTUAL column names verified against
-- the DDL migrations, so callers using either casing can keep
-- working until Phase 2 de-dup completes.
--
-- Idempotency: uses CREATE OR REPLACE VIEW (no DROP/CASCADE), so
-- re-running this migration is safe.
--
-- Security: grants SELECT only to the `app_user` and `authenticated`
-- roles (whichever exist). No INSERT/UPDATE/DELETE is granted.
--
-- Verification: the trailing DO $$ block runs a cheap COUNT(*)
-- against every view so any column-name typo fails the migration
-- loudly (via RAISE) instead of being caught by the next deploy.
-- =============================================================

-- ─── users ─── (PascalCase "User") ────────────────────────────────
-- Mirrors the full User column set so snake_case callers keep working.
-- All column names match the GORM struct tags in
-- internal/domain/common/user.go. CREATE OR REPLACE makes this
-- idempotent without dropping dependent objects.
CREATE OR REPLACE VIEW public.users AS
SELECT
    -- Core identity
    id, email, name, username, avatar, role, status,
    status_reason, status_expires_at,
    -- Email / phone verification
    email_verified, phone_verified,
    email_verification_token, email_verification_expires,
    phone_verification_otp, phone_verification_expires,
    phone_verification_attempts, phone_verification_last_sent,
    -- Profile
    phone, country, grade_level, education_type, section, bio,
    city, gender, school, date_of_birth, alternative_phone,
    -- Instructor
    instructor_status, instructor_specialties, instructor_languages,
    commission_rate, experience_years,
    -- Preferences
    wake_up_time, sleep_time, focus_strategy,
    email_notifications, sms_notifications, biometric_enabled,
    two_factor_enabled,
    -- Gamification
    total_xp, level, current_streak, longest_streak,
    total_study_time, tasks_completed, exams_passed,
    study_xp, task_xp, exam_xp, challenge_xp, quest_xp, season_xp,
    -- OAuth / external
    google_id, github_id,
    -- Interests & referral
    interested_subjects, study_goal, subjects_taught, classes_taught,
    referral_code,
    -- Credits & usage
    additional_ai_credits, additional_exam_credits,
    last_usage_reset, monthly_ai_message_count, monthly_exam_count,
    -- Billing
    balance, ai_credits, exam_credits, version,
    -- Subscriptions
    active_subscription_id, subscription_expires_at,
    -- Security / Auth tokens (read-only surface)
    last_login,
    reset_password_token, reset_password_expires,
    magic_link_token, magic_link_expires,
    verification_token, verification_expires,
    -- Permissions
    permissions,
    -- Misc
    archive_reason,
    -- Timestamps
    created_at, updated_at, deleted_at
FROM public."User";

-- ─── exam_results ─── (PascalCase "ExamResult" — partitioned) ────
-- Column names match the DDL in 0000_baseline_schema.sql:1245
-- (snake_case columns; max_score is double precision). The underlying
-- table is PARTITION BY RANGE (taken_at) — plain VIEW forwards
-- partition pruning correctly as long as the planner sees the
-- `taken_at` predicate.
CREATE OR REPLACE VIEW public.exam_results AS
SELECT
    id, user_id, exam_id, score, max_score, passed,
    answers, taken_at, created_at, updated_at, deleted_at
FROM public."ExamResult";

-- ─── books ─── (PascalCase "Book") ────────────────────────────────
-- Final column set after baseline (0000) + 0113 (rename createdAt/updatedAt
-- → snake_case). The baseline DDL exposes BOTH PascalCase and snake_case
-- variants of subject/cover/download (legacy data + new code coexist),
-- so the view exposes both to keep both callers working.
CREATE OR REPLACE VIEW public.books AS
SELECT
    id, title, author, description,
    "subjectId", subject_id,
    "coverUrl", cover_url,
    "downloadUrl", download_url,
    rating, views, downloads, tags,
    price, is_free,
    "uploaderId",
    created_at, updated_at, deleted_at
FROM public."Book";

-- ─── payments ─── (PascalCase "Payment") ──────────────────────────
-- Column names verified from 0000_baseline_schema.sql:1645.
-- Payment uses snake_case for core columns (id, user_id, plan_id,
-- amount, currency, status, method, reference, paymob_order_id,
-- external_txn_id, completed_at, created_at, updated_at, deleted_at,
-- subject_id, order_id) AND PascalCase for the legacy discount /
-- promo columns that pre-date the snake_case rename. Both shapes
-- are exposed so callers don't break.
CREATE OR REPLACE VIEW public.payments_view AS
SELECT
    id, user_id, plan_id, amount, currency, status, method,
    reference, paymob_order_id, external_txn_id,
    completed_at, created_at, updated_at, deleted_at,
    subject_id, order_id,
    "paymentData", "errorMessage",
    "couponId", "discountAmount",
    "referralRewardId", "creditAmount",
    "balanceUsed", "promoDiscount", "prorationDiscount",
    "archiveReason"
FROM public."Payment";

-- ─── orders ─── (PascalCase "Order") ──────────────────────────────
-- Column names verified from 0157_add_wishlist_cart_order.sql and the
-- GORM Order struct (no deleted_at — orders are hard-deleted only).
CREATE OR REPLACE VIEW public.orders_view AS
SELECT
    id, order_number, user_id, status, total, currency,
    payment_method, transaction_id, coupon_code, discount_amount,
    created_at, updated_at
FROM public."Order";

-- ─── subscriptions ─── (PascalCase "Subscription") ───────────────
-- Column names verified from 0000_baseline_schema.sql:2183.
-- Subscription uses ALL PascalCase columns (legacy Prisma-style
-- schema). This is the only view in this file where every column
-- name is PascalCase.
CREATE OR REPLACE VIEW public.subscriptions_view AS
SELECT
    id, "userId", "planId", status, "startDate", "endDate",
    "gracePeriodEndDate", "createdAt", "updatedAt"
FROM public."Subscription";

-- ─── notifications ─── (PascalCase "Notification") ───────────────
-- Final column set after baseline (0000) + 0022 (rename PascalCase
-- → snake_case and add category/priority/status/channels/broadcast_id/
-- actions) + 0161 (add is_active). All columns are snake_case now.
CREATE OR REPLACE VIEW public.notifications_view AS
SELECT
    id, user_id, title, message, type, icon, link,
    is_read, is_deleted, action_url,
    category, priority, status,
    channels, broadcast_id, actions,
    is_active,
    created_at, updated_at, deleted_at
FROM public."Notification";

-- ─── audit_log ─── (PascalCase "AuditLog") ───────────────────────
-- Column names verified from 0021_add_missing_tables.sql:6.
-- Note: `severity` and `status` exist in the DDL but NOT in the Go
-- AuditLog struct — they are read-only here so the audit pipeline
-- can still report on them from SQL tooling.
CREATE OR REPLACE VIEW public.audit_log_view AS
SELECT
    id, user_id, event_type, severity, action, resource,
    resource_id, changes, metadata, ip_address, user_agent,
    device_info, location, status, created_at
FROM public."AuditLog";

-- ─── Grants: let app_user + authenticated SELECT the views ──────
-- `app_user` is the application role; `authenticated` is the
-- PostgREST/Supabase convention. We grant to whichever roles exist.
DO $$
DECLARE
    view_name text;
    views text[] := ARRAY[
        'users', 'exam_results', 'books', 'payments_view',
        'orders_view', 'subscriptions_view',
        'notifications_view', 'audit_log_view'
    ];
BEGIN
    FOREACH view_name IN ARRAY views LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
            EXECUTE format('GRANT SELECT ON public.%I TO app_user', view_name);
        END IF;
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
            EXECUTE format('GRANT SELECT ON public.%I TO authenticated', view_name);
        END IF;
    END LOOP;
    RAISE NOTICE 'Granted SELECT on % compatibility views', array_length(views, 1);
END $$;

-- ─── Documentation comments ──────────────────────────────────────
COMMENT ON VIEW public.users             IS 'snake_case compatibility view over PascalCase "User" table — Phase 2 de-dup';
COMMENT ON VIEW public.exam_results       IS 'snake_case compatibility view over PascalCase "ExamResult" (partitioned by taken_at) — Phase 2 de-dup';
COMMENT ON VIEW public.books              IS 'snake_case compatibility view over PascalCase "Book" table (exposes both PascalCase and snake_case variants) — Phase 2 de-dup';
COMMENT ON VIEW public.payments_view      IS 'snake_case compatibility view over PascalCase "Payment" table (mixed snake/Pascal columns) — Phase 2 de-dup';
COMMENT ON VIEW public.orders_view        IS 'snake_case compatibility view over PascalCase "Order" table — Phase 2 de-dup';
COMMENT ON VIEW public.subscriptions_view IS 'snake_case compatibility view over PascalCase "Subscription" table (ALL PascalCase columns) — Phase 2 de-dup';
COMMENT ON VIEW public.notifications_view IS 'snake_case compatibility view over PascalCase "Notification" table — Phase 2 de-dup';
COMMENT ON VIEW public.audit_log_view     IS 'snake_case compatibility view over PascalCase "AuditLog" table — Phase 2 de-dup';

-- ─── Smoke tests ─────────────────────────────────────────────────
-- Run a cheap COUNT(*) on every view to confirm the SELECT path
-- works through the planner (catches missing-column mistakes before
-- the migration is marked applied). These do NOT modify data.
DO $$
DECLARE
    v_count bigint;
BEGIN
    SELECT COUNT(*) INTO v_count FROM public.users;
    RAISE NOTICE 'users: % rows visible', v_count;

    SELECT COUNT(*) INTO v_count FROM public.exam_results;
    RAISE NOTICE 'exam_results: % rows visible', v_count;

    SELECT COUNT(*) INTO v_count FROM public.books;
    RAISE NOTICE 'books: % rows visible', v_count;

    SELECT COUNT(*) INTO v_count FROM public.payments_view;
    RAISE NOTICE 'payments_view: % rows visible', v_count;

    SELECT COUNT(*) INTO v_count FROM public.orders_view;
    RAISE NOTICE 'orders_view: % rows visible', v_count;

    SELECT COUNT(*) INTO v_count FROM public.subscriptions_view;
    RAISE NOTICE 'subscriptions_view: % rows visible', v_count;

    SELECT COUNT(*) INTO v_count FROM public.notifications_view;
    RAISE NOTICE 'notifications_view: % rows visible', v_count;

    SELECT COUNT(*) INTO v_count FROM public.audit_log_view;
    RAISE NOTICE 'audit_log_view: % rows visible', v_count;
END $$;

-- Done.