-- ============================================================
-- Migration 0159: Remove duplicate/redundant refresh_token_hash indexes on UserSession
-- ============================================================
-- Over migrations 0001, 0044, 0047, 0127, 0130, 0137 several overlapping
-- indexes were added on UserSession.refresh_token_hash, each attempting to
-- fix the same slow-query symptom without removing the earlier attempt.
-- As of this migration there are 5 indexes covering refresh_token_hash
-- alone, which is pure write overhead (every INSERT/UPDATE/session-rotation
-- must maintain all of them) with no read benefit, since Postgres only
-- ever uses one per query.
--
-- Kept:
--   idx_usersession_refresh_token_hash        UNIQUE (refresh_token_hash) WHERE deleted_at IS NULL
--     -> exact match for the hot path: WHERE refresh_token_hash = ? AND deleted_at IS NULL
--   idx_user_session_refresh_token_hash_active UNIQUE (refresh_token_hash) WHERE deleted_at IS NULL AND is_active = true
--     -> exact match for the "active session" lookup path
--
-- Dropped (redundant subsets/duplicates of the two kept above):
--   idx_user_session_refresh_hash_deleted   -- exact duplicate of idx_usersession_refresh_token_hash, minus the UNIQUE
--   idx_user_session_refresh_active         -- (refresh_token_hash, is_active) redundant once is_active=true is in predicate
--   idx_user_session_refresh_hash           -- WHERE refresh_token_hash IS NOT NULL, broader/less selective, superseded
-- ============================================================

DROP INDEX CONCURRENTLY IF EXISTS public.idx_user_session_refresh_hash_deleted;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_user_session_refresh_active;
DROP INDEX CONCURRENTLY IF EXISTS public.idx_user_session_refresh_hash;

ANALYZE public."UserSession";
