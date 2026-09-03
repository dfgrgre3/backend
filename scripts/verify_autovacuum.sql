-- Verify per-table autovacuum settings applied by 0174
SELECT
    n.nspname AS schema,
    c.relname AS table_name,
    array_to_string(c.reloptions, E'\n') AS storage_options
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relkind IN ('r','p')
  AND c.reloptions IS NOT NULL
  AND array_length(c.reloptions,1) > 0
ORDER BY c.relname;