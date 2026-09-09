CREATE TABLE IF NOT EXISTS public.newsletter_subscribers (
    email VARCHAR(254) PRIMARY KEY,
    subscribed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_newsletter_subscribers_subscribed_at
    ON public.newsletter_subscribers (subscribed_at DESC);