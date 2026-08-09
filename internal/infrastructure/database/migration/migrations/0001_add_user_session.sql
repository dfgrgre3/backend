-- ==========================================
-- كود #1 (النسخة المحسنة): UserSession Table
-- ==========================================

-- 1. إنشاء الجدول مع إزالة التكرار وإضافة القيود
CREATE TABLE IF NOT EXISTS public."UserSession" (
    id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    
    -- تخزين Hash فقط للتوكن لأسباب أمنية (SHA-256)
    refresh_token_hash text NOT NULL,
    
    -- بيانات الجلسة والتدقيق
    user_agent text,
    ip_address inet NOT NULL, -- استخدام نوع inet بدلاً من text للتحقق من صحة الـ IP
    location text, -- تأكد من توافق هذا الحقل مع سياسة الخصوصية (GDPR/PII)
    device_type text CHECK (device_type IN ('web', 'mobile', 'tablet', 'desktop', 'unknown')),
    
    -- توحيد الحالة في عمود واحد فقط (إزالة is_active المكرر)
    status text NOT NULL DEFAULT 'active' 
        CHECK (status IN ('active', 'expired', 'revoked', 'logged_out')),
    
    -- التواريخ والأوقات
    last_accessed_at timestamptz,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    revoked_by uuid,
    
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Soft Delete
    deleted_at timestamptz,
    
    -- قيود النزاهة المنطقية
    CONSTRAINT chk_session_dates CHECK (expires_at > created_at),
    CONSTRAINT chk_revocation_consistency CHECK (
        (status = 'revoked' AND revoked_at IS NOT NULL) OR 
        (status != 'revoked' AND revoked_at IS NULL)
    )
);

-- 2. الفهارس المحسنة (إزالة الفهارس عديمة الفائدة وإضافة المركبة)
-- فهرس مركب لجلب جلسات المستخدم النشطة بسرعة (يغطي معظم استعلامات التطبيق)
CREATE INDEX IF NOT EXISTS idx_usersession_user_status 
    ON public."UserSession" (user_id, status) 
    WHERE deleted_at IS NULL;

-- فهرس فريد للهاش (ليس التوكن الخام)
CREATE UNIQUE INDEX IF NOT EXISTS idx_usersession_refresh_token_hash 
    ON public."UserSession" (refresh_token_hash) 
    WHERE deleted_at IS NULL;

-- فهرس للتنظيف الدوري للجلسات المنتهية
CREATE INDEX IF NOT EXISTS idx_usersession_expires_cleanup 
    ON public."UserSession" (expires_at) 
    WHERE status = 'active' AND deleted_at IS NULL;

-- 3. المفتاح الأجنبي (تعديل سلوك الحذف ليتوافق مع Soft Delete)
ALTER TABLE public."UserSession" 
    ADD CONSTRAINT fk_usersession_user 
    FOREIGN KEY (user_id) REFERENCES public."User"(id) 
    ON DELETE RESTRICT; -- منع حذف المستخدم إذا كانت له جلسات (لحفظ سجل التدقيق)

-- 4. Trigger لتحديث updated_at تلقائياً
CREATE OR REPLACE FUNCTION fn_update_usersession_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_usersession_updated_at ON public."UserSession";
CREATE TRIGGER trg_usersession_updated_at
    BEFORE UPDATE ON public."UserSession"
    FOR EACH ROW
    EXECUTE FUNCTION fn_update_usersession_timestamp();

-- 5. تعليق توثيقي للجدول
COMMENT ON TABLE public."UserSession" IS 'جدول جلسات المستخدمين - يخزن هاش التوكن فقط ويدعم Soft Delete';
COMMENT ON COLUMN public."UserSession".refresh_token_hash IS 'SHA-256 hash of the refresh token (NEVER store raw tokens)';
COMMENT ON COLUMN public."UserSession".ip_address IS 'عنوان IP بصيغة inet للتحقق التلقائي من الصيغة';