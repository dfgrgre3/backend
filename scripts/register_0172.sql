INSERT INTO schema_migrations (id, checksum)
VALUES ('0172_install_pg_cron_and_examresult_partition_mgmt', 'ee0726e68d9246dd5363daa194a2724458a267e428953b9161927647cb354bac')
ON CONFLICT (id) DO UPDATE SET checksum = EXCLUDED.checksum
RETURNING id, checksum, "appliedAt";