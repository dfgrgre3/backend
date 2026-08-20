-- ==========================================
-- كود #3 (النسخة المحسنة): Security Audit & IP Whitelisting
-- ==========================================

-- 1. جدول التدقيق الموحد (دمج AuditLog + security_audit_logs)
CREATE TABLE IF NOT EXISTS public."AuditLog" (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid,
    event_type text NOT NULL CHECK (event_type IN (
        'login', 'logout', 'session_created', 'session_revoked',
        'ip_blocked', 'ip_whitelisted', 'permission_changed',
        'data_accessed', 'data_modified', 'security_violation', 'system'
    )),
    severity text NOT NULL DEFAULT 'info' CHECK (severity IN ('debug', 'info', 'warning', 'error', 'critical')),
    action text,
    resource text,
    resource_id text,
    changes jsonb DEFAULT '{}'::jsonb,      -- كان text، الآن jsonb للاستعلام الداخلي
    metadata jsonb DEFAULT '{}'::jsonb,     -- كان text، الآن jsonb
    ip_address inet,                        -- كان text، الآن inet
    user_agent text,
    device_info text,
    location text,
    status text NOT NULL DEFAULT 'unread' CHECK (status IN ('unread', 'read', 'archived')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE public."AuditLog" IS 'سجل تدقيق أمني موحد - يجمع تدقيقات المستخدمين والأمان في جدول واحد';
COMMENT ON COLUMN public."AuditLog".changes IS 'JSONB: الفرق بين القيم القديمة والجديدة عند التعديل';
COMMENT ON COLUMN public."AuditLog".metadata IS 'JSONB: بيانات إضافية مرنة حسب نوع الحدث';

-- فهارس AuditLog
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auditlog_user_created 
    ON public."AuditLog" (user_id, created_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auditlog_event_severity 
    ON public."AuditLog" (event_type, severity) WHERE severity IN ('warning', 'error', 'critical');
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auditlog_ip_created 
    ON public."AuditLog" (ip_address, created_at DESC) WHERE ip_address IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auditlog_status 
    ON public."AuditLog" (status) WHERE status = 'unread';
-- فهرس GIN للاستعلام داخل JSONB
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auditlog_changes_gin 
    ON public."AuditLog" USING GIN(changes);


-- 2. إعدادات القائمة البيضاء للـ IP
CREATE TABLE IF NOT EXISTS public."IpWhitelistSetting" (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    is_enabled boolean NOT NULL DEFAULT false,
    enforce_for_admins boolean NOT NULL DEFAULT true,
    enforce_for_api boolean NOT NULL DEFAULT false,
    default_action text NOT NULL DEFAULT 'deny' CHECK (default_action IN ('allow', 'deny')),
    allow_internal_ips boolean NOT NULL DEFAULT true,
    log_blocked_attempts boolean NOT NULL DEFAULT true,
    notify_on_violation boolean NOT NULL DEFAULT true,
    notify_email text CHECK (notify_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}$'),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE public."IpWhitelistSetting" IS 'إعدادات عالمية للقائمة البيضاء - صف واحد فقط (Singleton Pattern)';

-- قيد لضمان صف واحد فقط (Singleton)
CREATE UNIQUE INDEX IF NOT EXISTS idx_ipwhitelistsetting_singleton 
    ON public."IpWhitelistSetting" ((true));


-- 3. إدخالات القائمة البيضاء
CREATE TABLE IF NOT EXISTS public."IpWhitelistEntry" (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    ip_address inet NOT NULL,               -- كان text، الآن inet
    cidr cidr,                              -- نوع منفصل لـ CIDR
    description text,
    type text NOT NULL CHECK (type IN ('permanent', 'temporary', 'internal', 'vpn', 'office')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'expired')),
    expires_at timestamptz,
    last_used_at timestamptz,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT chk_temp_expiry CHECK (
        (type = 'temporary' AND expires_at IS NOT NULL) OR
        (type != 'temporary')
    )
);

COMMENT ON TABLE public."IpWhitelistEntry" IS 'إدخالات القائمة البيضاء مع دعم IPv4/IPv6 وCIDR';

-- فهارس IpWhitelistEntry
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipwhitelistentry_ip_status 
    ON public."IpWhitelistEntry" (ip_address, status) WHERE status = 'active';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipwhitelistentry_cidr 
    ON public."IpWhitelistEntry" USING GIST(cidr inet_ops) WHERE cidr IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipwhitelistentry_expires 
    ON public."IpWhitelistEntry" (expires_at) WHERE type = 'temporary' AND status = 'active';


-- 4. محاولات الـ IP المحظورة (كل محاولة = صف مستقل، بدون count)
CREATE TABLE IF NOT EXISTS public."BlockedIpAttempt" (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    ip_address inet NOT NULL,               -- كان text، الآن inet
    endpoint text,
    method text CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS', 'HEAD')),
    user_agent text,
    location text,
    reason text NOT NULL,
    user_id uuid,
    attempted_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE public."BlockedIpAttempt" IS 'سجل محاولات الوصول المحظورة - كل محاولة صف مستقل للتدقيق الدقيق';

-- فهارس BlockedIpAttempt
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_blockedip_ip_attempted 
    ON public."BlockedIpAttempt" (ip_address, attempted_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_blockedip_attempted_desc 
    ON public."BlockedIpAttempt" (attempted_at DESC);


-- 5. المفاتيح الأجنبية
ALTER TABLE public."AuditLog" 
    ADD CONSTRAINT fk_auditlog_user 
    FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE SET NULL;

ALTER TABLE public."IpWhitelistEntry" 
    ADD CONSTRAINT fk_ipwhitelistentry_createdby 
    FOREIGN KEY (created_by) REFERENCES public."User"(id) ON DELETE RESTRICT;

ALTER TABLE public."BlockedIpAttempt" 
    ADD CONSTRAINT fk_blockedip_user 
    FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE SET NULL;


-- 6. Trigger موحد لتحديث updated_at (إعادة استخدام وتعميم)
CREATE OR REPLACE FUNCTION fn_update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ipwhitelistsetting_updated ON public."IpWhitelistSetting";
CREATE TRIGGER trg_ipwhitelistsetting_updated
    BEFORE UPDATE ON public."IpWhitelistSetting"
    FOR EACH ROW EXECUTE FUNCTION fn_update_timestamp();

DROP TRIGGER IF EXISTS trg_ipwhitelistentry_updated ON public."IpWhitelistEntry";
CREATE TRIGGER trg_ipwhitelistentry_updated
    BEFORE UPDATE ON public."IpWhitelistEntry"
    FOR EACH ROW EXECUTE FUNCTION fn_update_timestamp();