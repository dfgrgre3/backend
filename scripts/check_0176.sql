-- Check whether 0176 was registered
SELECT id, substring(checksum,1,12) || '...' AS checksum_short, "appliedAt"
FROM schema_migrations
WHERE id LIKE '0176%' OR id LIKE '017%' ORDER BY id;