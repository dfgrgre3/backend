-- Create login_history table (without FK constraint - users managed by Supabase Auth)
CREATE TABLE IF NOT EXISTS login_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    ip VARCHAR(45),
    user_agent TEXT,
    status VARCHAR(20) NOT NULL,
    reason VARCHAR(255),
    country VARCHAR(100),
    created_at TIMESTAMPTZ(6) DEFAULT NOW()
);

-- Create index for performance
CREATE INDEX IF NOT EXISTS idx_login_history_user_created ON login_history(user_id, created_at DESC);