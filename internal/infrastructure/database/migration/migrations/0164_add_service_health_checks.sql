-- service_health_checks stores one row per periodic probe run (database,
-- cache, storage, search, queue, scheduler, api) so the admin panel can show
-- real historical status/latency per service instead of only the live probe
-- taken at request time.
CREATE TABLE IF NOT EXISTS public.service_health_checks (
    checked_at timestamptz NOT NULL,
    service_key varchar(32) NOT NULL,
    status varchar(16) NOT NULL,
    latency_ms double precision NOT NULL DEFAULT 0 CHECK (latency_ms >= 0),
    error_rate double precision,
    details text NOT NULL DEFAULT '',
    PRIMARY KEY (checked_at, service_key)
);

CREATE INDEX IF NOT EXISTS idx_service_health_checks_service_time
    ON public.service_health_checks (service_key, checked_at DESC);
