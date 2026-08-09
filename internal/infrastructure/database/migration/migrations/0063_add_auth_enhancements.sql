-- =============================================================================
-- Migration 0063: Authentication & Authorization Enhancements
-- 
-- التغييرات:
-- 1. إضافة حقل level إلى roles للـ hierarchy
-- 2. إضافة جداول: user_permissions, trusted_devices, security_events,
--    blocked_tokens, blacklisted_ips, backup_codes, two_factor_secrets
-- 3. إضافة أعمدة جديدة لـ login_history, oauth_accounts, verification_codes
-- 4. تحديث user_sessions مع remember_me و absolute_expires_at
-- 5. إضافة Role Levels constants
-- =============================================================================

BEGIN;

-- ─────────────────────────────────────────────
--  1. إضافة حقل level إلى roles
-- ─────────────────────────────────────────────
ALTER TABLE roles ADD COLUMN IF NOT EXISTS level INT NOT NULL DEFAULT 1;

-- تحديث مستوى كل دور
UPDATE roles SET level = 1 WHERE name = 'STUDENT';
UPDATE roles SET level = 2 WHERE name = 'PARENT';
UPDATE roles SET level = 3 WHERE name = 'TEACHER';
UPDATE roles SET level = 4 WHERE name = 'SUPPORT';
UPDATE roles SET level = 5 WHERE name = 'MODERATOR';
UPDATE roles SET level = 6 WHERE name = 'ADMIN';
UPDATE roles SET level = 7 WHERE name = 'SUPER_ADMIN';

-- إضافة دور SUPPORT إذا لم يكن موجوداً
INSERT INTO roles (name, description, level, is_system)
SELECT 'SUPPORT', 'Support team member who can manage tickets', 4, true
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'SUPPORT');

-- إضافة دور PARENT إذا لم يكن موجوداً
INSERT INTO roles (name, description, level, is_system)
SELECT 'PARENT', 'Parent who can monitor their children', 2, true
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'PARENT');

