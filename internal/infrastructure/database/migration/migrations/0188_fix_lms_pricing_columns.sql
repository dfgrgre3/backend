-- Ensure the LMS pricing table includes the discount and subscription metadata
-- expected by the Go models and admin pricing API.

ALTER TABLE IF EXISTS "LmsPricing"
    ADD COLUMN IF NOT EXISTS "discount_price" NUMERIC(19,4),
    ADD COLUMN IF NOT EXISTS "discount_start_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "discount_end_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "subscription_plan_id" UUID;
