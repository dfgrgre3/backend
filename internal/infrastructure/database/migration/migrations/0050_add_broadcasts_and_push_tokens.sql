-- Migration: 0050_add_broadcasts_and_push_tokens.sql
-- Description: Create broadcasts and push_tokens tables for notification system

CREATE TABLE IF NOT EXISTS public."broadcasts" (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title varchar(200) NOT NULL,
    message text NOT NULL,
    type varchar(20) DEFAULT 'info',
    channels jsonb DEFAULT '[]',
    target_count integer DEFAULT 0,
    success_count integer DEFAULT 0,
    failure_count integer DEFAULT 0,
    status varchar(20) DEFAULT 'draft',
    scheduled_for timestamp with time zone,
    sent_at timestamp with time zone,
    created_by uuid NOT NULL,
    cancelled_by uuid,
    cancelled_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS public."push_tokens" (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    token varchar(500) NOT NULL,
    platform varchar(20) NOT NULL,
    provider varchar(20) NOT NULL,
    is_active boolean DEFAULT true,
    last_used timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone
);

CREATE INDEX IF NOT EXISTS idx_broadcasts_deleted_at ON public."broadcasts"(deleted_at);
CREATE INDEX IF NOT EXISTS idx_push_tokens_user_id ON public."push_tokens"(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_push_tokens_token ON public."push_tokens"(token);
