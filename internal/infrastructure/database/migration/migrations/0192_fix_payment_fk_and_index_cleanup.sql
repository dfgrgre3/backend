-- 0192_fix_payment_fk_and_index_cleanup.sql
--
-- Addresses findings from the DB review (2026-09-11):
--   1. Payment/Installment/Invoice -> User cascades destroy financial
--      history on user deletion. Switch to RESTRICT so a User row cannot
--      be deleted while payment/invoice/installment records reference it;
--      callers must anonymize/soft-delete the user instead.
--   2. Payment.plan_id had no FK at all, allowing payments to reference
--      a non-existent/deleted SubscriptionPlan.
--   3. Drop exact-duplicate indexes on the hot, write-heavy Payment table
--      (same columns/predicate, added independently across migrations).
--      Only removes indexes that are byte-for-byte redundant with another
--      surviving index -- partial/covering indexes with distinct
--      predicates are left alone.

BEGIN;

-- ── 1. Cascade -> Restrict on financial tables ─────────────────────────

ALTER TABLE public."Payment"
    DROP CONSTRAINT IF EXISTS "fk_User_payments",
    DROP CONSTRAINT IF EXISTS "fk_payments_user_id";

ALTER TABLE public."Payment"
    ADD CONSTRAINT "fk_payments_user_id"
    FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE RESTRICT;

ALTER TABLE public."Invoice"
    DROP CONSTRAINT IF EXISTS "fk_invoices_user_id";

ALTER TABLE public."Invoice"
    ADD CONSTRAINT "fk_invoices_user_id"
    FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE RESTRICT;

ALTER TABLE public."Installment"
    DROP CONSTRAINT IF EXISTS "Installment_user_id_fkey";

ALTER TABLE public."Installment"
    ADD CONSTRAINT "Installment_user_id_fkey"
    FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE RESTRICT;

-- ── 2. Missing FK: Payment.plan_id -> SubscriptionPlan.id ──────────────
-- SET NULL rather than RESTRICT/CASCADE: a retired plan should not block
-- deletion, and the payment record itself must never be lost.

ALTER TABLE public."Payment"
    ADD CONSTRAINT "fk_payment_plan_id"
    FOREIGN KEY (plan_id) REFERENCES public."SubscriptionPlan"(id) ON DELETE SET NULL
    NOT VALID;

ALTER TABLE public."Payment" VALIDATE CONSTRAINT "fk_payment_plan_id";

-- ── 3. Duplicate index cleanup on "Payment" ─────────────────────────────
-- Each pair below is structurally identical (same columns, same type,
-- same predicate); keep the snake_case/idx_-prefixed one as canonical
-- and drop its twin, except where a UNIQUE index already supersedes a
-- plain non-unique one.

DROP INDEX IF EXISTS public."Payment_userId_idx";                 -- dup of idx_Payment_user_id
DROP INDEX IF EXISTS public."idx_Payment_plan_id";                -- dup of idx_payment_plan_id
DROP INDEX IF EXISTS public."Payment_status_idx";                 -- dup of idx_Payment_status
DROP INDEX IF EXISTS public."idx_Payment_external_txn_id";        -- superseded by idx_payment_external_txn_id_unique
DROP INDEX IF EXISTS public."idx_Payment_paymob_order_id";        -- superseded by idx_payment_paymob_order_id_unique
DROP INDEX IF EXISTS public."idx_payment_deleted_at";             -- dup of idx_Payment_deleted_at
DROP INDEX IF EXISTS public."idx_payment_subject";                -- dup of idx_Payment_subject_id
DROP INDEX IF EXISTS public."Payment_userId_createdAt_idx";       -- dup of idx_payment_user_created_desc
DROP INDEX IF EXISTS public."idx_payment_active_user_created";    -- superseded by idx_payment_user_created_covering_safe (same predicate, plus INCLUDE columns)
DROP INDEX IF EXISTS public."idx_payment_user_status_active_safe"; -- dup of idx_payment_user_status_created (identical columns + predicate)

COMMIT;
