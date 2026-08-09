-- Course Pricing and Bundles
-- Migration: 0108_add_course_pricing_and_bundles.sql

-- ============================================================
-- 1. Course Pricing Configuration
-- ============================================================

CREATE TABLE IF NOT EXISTS course_pricing (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL UNIQUE REFERENCES "Subject"(id) ON DELETE CASCADE,
    pricing_type VARCHAR(50) NOT NULL DEFAULT 'ONE_TIME'
        CHECK (pricing_type IN ('FREE', 'ONE_TIME', 'SUBSCRIPTION', 'BUNDLE')),
    price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
    currency VARCHAR(10) NOT NULL DEFAULT 'EGP'
        CHECK (currency IN ('EGP', 'USD', 'EUR', 'SAR', 'AED', 'GBP')),
    discount_price DECIMAL(12,2) DEFAULT NULL,
    discount_start_at TIMESTAMPTZ DEFAULT NULL,
    discount_end_at TIMESTAMPTZ DEFAULT NULL,
    subscription_plan_id UUID DEFAULT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE course_pricing IS 'Stores pricing configuration for each course: free, one-time, subscription, or bundle.';
COMMENT ON COLUMN course_pricing.pricing_type IS 'FREE: no payment required. ONE_TIME: single payment. SUBSCRIPTION: recurring via plan. BUNDLE: part of a course bundle.';
COMMENT ON COLUMN course_pricing.discount_price IS 'Discounted price during promotion period.';

-- ============================================================
-- 2. Course Bundles
-- ============================================================

CREATE TABLE IF NOT EXISTS course_bundles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    name_ar VARCHAR(255) DEFAULT NULL,
    description TEXT DEFAULT NULL,
    description_ar TEXT DEFAULT NULL,
    price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
    currency VARCHAR(10) NOT NULL DEFAULT 'EGP'
        CHECK (currency IN ('EGP', 'USD', 'EUR', 'SAR', 'AED', 'GBP')),
    discount_price DECIMAL(12,2) DEFAULT NULL,
    discount_percentage DECIMAL(5,2) DEFAULT NULL,
    discount_start_at TIMESTAMPTZ DEFAULT NULL,
    discount_end_at TIMESTAMPTZ DEFAULT NULL,
    course_ids UUID[] DEFAULT '{}',
    thumbnail_url TEXT DEFAULT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    featured_until TIMESTAMPTZ DEFAULT NULL,
    -- Aggregate stats (denormalized for performance)
    total_courses INTEGER NOT NULL DEFAULT 0,
    total_students INTEGER NOT NULL DEFAULT 0,
    total_revenue DECIMAL(14,2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

COMMENT ON TABLE course_bundles IS 'Groups multiple courses into a single purchasable bundle with a discounted price.';
COMMENT ON COLUMN course_bundles.course_ids IS 'Array of Subject IDs included in this bundle.';

-- ============================================================
-- 3. Bundle Enrollments (Student purchases of bundles)
-- ============================================================

CREATE TABLE IF NOT EXISTS bundle_enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "User"(id) ON DELETE CASCADE,
    bundle_id UUID NOT NULL REFERENCES course_bundles(id) ON DELETE CASCADE,
    payment_id UUID DEFAULT NULL,
    price_paid DECIMAL(12,2) NOT NULL DEFAULT 0.00,
    currency VARCHAR(10) NOT NULL DEFAULT 'EGP',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'EXPIRED', 'CANCELLED', 'REFUNDED')),
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    UNIQUE(user_id, bundle_id)
);

COMMENT ON TABLE bundle_enrollments IS 'Tracks which users have purchased which bundles.';

-- ============================================================
-- 4. Bundle-Course Junction (explicit link table)
-- ============================================================

CREATE TABLE IF NOT EXISTS bundle_courses (
    bundle_id UUID NOT NULL REFERENCES course_bundles(id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bundle_id, course_id)
);

COMMENT ON TABLE bundle_courses IS 'Many-to-many junction between bundles and courses. Prefer this over course_ids[] array for referential integrity.';

-- ============================================================
-- 5. Course Versioning
-- ============================================================

CREATE TABLE IF NOT EXISTS course_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES "Subject"(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    version_number INTEGER NOT NULL DEFAULT 1,
    change_summary TEXT DEFAULT NULL,
    change_summary_ar TEXT DEFAULT NULL,
    curriculum_snapshot JSONB DEFAULT '{}',
    created_by UUID DEFAULT NULL REFERENCES "User"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(subject_id, version_number)
);

COMMENT ON TABLE course_versions IS 'Stores immutable snapshots of course curriculum at each version for rollback/history.';

-- ============================================================
-- Indexes
-- ============================================================

-- course_pricing indexes
CREATE INDEX IF NOT EXISTS idx_course_pricing_subject_id ON course_pricing(subject_id);
CREATE INDEX IF NOT EXISTS idx_course_pricing_pricing_type ON course_pricing(pricing_type);
CREATE INDEX IF NOT EXISTS idx_course_pricing_active ON course_pricing(is_active) WHERE is_active = TRUE;

-- course_bundles indexes
CREATE INDEX IF NOT EXISTS idx_course_bundles_is_active ON course_bundles(is_active) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_course_bundles_is_featured ON course_bundles(is_featured) WHERE is_featured = TRUE;
CREATE INDEX IF NOT EXISTS idx_course_bundles_deleted_at ON course_bundles(deleted_at);
CREATE INDEX IF NOT EXISTS idx_course_bundles_created_at ON course_bundles(created_at DESC);

-- bundle_enrollments indexes
CREATE INDEX IF NOT EXISTS idx_bundle_enrollments_user_id ON bundle_enrollments(user_id);
CREATE INDEX IF NOT EXISTS idx_bundle_enrollments_bundle_id ON bundle_enrollments(bundle_id);
CREATE INDEX IF NOT EXISTS idx_bundle_enrollments_status ON bundle_enrollments(status);
CREATE INDEX IF NOT EXISTS idx_bundle_enrollments_expires_at ON bundle_enrollments(expires_at) WHERE expires_at IS NOT NULL;

-- bundle_courses indexes
CREATE INDEX IF NOT EXISTS idx_bundle_courses_course_id ON bundle_courses(course_id);
CREATE INDEX IF NOT EXISTS idx_bundle_courses_bundle_id ON bundle_courses(bundle_id);

-- course_versions indexes
CREATE INDEX IF NOT EXISTS idx_course_versions_subject_id ON course_versions(subject_id);
CREATE INDEX IF NOT EXISTS idx_course_versions_latest ON course_versions(subject_id, version_number DESC);
