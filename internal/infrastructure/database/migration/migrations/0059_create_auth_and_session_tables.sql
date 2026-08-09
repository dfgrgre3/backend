-- Migration to create roles, permissions, oauth accounts, verification codes, and login history tables

-- 1. Roles table
CREATE TABLE IF NOT EXISTS "roles" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. Permissions table
CREATE TABLE IF NOT EXISTS "permissions" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    module VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. Role Permissions join table
CREATE TABLE IF NOT EXISTS "role_permissions" (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, permission_id)
);

-- 4. User Roles join table
CREATE TABLE IF NOT EXISTS "user_roles" (
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    assigned_by UUID,
    PRIMARY KEY (user_id, role_id)
);

-- 5. OAuth Accounts table
CREATE TABLE IF NOT EXISTS "oauth_accounts" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_user_id)
);
CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user_id ON "oauth_accounts"(user_id);

-- 6. Verification Codes table
CREATE TABLE IF NOT EXISTS "verification_codes" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    code VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_verification_codes_user_id ON "verification_codes"(user_id);
CREATE INDEX IF NOT EXISTS idx_verification_codes_type ON "verification_codes"(type);
CREATE INDEX IF NOT EXISTS idx_verification_codes_is_used ON "verification_codes"(is_used);

-- 7. Login History table
CREATE TABLE IF NOT EXISTS "login_history" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    ip VARCHAR(45),
    user_agent TEXT,
    status VARCHAR(20) NOT NULL,
    reason VARCHAR(255),
    country VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_login_history_user_id ON "login_history"(user_id);
CREATE INDEX IF NOT EXISTS idx_login_history_created_at ON "login_history"(created_at);
