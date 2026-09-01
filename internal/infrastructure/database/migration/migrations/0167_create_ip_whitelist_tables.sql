-- Migration to create the IP whitelist tables used by the GORM models.
-- 0021_add_missing_tables.sql created PascalCase variants ("IpWhitelistSetting",
-- "IpWhitelistEntry", "BlockedIpAttempt") with different columns, while the
-- GORM models map to the snake_case names below, so these tables were missing
-- and the security/ip-whitelist endpoints returned 500 (relation does not exist).

-- 1. IP whitelist entries
CREATE TABLE IF NOT EXISTS ip_whitelist_entries (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    ip_address varchar(50) NOT NULL,
    cidr varchar(50),
    description varchar(500),
    type varchar(20) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'active',
    is_temporary boolean NOT NULL DEFAULT false,
    expires_at timestamptz,
    last_used_at timestamptz,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    country varchar(100),
    city varchar(100)
);

CREATE INDEX IF NOT EXISTS idx_ip_whitelist_entries_ip ON ip_whitelist_entries (ip_address);
CREATE INDEX IF NOT EXISTS idx_ip_whitelist_entries_deleted_at ON ip_whitelist_entries (deleted_at);

-- 2. IP whitelist settings (singleton row)
CREATE TABLE IF NOT EXISTS ip_whitelist_settings (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    is_enabled boolean NOT NULL DEFAULT false,
    enforce_for_admins boolean NOT NULL DEFAULT true,
    enforce_for_api boolean NOT NULL DEFAULT false,
    default_action varchar(10) NOT NULL DEFAULT 'allow',
    allow_internal_ips boolean NOT NULL DEFAULT true,
    internal_ip_ranges text[],
    log_blocked_attempts boolean NOT NULL DEFAULT true,
    notify_on_violation boolean NOT NULL DEFAULT true,
    notify_email varchar(255),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ip_whitelist_settings_singleton ON ip_whitelist_settings ((true));

-- 3. Blocked IP attempts
CREATE TABLE IF NOT EXISTS blocked_ip_attempts (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    ip_address varchar(50) NOT NULL,
    endpoint varchar(500),
    method varchar(10),
    user_agent text,
    location varchar(200),
    reason varchar(200) NOT NULL,
    user_id uuid,
    count integer NOT NULL DEFAULT 1,
    attempted_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_blocked_ip_attempts_ip ON blocked_ip_attempts (ip_address);
CREATE INDEX IF NOT EXISTS idx_blocked_ip_attempts_user ON blocked_ip_attempts (user_id);
