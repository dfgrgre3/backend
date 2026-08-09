BEGIN;

CREATE TABLE IF NOT EXISTS public.http_metric_buckets (
    bucket_start timestamptz NOT NULL,
    route text NOT NULL,
    method varchar(10) NOT NULL,
    status integer NOT NULL,
    request_count bigint NOT NULL DEFAULT 0 CHECK (request_count >= 0),
    error_count bigint NOT NULL DEFAULT 0 CHECK (error_count >= 0),
    slow_count bigint NOT NULL DEFAULT 0 CHECK (slow_count >= 0),
    duration_sum_ms bigint NOT NULL DEFAULT 0 CHECK (duration_sum_ms >= 0),
    duration_max_ms bigint NOT NULL DEFAULT 0 CHECK (duration_max_ms >= 0),
    p50_ms double precision NOT NULL DEFAULT 0 CHECK (p50_ms >= 0),
    p95_ms double precision NOT NULL DEFAULT 0 CHECK (p95_ms >= 0),
    p99_ms double precision NOT NULL DEFAULT 0 CHECK (p99_ms >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (bucket_start, route, method, status)
);

CREATE INDEX IF NOT EXISTS idx_http_metric_buckets_time
    ON public.http_metric_buckets (bucket_start DESC);

COMMIT;
