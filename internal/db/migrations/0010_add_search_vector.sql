-- ==========================================
-- كود #2 (النسخة المحسنة): Full-Text Search for Subjects
-- ==========================================

-- 1. إضافة عمود البحث مع قيد NOT NULL وقيمة افتراضية
ALTER TABLE "Subject" 
    ADD COLUMN IF NOT EXISTS search_vector tsvector NOT NULL DEFAULT ''::tsvector;

COMMENT ON COLUMN "Subject".search_vector IS 'TSVector محسوب تلقائياً للبحث النصي الكامل - يدعم العربية والإنجليزية مع أوزان أولوية';

-- 2. إنشاء فهرس GIN جزئي وآمن للإنتاج
-- CONCURRENTLY لا يقفل الجدول (يتطلب عدم وجود transaction مفتوح)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subject_search_vector 
    ON "Subject" USING GIN(search_vector)
    WHERE deleted_at IS NULL; -- استبعاد السجلات المحذوفة ناعماً

-- 3. دالة محسنة مع أوزان ودعم للنصوص الطويلة
CREATE OR REPLACE FUNCTION update_subject_search_vector()
RETURNS trigger AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.code, ''), 100)), 'A') ||
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.name, ''), 500)), 'B') ||
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.name_ar, ''), 500)), 'B') ||
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.description, ''), 2000)), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION update_subject_search_vector() IS 'تحديث search_vector مع أوزان: Code(A), Name/NameAr(B), Description(D)';

-- 4. Trigger محدد النطاق
DROP TRIGGER IF EXISTS trg_subject_search_vector ON "Subject";
CREATE TRIGGER trg_subject_search_vector
    BEFORE INSERT OR UPDATE OF name, name_ar, description, code
    ON "Subject"
    FOR EACH ROW
    EXECUTE FUNCTION update_subject_search_vector();

-- 5. تعبئة البيانات الموجودة بشكل آمن (Batch-Safe)
-- يُنفذ فقط للسجلات التي لم تُحسب بعد أو كانت NULL سابقاً
UPDATE "Subject"
SET search_vector = 
    setweight(to_tsvector('simple', LEFT(COALESCE(code, ''), 100)), 'A') ||
    setweight(to_tsvector('simple', LEFT(COALESCE(name, ''), 500)), 'B') ||
    setweight(to_tsvector('simple', LEFT(COALESCE(name_ar, ''), 500)), 'B') ||
    setweight(to_tsvector('simple', LEFT(COALESCE(description, ''), 2000)), 'D')
WHERE search_vector = ''::tsvector 
   OR search_vector IS NULL;