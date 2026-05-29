CREATE TABLE IF NOT EXISTS public.two_factor_settings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL UNIQUE,
    method varchar(20),
    secret varchar(100),
    is_enabled boolean DEFAULT false,
    is_enforced boolean DEFAULT false,
    backup_codes text[] DEFAULT ARRAY[]::text[],
    verified_devices text[] DEFAULT ARRAY[]::text[],
    pending_setup boolean DEFAULT false,
    activated_at timestamptz,
    deactivated_at timestamptz,
    last_used_at timestamptz,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_two_factor_settings_deleted_at
    ON public.two_factor_settings (deleted_at);

CREATE INDEX IF NOT EXISTS idx_two_factor_settings_user_id
    ON public.two_factor_settings (user_id)
    WHERE deleted_at IS NULL;

ALTER TABLE IF EXISTS public."SubscriptionPlan"
    ADD COLUMN IF NOT EXISTS name_ar text,
    ADD COLUMN IF NOT EXISTS is_active boolean DEFAULT true,
    ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at timestamptz DEFAULT now();

UPDATE public."SubscriptionPlan"
SET name_ar = COALESCE(name_ar, "nameAr", name)
WHERE name_ar IS NULL;

UPDATE public."SubscriptionPlan"
SET is_active = COALESCE(is_active, "isActive", true)
WHERE is_active IS NULL;

UPDATE public."SubscriptionPlan"
SET created_at = COALESCE(created_at, "createdAt", now()),
    updated_at = COALESCE(updated_at, "updatedAt", now())
WHERE created_at IS NULL
   OR updated_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_subscription_plan_is_active
    ON public."SubscriptionPlan" (is_active)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS public."UserSubscription" (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'PENDING',
    start_date timestamptz NOT NULL DEFAULT now(),
    end_date timestamptz NOT NULL,
    auto_renew boolean DEFAULT true,
    paymob_subscription_id text,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now()
);

ALTER TABLE IF EXISTS public."UserSubscription"
    ADD COLUMN IF NOT EXISTS user_id uuid,
    ADD COLUMN IF NOT EXISTS plan_id uuid,
    ADD COLUMN IF NOT EXISTS start_date timestamptz,
    ADD COLUMN IF NOT EXISTS end_date timestamptz,
    ADD COLUMN IF NOT EXISTS auto_renew boolean DEFAULT true,
    ADD COLUMN IF NOT EXISTS paymob_subscription_id text,
    ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at timestamptz DEFAULT now();

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'UserSubscription'
          AND column_name = 'userId'
    ) THEN
        UPDATE public."UserSubscription"
        SET user_id = COALESCE(user_id, "userId");
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'UserSubscription'
          AND column_name = 'planId'
    ) THEN
        UPDATE public."UserSubscription"
        SET plan_id = COALESCE(plan_id, "planId");
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'UserSubscription'
          AND column_name = 'startDate'
    ) THEN
        UPDATE public."UserSubscription"
        SET start_date = COALESCE(start_date, "startDate");
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'UserSubscription'
          AND column_name = 'endDate'
    ) THEN
        UPDATE public."UserSubscription"
        SET end_date = COALESCE(end_date, "endDate");
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'UserSubscription'
          AND column_name = 'createdAt'
    ) THEN
        UPDATE public."UserSubscription"
        SET created_at = COALESCE(created_at, "createdAt");
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'UserSubscription'
          AND column_name = 'updatedAt'
    ) THEN
        UPDATE public."UserSubscription"
        SET updated_at = COALESCE(updated_at, "updatedAt");
    END IF;
END $$;

UPDATE public."UserSubscription"
SET start_date = COALESCE(start_date, now()),
    created_at = COALESCE(created_at, now()),
    updated_at = COALESCE(updated_at, now());

CREATE INDEX IF NOT EXISTS idx_user_subscription_user_status_end
    ON public."UserSubscription" (user_id, status, end_date DESC);

CREATE INDEX IF NOT EXISTS idx_user_subscription_plan_id
    ON public."UserSubscription" (plan_id);
