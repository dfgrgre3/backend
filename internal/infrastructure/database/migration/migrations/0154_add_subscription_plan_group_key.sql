-- 0154_add_subscription_plan_group_key.sql
-- Adds group_key to SubscriptionPlan so multiple interval variants (MONTHLY /
-- YEARLY / FOREVER) of the same plan tier can be linked together — e.g. a
-- "Premium" tier with a monthly-priced row and a separately-priced yearly
-- row. The frontend billing-cycle toggle uses this to switch between the
-- real plan record for the selected interval instead of guessing a price.
--
-- Existing plans get group_key backfilled to their own id so each currently
-- stands as its own single-interval group (no behavior change until an
-- admin explicitly links a yearly variant to a monthly one).

ALTER TABLE IF EXISTS public."SubscriptionPlan"
    ADD COLUMN IF NOT EXISTS group_key VARCHAR(64);

UPDATE public."SubscriptionPlan"
    SET group_key = id
    WHERE group_key IS NULL;

CREATE INDEX IF NOT EXISTS idx_subscription_plan_group_key
    ON public."SubscriptionPlan" (group_key);
