-- Migration 0134: align UserSubscription with the soft-delete-aware model/query layer.
-- Existing installations created by migration 0048 did not include this column.

ALTER TABLE IF EXISTS public."UserSubscription"
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_user_subscription_deleted_at
    ON public."UserSubscription" (deleted_at);
