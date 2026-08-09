-- Migration 0142: Admin dashboard operational tables
-- Adds persistence for operational alerts, per-user saved filters, and
-- asynchronous dashboard export jobs. All three are consumed by the
-- /api/admin/dashboard/* endpoints.

-- ── Operational alerts ───────────────────────
CREATE TABLE IF NOT EXISTS "DashboardAlert" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    severity TEXT NOT NULL DEFAULT 'info',
    category TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    source TEXT NOT NULL DEFAULT 'system',
    dedupe_key TEXT,
    occurrence_count BIGINT NOT NULL DEFAULT 1,
    state TEXT NOT NULL DEFAULT 'open',
    related_entity_type TEXT,
    related_entity_id TEXT,
    action_url TEXT,
    required_permission TEXT,
    acknowledged_by UUID,
    acknowledged_at TIMESTAMPTZ,
    acknowledge_note TEXT,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_dashboard_alert_severity
        CHECK (severity IN ('info', 'warning', 'error', 'critical')),
    CONSTRAINT chk_dashboard_alert_state
        CHECK (state IN ('open', 'acknowledged', 'resolved', 'suppressed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_alert_dedupe
    ON "DashboardAlert" (dedupe_key) WHERE dedupe_key IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_dashboard_alert_state_seen
    ON "DashboardAlert" (state, last_seen_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_dashboard_alert_severity
    ON "DashboardAlert" (severity, last_seen_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_dashboard_alert_category
    ON "DashboardAlert" (category) WHERE deleted_at IS NULL;

ALTER TABLE "DashboardAlert" DISABLE ROW LEVEL SECURITY;

-- ── Saved filters ────────────────────────────
CREATE TABLE IF NOT EXISTS "DashboardSavedFilter" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    filter_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT false,
    visibility TEXT NOT NULL DEFAULT 'private',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_dashboard_saved_filter_visibility
        CHECK (visibility IN ('private', 'shared')),
    CONSTRAINT fk_dashboard_saved_filter_user
        FOREIGN KEY (user_id) REFERENCES "User"(id) ON DELETE CASCADE
);

-- Name uniqueness is per owner and ignores soft-deleted rows.
CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_saved_filter_user_name
    ON "DashboardSavedFilter" (user_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_dashboard_saved_filter_user
    ON "DashboardSavedFilter" (user_id, created_at DESC) WHERE deleted_at IS NULL;

ALTER TABLE "DashboardSavedFilter" DISABLE ROW LEVEL SECURITY;

-- ── Export jobs ──────────────────────────────
CREATE TABLE IF NOT EXISTS "DashboardExportJob" (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    export_scope TEXT NOT NULL,
    file_format TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    progress INT NOT NULL DEFAULT 0,
    filters JSONB,
    include_charts_metadata BOOLEAN NOT NULL DEFAULT false,
    include_sensitive_fields BOOLEAN NOT NULL DEFAULT false,
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    row_count BIGINT NOT NULL DEFAULT 0,
    file_url TEXT,
    error_code TEXT,
    expires_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_dashboard_export_format
        CHECK (file_format IN ('csv', 'xlsx', 'pdf')),
    CONSTRAINT chk_dashboard_export_status
        CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'expired')),
    CONSTRAINT chk_dashboard_export_progress
        CHECK (progress BETWEEN 0 AND 100),
    CONSTRAINT fk_dashboard_export_user
        FOREIGN KEY (user_id) REFERENCES "User"(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_dashboard_export_user_created
    ON "DashboardExportJob" (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dashboard_export_status
    ON "DashboardExportJob" (status, created_at DESC);

ALTER TABLE "DashboardExportJob" DISABLE ROW LEVEL SECURITY;
