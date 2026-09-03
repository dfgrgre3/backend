-- =============================================================
-- Comprehensive verification of phases 2 & 3 migrations applied
-- 0172 (partition mgmt), 0173 (production tuning), 0174 (autovacuum),
-- 0175 (compat views), 0176 (deleted_at removal Phase A)
-- =============================================================

\echo '=== [1] schema_migrations: 0172-0176 ==='
SELECT id, substring(checksum,1,12) || '...' AS checksum_short, "appliedAt"
FROM schema_migrations
WHERE id IN ('0171_phase1_hardening',
             '0172_install_pg_cron_and_examresult_partition_mgmt',
             '0173_production_postgres_tuning',
             '0174_per_table_autovacuum_tuning',
             '0175_legacy_compat_views',
             '0176_deleted_at_removal_phase_a')
ORDER BY id;

\echo ''
\echo '=== [2] Production tuning: ALTER SYSTEM SET values ==='
SELECT name, setting, unit
FROM pg_settings
WHERE name IN ('log_min_duration_statement','statement_timeout','idle_in_transaction_session_timeout',
               'work_mem','maintenance_work_mem','effective_cache_size','random_page_cost',
               'log_lock_waits','log_temp_files','default_statistics_target',
               'autovacuum_max_workers','autovacuum_naptime')
ORDER BY name;

\echo ''
\echo '=== [3] Autovacuum per-table (custom storage params) ==='
SELECT c.relname AS table_name,
       (SELECT option || '=' || (array_length(replace(option_value,'{','')::text[],1))::text
        FROM unnest(c.reloptions) AS option_value LIMIT 1) AS sample_option
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relkind IN ('r','p')
  AND c.reloptions IS NOT NULL
  AND array_length(c.reloptions,1) > 0
  AND c.reloptions::text LIKE '%autovacuum%'
ORDER BY c.relname;

\echo ''
\echo '=== [4] Compat views (snake_case → PascalCase tables) ==='
SELECT viewname, viewowner
FROM pg_views
WHERE schemaname = 'public'
  AND viewname IN ('users','exam_results','books','payments_view','orders_view',
                   'subscriptions_view','notifications_view','audit_log_view')
ORDER BY viewname;

\echo ''
\echo '=== [5] Compat views: row counts ==='
SELECT 'users' AS view, (SELECT count(*) FROM users) AS rows
UNION ALL SELECT 'exam_results', (SELECT count(*) FROM exam_results)
UNION ALL SELECT 'books', (SELECT count(*) FROM books)
UNION ALL SELECT 'payments_view', (SELECT count(*) FROM payments_view)
UNION ALL SELECT 'orders_view', (SELECT count(*) FROM orders_view)
UNION ALL SELECT 'subscriptions_view', (SELECT count(*) FROM subscriptions_view)
UNION ALL SELECT 'notifications_view', (SELECT count(*) FROM notifications_view)
UNION ALL SELECT 'audit_log_view', (SELECT count(*) FROM audit_log_view);

\echo ''
\echo '=== [6] Helper functions present ==='
SELECT proname, pg_get_function_arguments(oid) AS args
FROM pg_proc
WHERE pronamespace = 'public'::regnamespace
  AND proname IN ('ensure_examresult_partitions','record_delete')
ORDER BY proname;

\echo ''
\echo '=== [7] UserSession.deleted_at removed ==='
SELECT column_name FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = 'UserSession'
  AND column_name = 'deleted_at';

\echo ''
\echo '=== [8] ExamResult partitions (should include 2026_11, 2026_12) ==='
SELECT c.relname AS partition,
       pg_get_expr(c.relpartbound, c.oid) AS bound
FROM pg_inherits i
JOIN pg_class c ON c.oid = i.inhrelid
WHERE inhparent = 'public."ExamResult"'::regclass
ORDER BY c.relname;

\echo ''
\echo '=== [9] Extensions installed ==='
SELECT extname, extversion FROM pg_extension ORDER BY extname;

\echo ''
\echo '=== [10] Tables that still have deleted_at (post 0176) ==='
SELECT c.relname AS table_name
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'deleted_at'
WHERE n.nspname = 'public' AND c.relkind = 'r'
  AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY c.relname;