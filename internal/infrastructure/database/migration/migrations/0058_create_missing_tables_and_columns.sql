-- Migration to create missing tables and columns to align GORM models with database schema

-- 1. Alter existing tables to add missing columns
ALTER TABLE "ContentReport" ADD COLUMN IF NOT EXISTS resolved_by UUID;
ALTER TABLE "ContentReport" ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;

ALTER TABLE "Contest" ADD COLUMN IF NOT EXISTS questions_count INTEGER DEFAULT 0;
ALTER TABLE "Contest" ADD COLUMN IF NOT EXISTS participants_count INTEGER DEFAULT 0;

ALTER TABLE "ContestQuestion" ADD COLUMN IF NOT EXISTS "order" INTEGER DEFAULT 0;

DO $$
BEGIN
    IF to_regclass('public."IpWhitelistEntry"') IS NOT NULL THEN
        ALTER TABLE public."IpWhitelistEntry" ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
        ALTER TABLE public."IpWhitelistEntry" ADD COLUMN IF NOT EXISTS country VARCHAR(100);
        ALTER TABLE public."IpWhitelistEntry" ADD COLUMN IF NOT EXISTS city VARCHAR(100);
    ELSIF to_regclass('public."ip_whitelist_entries"') IS NOT NULL THEN
        ALTER TABLE public."ip_whitelist_entries" ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
        ALTER TABLE public."ip_whitelist_entries" ADD COLUMN IF NOT EXISTS country VARCHAR(100);
        ALTER TABLE public."ip_whitelist_entries" ADD COLUMN IF NOT EXISTS city VARCHAR(100);
    END IF;
END $$;

-- 2. Drop and recreate AnalyticsEvent table to match the new structure
DROP TABLE IF EXISTS "AnalyticsEvent" CASCADE;
CREATE TABLE "AnalyticsEvent" (
    id UUID PRIMARY KEY,
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    user_id UUID,
    payload JSONB NOT NULL DEFAULT '{}',
    source VARCHAR(50) DEFAULT 'frontend',
    ip_address VARCHAR(50),
    user_agent TEXT,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_analytics_event_id ON "AnalyticsEvent"(event_id);
CREATE INDEX IF NOT EXISTS idx_analytics_event_type ON "AnalyticsEvent"(event_type);
CREATE INDEX IF NOT EXISTS idx_analytics_event_user ON "AnalyticsEvent"(user_id);
CREATE INDEX IF NOT EXISTS idx_analytics_event_received ON "AnalyticsEvent"(received_at);
CREATE INDEX IF NOT EXISTS idx_analytics_event_processed ON "AnalyticsEvent"(processed_at);

-- 3. Create missing tables
CREATE TABLE IF NOT EXISTS "AIConversation" (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    subject_id UUID,
    topic_id UUID,
    title TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS "AIMessage" (
    id UUID NOT NULL,
    conversation_id UUID NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    model TEXT,
    tokens_used INTEGER,
    latency BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    PRIMARY KEY (id, created_at)
);

CREATE TABLE IF NOT EXISTS "ticket_messages" (
    id UUID PRIMARY KEY,
    ticket_id UUID NOT NULL,
    sender_id UUID NOT NULL,
    sender_name VARCHAR(200) NOT NULL,
    sender_role VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    attachments JSONB,
    is_internal BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS "user_journey_steps" (
    id UUID PRIMARY KEY,
    journey_id UUID NOT NULL,
    user_id UUID NOT NULL,
    session_id VARCHAR(100) NOT NULL,
    page VARCHAR(500) NOT NULL,
    action VARCHAR(100) NOT NULL,
    metadata JSONB,
    timestamp TIMESTAMPTZ NOT NULL,
    duration BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS "scheduled_items" (
    id UUID PRIMARY KEY,
    type VARCHAR(20) NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    content JSONB,
    scheduled_for TIMESTAMPTZ NOT NULL,
    timezone VARCHAR(50) DEFAULT 'UTC',
    frequency VARCHAR(20) DEFAULT 'once',
    status VARCHAR(20) DEFAULT 'pending',
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    error TEXT,
    executed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS "backups" (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    size BIGINT,
    status VARCHAR(20) DEFAULT 'pending',
    checksum VARCHAR(64),
    download_url VARCHAR(500),
    includes_files BOOLEAN DEFAULT FALSE,
    includes_database BOOLEAN DEFAULT FALSE,
    tables TEXT[],
    retention_days INTEGER DEFAULT 30,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    error TEXT,
    restored_at TIMESTAMPTZ,
    restored_by UUID,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS "user_flows" (
    id UUID PRIMARY KEY,
    from_page VARCHAR(500) NOT NULL,
    to_page VARCHAR(500) NOT NULL,
    date DATE NOT NULL,
    count BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_flow ON user_flows(from_page, to_page, date);

CREATE TABLE IF NOT EXISTS "Campaign" (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) DEFAULT 'email',
    status VARCHAR(50) DEFAULT 'DRAFT',
    target_role VARCHAR(50),
    content TEXT,
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    sent_count INTEGER DEFAULT 0,
    open_count INTEGER DEFAULT 0,
    click_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS "support_tickets" (
    id UUID PRIMARY KEY,
    ticket_number VARCHAR(20) NOT NULL,
    user_id UUID NOT NULL,
    user_name VARCHAR(200),
    user_email VARCHAR(255),
    subject VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'open',
    priority VARCHAR(10) DEFAULT 'medium',
    assigned_to UUID,
    assigned_to_name VARCHAR(200),
    tags TEXT[],
    related_entity_type VARCHAR(50),
    related_entity_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    satisfaction_rating INTEGER,
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_tickets_number ON support_tickets(ticket_number);

CREATE TABLE IF NOT EXISTS "user_journeys" (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    session_id VARCHAR(100) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    total_duration BIGINT,
    conversion_goal VARCHAR(100),
    completed BOOLEAN DEFAULT FALSE,
    device_info VARCHAR(500),
    ip_address VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS "page_views" (
    id UUID PRIMARY KEY,
    page VARCHAR(500) NOT NULL,
    date DATE NOT NULL,
    views BIGINT DEFAULT 0,
    unique_visitors BIGINT DEFAULT 0,
    avg_duration DOUBLE PRECISION,
    bounces BIGINT DEFAULT 0,
    exits BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_page_date ON page_views(page, date);

CREATE TABLE IF NOT EXISTS "conversion_events" (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    session_id VARCHAR(100) NOT NULL,
    journey_id UUID,
    goal VARCHAR(100) NOT NULL,
    value DOUBLE PRECISION,
    currency VARCHAR(3),
    timestamp TIMESTAMPTZ NOT NULL,
    journey_steps INTEGER,
    source VARCHAR(100),
    campaign_id UUID,
    metadata JSONB,
    ip_address VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS "active_user_stats" (
    id UUID PRIMARY KEY,
    date DATE NOT NULL,
    daily_active BIGINT DEFAULT 0,
    weekly_active BIGINT DEFAULT 0,
    monthly_active BIGINT DEFAULT 0,
    new_users BIGINT DEFAULT 0,
    returning_users BIGINT DEFAULT 0,
    avg_session_duration DOUBLE PRECISION,
    total_sessions BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_active_user_stats_date ON active_user_stats(date);

CREATE TABLE IF NOT EXISTS "custom_reports" (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    widgets JSONB,
    filters JSONB,
    date_range_from TIMESTAMPTZ,
    date_range_to TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    is_public BOOLEAN DEFAULT FALSE,
    last_run_at TIMESTAMPTZ,
    schedule_frequency VARCHAR(20),
    schedule_email_to TEXT[]
);
