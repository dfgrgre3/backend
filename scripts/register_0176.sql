-- Register migration 0176 (was applied directly via psql earlier)
INSERT INTO schema_migrations (id, checksum)
VALUES ('0176_deleted_at_removal_phase_a', '039b6ccb31454841f57ea09e84ef2c25c2b9911a9eb8b24ff6dfa1b8a2ea0a12')
ON CONFLICT (id) DO UPDATE SET checksum = EXCLUDED.checksum
RETURNING id, substring(checksum,1,12) || '...' AS checksum_short, "appliedAt";