-- ─────────────────────────────────────────────
--  2. إنشاء جدول user_permissions
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_permissions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    grant_type    VARCHAR(10) NOT NULL DEFAULT 'ALLOW' CHECK (grant_type IN ('ALLOW', 'DENY')),
    created_at    TIMESTAMPTZ DEFAULT now(),
    UNIQUE(user_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_user_permissions_user ON user_permissions(user_id);

COMMENT ON TABLE user_permissions IS 'مستخدم - صلاحيات مباشرة (تجاوز الأدوار)';
COMMENT ON COLUMN user_permissions.grant_type IS 'ALLOW: منح، DENY: منع';

-- ─────────────────────────────────────────────
--  3. إنشاء جدول trusted_devices
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS trusted_devices (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    device_name       VARCHAR(255),
    device_type       VARCHAR(50), -- mobile, tablet, desktop
    fingerprint_hash  VARCHAR(64) NOT NULL,
    user_agent        TEXT,
    ip_address        VARCHAR(45),
    trusted_at        TIMESTAMPTZ DEFAULT now(),
    expires_at        TIMESTAMPTZ NOT NULL,
    last_used_at      TIMESTAMPTZ DEFAULT now(),
    is_active         BOOLEAN DEFAULT true,
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ DEFAULT now(),
    UNIQUE(user_id, fingerprint_hash)
);

CREATE INDEX IF NOT EXISTS idx_trusted_devices_user ON trusted_devices(user_id);
CREATE INDEX IF NOT EXISTS idx_trusted_devices_fingerprint ON trusted_devices(fingerprint_hash);
CREATE INDEX IF NOT EXISTS idx_trusted_devices_expires ON trusted_devices(expires_at);

COMMENT ON TABLE trusted_devices IS 'الأجهزة الموثوقة للمستخدمين (تخطي MFA)';

-- ─────────────────────────────────────────────
--  4. إنشاء جدول security_events
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS security_events (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID REFERENCES "User"(id) ON DELETE SET NULL,
    event_type        VARCHAR(50) NOT NULL,
    severity          VARCHAR(20) NOT NULL DEFAULT 'info' CHECK (severity IN ('info', 'warning', 'critical')),
    ip_address        VARCHAR(45),
    user_agent        TEXT,
    fingerprint_hash  VARCHAR(64),
    location          JSONB,
    metadata          JSONB DEFAULT '{}',
    resolved          BOOLEAN DEFAULT false,
    resolved_by       UUID REFERENCES "User"(id),
    resolved_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_events_user ON security_events(user_id);
CREATE INDEX IF NOT EXISTS idx_security_events_type ON security_events(event_type);
CREATE INDEX IF NOT EXISTS idx_security_events_severity ON security_events(severity);
CREATE INDEX IF NOT EXISTS idx_security_events_created ON security_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_events_unresolved ON security_events(resolved, severity) WHERE resolved = false;

COMMENT ON TABLE security_events IS 'أحداث أمنية (محاولات اختراق، تسجيل مشبوه، إلخ)';

-- ─────────────────────────────────────────────
--  5. إنشاء جدول blocked_tokens
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS blocked_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jti         VARCHAR(64) NOT NULL UNIQUE,
    user_id     UUID REFERENCES "User"(id) ON DELETE SET NULL,
    reason      VARCHAR(50) NOT NULL CHECK (reason IN ('logout', 'revoke', 'rotation', 'security')),
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_blocked_tokens_jti ON blocked_tokens(jti);
CREATE INDEX IF NOT EXISTS idx_blocked_tokens_expires ON blocked_tokens(expires_at);

COMMENT ON TABLE blocked_tokens IS 'قائمة JTI المحظورة (منع إعادة استخدام التوكن)';

-- ─────────────────────────────────────────────
--  6. إنشاء جدول blacklisted_ips
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS blacklisted_ips (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address    VARCHAR(45) NOT NULL,
    reason        VARCHAR(255),
    blocked_by    UUID REFERENCES "User"(id) ON DELETE SET NULL,
    blocked_at    TIMESTAMPTZ DEFAULT now(),
    expires_at    TIMESTAMPTZ,
    is_permanent  BOOLEAN DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_blacklisted_ips_ip ON blacklisted_ips(ip_address);
CREATE INDEX IF NOT EXISTS idx_blacklisted_ips_expires ON blacklisted_ips(expires_at);

COMMENT ON TABLE blacklisted_ips IS 'عناوين IP المحظورة';

-- ─────────────────────────────────────────────
--  7. إنشاء جدول backup_codes
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS backup_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    code_hash   VARCHAR(64) NOT NULL,
    is_used     BOOLEAN DEFAULT false,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(user_id, code_hash)
);

CREATE INDEX IF NOT EXISTS idx_backup_codes_user ON backup_codes(user_id);

COMMENT ON TABLE backup_codes IS 'رموز احتياطية لاستعادة الوصول عند فقدان MFA';

-- ─────────────────────────────────────────────
--  8. إنشاء جدول two_factor_secrets
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS two_factor_secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL UNIQUE REFERENCES "User"(id) ON DELETE CASCADE,
    secret          TEXT NOT NULL,
    method          VARCHAR(20) NOT NULL DEFAULT 'app' CHECK (method IN ('app', 'sms', 'email')),
    phone_number    VARCHAR(20),
    is_enabled      BOOLEAN DEFAULT false,
    enabled_at      TIMESTAMPTZ,
    last_verified   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_two_factor_user ON two_factor_secrets(user_id);

COMMENT ON TABLE two_factor_secrets IS 'أسرار المصادقة الثنائية (TOTP)';

-- ─────────────────────────────────────────────
--  9. تحديث login_history - إضافة أعمدة جديدة
-- ─────────────────────────────────────────────
ALTER TABLE login_history ADD COLUMN IF NOT EXISTS device_fingerprint VARCHAR(64);
ALTER TABLE login_history ADD COLUMN IF NOT EXISTS suspicious BOOLEAN DEFAULT false;
ALTER TABLE login_history ADD COLUMN IF NOT EXISTS mfa_used BOOLEAN DEFAULT false;
ALTER TABLE login_history ADD COLUMN IF NOT EXISTS city VARCHAR(100);
ALTER TABLE login_history ADD COLUMN IF NOT EXISTS region VARCHAR(100);
ALTER TABLE login_history ADD COLUMN IF NOT EXISTS isp VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_login_history_ip ON login_history(ip);
CREATE INDEX IF NOT EXISTS idx_login_history_user_created ON login_history(user_id, created_at DESC);

-- ─────────────────────────────────────────────
--  10. تحديث user_sessions - إضافة أعمدة جديدة
-- ─────────────────────────────────────────────
ALTER TABLE "UserSession" ADD COLUMN IF NOT EXISTS remember_me BOOLEAN DEFAULT false;
ALTER TABLE "UserSession" ADD COLUMN IF NOT EXISTS fingerprint_hash VARCHAR(64);
ALTER TABLE "UserSession" ADD COLUMN IF NOT EXISTS absolute_expires_at TIMESTAMPTZ;

-- تحديث absolute_expires_at للجلسات الحالية (30 يوم من الآن)
UPDATE "UserSession" SET absolute_expires_at = expires_at WHERE absolute_expires_at IS NULL;

-- ─────────────────────────────────────────────
--  11. تحديث oauth_accounts - إضافة أعمدة جديدة
-- ─────────────────────────────────────────────
ALTER TABLE oauth_accounts ADD COLUMN IF NOT EXISTS email VARCHAR(255);
ALTER TABLE oauth_accounts ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE oauth_accounts ADD COLUMN IF NOT EXISTS name VARCHAR(150);
ALTER TABLE oauth_accounts ADD COLUMN IF NOT EXISTS raw_attributes JSONB;

-- ─────────────────────────────────────────────
--  12. تحديث verification_codes - إضافة أعمدة جديدة
-- ─────────────────────────────────────────────
ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS max_attempts INT DEFAULT 5;
ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS attempt_count INT DEFAULT 0;
ALTER TABLE verification_codes ADD COLUMN IF NOT EXISTS last_attempted_at TIMESTAMPTZ;

-- ─────────────────────────────────────────────
--  13. تحديث profiles - إضافة أعمدة جديدة
-- ─────────────────────────────────────────────
-- Skip if profiles table doesn't exist
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'profiles' AND table_schema = 'public') THEN
        ALTER TABLE profiles ADD COLUMN IF NOT EXISTS language VARCHAR(10) DEFAULT 'ar';
        ALTER TABLE profiles ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'Africa/Cairo';
        ALTER TABLE profiles ADD COLUMN IF NOT EXISTS two_factor_method VARCHAR(20) DEFAULT 'none';
    END IF;
END $$;

-- ─────────────────────────────────────────────
--  14. إضافة المعرفة (Comments) للتوثيق
-- ─────────────────────────────────────────────
COMMENT ON COLUMN login_history.device_fingerprint IS 'بصمة الجهاز لتتبع الأجهزة';
COMMENT ON COLUMN login_history.suspicious IS 'هل تسجيل الدخول مشبوه';
COMMENT ON COLUMN login_history.mfa_used IS 'هل تم استخدام MFA';
COMMENT ON COLUMN "UserSession".remember_me IS 'هل المستخدم اختار "تذكرني"';
COMMENT ON COLUMN "UserSession".fingerprint_hash IS 'بصمة الجهاز';
COMMENT ON COLUMN "UserSession".absolute_expires_at IS 'الحد الأقصى لصلاحية الجلسة';

-- Comment on profiles columns only if table exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'profiles' AND table_schema = 'public') THEN
        COMMENT ON COLUMN profiles.two_factor_method IS 'طريقة MFA: app, sms, email, none';
    END IF;
END $$;

COMMENT ON COLUMN verification_codes.max_attempts IS 'الحد الأقصى لمحاولات التحقق';
COMMENT ON COLUMN verification_codes.attempt_count IS 'عدد محاولات التحقق الحالية';

COMMIT;