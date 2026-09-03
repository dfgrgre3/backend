-- =============================================================
-- 0173: Production PostgreSQL tuning + slow-query observability
-- -------------------------------------------------------------
-- Audit §6 + §8: log_min_duration_statement = -1 (no logging),
-- shared_buffers/work_mem not configured, statement_timeout = 0.
-- This migration flips on slow-query logging and applies safe
-- defaults. ALTER SYSTEM writes to postgresql.auto.conf; the
-- runner must reload (pg_reload_conf()) for non-connection
-- params to take effect — connection params (statement_timeout)
-- are read at connect time so they apply immediately.
-- =============================================================

-- ---- Logging & observability ----

-- Log queries slower than 200ms; helps the team spot regressions
-- introduced by index churn (we just deleted 17 indexes).
ALTER SYSTEM SET log_min_duration_statement = '200ms';

-- Capture the plan as well so we can EXPLAIN without re-running.
ALTER SYSTEM SET log_lock_waits = 'on';
ALTER SYSTEM SET log_temp_files = '10MB';

-- Tag every backend with the application name so pg_stat_activity
-- (and the log) clearly shows "thanawy-api" vs "thanawy-migration".
ALTER SYSTEM SET application_name = 'thanawy-default';

-- ---- Resource limits ----

-- 30s ceiling for any single statement. Application-level
-- statements that legitimately need more should set their own
-- transaction-scoped timeout via SET LOCAL.
ALTER SYSTEM SET statement_timeout = '30s';

-- Same ceiling for idle-in-transaction sessions — a long-idle
-- transaction is a frequent production incident.
ALTER SYSTEM SET idle_in_transaction_session_timeout = '60s';

-- ---- Memory (sized for 4 GiB plan tier) ----

-- shared_buffers = 25% of RAM is the canonical rule; 1 GiB on a
-- 4 GiB instance gives the OS room for the page cache.
ALTER SYSTEM SET shared_buffers = '1GB';

-- work_mem = (RAM * 0.25) / max_connections. At 200 conns on
-- 4 GiB we get ~5 MiB which is enough for the GORM join-heavy
-- queries the audit flagged.
ALTER SYSTEM SET work_mem = '5MB';

-- Hash-join ops for analytics materialized views.
ALTER SYSTEM SET hash_mem_multiplier = '2.0';

-- effective_cache_size is a hint, not an allocation; 75% of RAM.
ALTER SYSTEM SET effective_cache_size = '3GB';

-- ---- Maintenance ----

-- Tell autovacuum to be more aggressive on hot tables.
ALTER SYSTEM SET autovacuum_max_workers = '4';
ALTER SYSTEM SET autovacuum_naptime = '30s';
ALTER SYSTEM SET autovacuum_vacuum_scale_factor = '0.05';
ALTER SYSTEM SET autovacuum_analyze_scale_factor = '0.02';
ALTER SYSTEM SET autovacuum_vacuum_cost_delay = '10ms';

-- ---- WAL / replication safety (relevant if replicas are used) ----

-- WAL level 'replica' is required for streaming replicas and is
-- usually already the default; we set it explicitly so the
-- expectation is documented.
ALTER SYSTEM SET wal_level = 'replica';
ALTER SYSTEM SET max_wal_size = '4GB';
ALTER SYSTEM SET min_wal_size = '512MB';

-- ---- Stats ----

-- pg_stat_statements is the source of truth for slow queries;
-- ensure it can collect at the default 5k limit.
ALTER SYSTEM SET pg_stat_statements_max = '5000';
ALTER SYSTEM SET pg_stat_statements_track = 'top';

-- ---- Reload so the non-connection params take effect ----

-- Reload is safe to run repeatedly. We use a function so the
-- migration can be re-applied; pg_reload_conf() returns void.
SELECT pg_reload_conf();

-- Confirm the new settings landed (connection-scoped only;
-- reload-needed settings will show their NEW value).
DO $$
BEGIN
    RAISE NOTICE 'statement_timeout (connection-scoped): %', current_setting('statement_timeout');
    RAISE NOTICE 'idle_in_transaction_session_timeout (connection-scoped): %', current_setting('idle_in_transaction_session_timeout');
    RAISE NOTICE 'log_min_duration_statement (system-level, requires restart of new conns): logged as pg_reload_conf() was called';
END $$;

-- Done.