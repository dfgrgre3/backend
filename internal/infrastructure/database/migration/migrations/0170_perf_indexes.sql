-- Migration: 0170_perf_indexes.sql
-- Purpose  : Add performance indexes for hot read paths that are not already
--            covered by the indexes declared in the GORM models. All indexes
--            are created with IF NOT EXISTS so re-running this migration is
--            safe.
--
-- Categories
--   1) Auth / security event lookups (most frequent: per-user, recency)
--   2) Token-store lookups (verification, password reset, refresh, MFA, blocked)
--   3) Analytics time-range scans
--   4) Admin RBAC lookup helpers
--
-- We avoid the CONCURRENTLY keyword here because migrations are applied
-- outside peak traffic; if you apply this on a live large database, switch
-- the CREATE INDEX statements to "CREATE INDEX CONCURRENTLY IF NOT EXISTS"
-- and run outside a transaction.

BEGIN;

-- 1) Auth / security
CREATE INDEX IF NOT EXISTS idx_login_history_user_created
  ON login_history (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_events_user_created
  ON security_events (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_events_unresolved
  ON security_events (created_at DESC)
  WHERE resolved = false;

-- 2) Token stores (compound lookups by user+type+used)
CREATE INDEX IF NOT EXISTS idx_verification_codes_user_type_active
  ON verification_codes (user_id, type)
  WHERE is_used = false;

CREATE INDEX IF NOT EXISTS idx_blocked_tokens_expires
  ON blocked_tokens (expires_at);

-- 3) Analytics time-range scans
--    (analytics_events table is not currently provisioned; indexes are added
--    here for the other analytics tables that ARE present.)
CREATE INDEX IF NOT EXISTS idx_conversion_events_user_created
  ON conversion_events (user_id, created_at DESC)
  WHERE user_id IS NOT NULL;

-- 4) Admin RBAC lookups (cover the mergeCustomRolePermissions + admin UI)
CREATE INDEX IF NOT EXISTS idx_user_roles_role
  ON user_roles (role_id);

CREATE INDEX IF NOT EXISTS idx_role_permissions_perm
  ON role_permissions (permission_id);

CREATE INDEX IF NOT EXISTS idx_permissions_name_module
  ON permissions (name, module);

-- 5) User common lookups (admin "find by email" + listing)
CREATE INDEX IF NOT EXISTS idx_user_status_role
  ON "User" (status, role);

COMMIT;
