-- Migration: Create separate credentials tables for improved security
-- This migration separates authentication credentials from user profile data
-- to improve security and comply with best practices

-- Create UserCredential table
CREATE TABLE IF NOT EXISTS "UserCredential" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    last_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    reset_token TEXT,
    reset_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_user_credential_user FOREIGN KEY (user_id) REFERENCES "User"(id) ON DELETE CASCADE
);

-- Create index on deleted_at for soft deletes
CREATE INDEX IF NOT EXISTS idx_user_credential_deleted_at ON "UserCredential"(deleted_at);

-- Create index on reset_token for password resets
CREATE INDEX IF NOT EXISTS idx_user_credential_reset_token ON "UserCredential"(reset_token) WHERE reset_token IS NOT NULL;

-- Create TwoFactorCredential table
CREATE TABLE IF NOT EXISTS "TwoFactorCredential" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    secret TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    backup_codes JSONB,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_2fa_credential_user FOREIGN KEY (user_id) REFERENCES "User"(id) ON DELETE CASCADE
);

-- Create index on deleted_at for soft deletes
CREATE INDEX IF NOT EXISTS idx_2fa_credential_deleted_at ON "TwoFactorCredential"(deleted_at);

-- Create SessionCredential table
CREATE TABLE IF NOT EXISTS "SessionCredential" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    refresh_token TEXT NOT NULL UNIQUE,
    device_id TEXT,
    user_agent TEXT,
    ip_address TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_session_credential_user FOREIGN KEY (user_id) REFERENCES "User"(id) ON DELETE CASCADE
);

-- Create index on user_id for session lookups
CREATE INDEX IF NOT EXISTS idx_session_credential_user_id ON "SessionCredential"(user_id);

-- Create index on expires_at for session cleanup
CREATE INDEX IF NOT EXISTS idx_session_credential_expires_at ON "SessionCredential"(expires_at);

-- Create index on deleted_at for soft deletes
CREATE INDEX IF NOT EXISTS idx_session_credential_deleted_at ON "SessionCredential"(deleted_at);

-- Migrate existing password hashes from User to UserCredential
INSERT INTO "UserCredential" (user_id, password_hash, last_changed_at, created_at, updated_at)
SELECT 
    id as user_id,
    password_hash,
    COALESCE(password_changed_at, created_at) as last_changed_at,
    created_at,
    updated_at
FROM "User"
WHERE password_hash IS NOT NULL
ON CONFLICT (user_id) DO NOTHING;

-- Migrate existing 2FA data from User to TwoFactorCredential
INSERT INTO "TwoFactorCredential" (user_id, secret, enabled, backup_codes, created_at, updated_at)
SELECT 
    id as user_id,
    two_factor_secret as secret,
    two_factor_enabled as enabled,
    backup_codes::jsonb as backup_codes,
    created_at,
    updated_at
FROM "User"
WHERE two_factor_secret IS NOT NULL
ON CONFLICT (user_id) DO NOTHING;

-- Note: We are NOT dropping the old columns from User yet to allow for rollback
-- The following columns will be removed in a future migration after verification:
-- - password_hash
-- - password_changed_at
-- - password_expires_at
-- - two_factor_secret
-- - two_factor_enabled
-- - backup_codes

-- Add comment to document the migration
COMMENT ON TABLE "UserCredential" IS 'Stores user authentication credentials separately from profile data for improved security';
COMMENT ON TABLE "TwoFactorCredential" IS 'Stores two-factor authentication secrets separately from user profile';
COMMENT ON TABLE "SessionCredential" IS 'Stores session tokens separately from user session data';
