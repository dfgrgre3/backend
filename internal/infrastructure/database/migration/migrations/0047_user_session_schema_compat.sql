ALTER TABLE IF EXISTS public."UserSession"
    ADD COLUMN IF NOT EXISTS refresh_token_hash text,
    ADD COLUMN IF NOT EXISTS device_type text,
    ADD COLUMN IF NOT EXISTS status text DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS is_active boolean DEFAULT true,
    ADD COLUMN IF NOT EXISTS last_accessed timestamptz,
    ADD COLUMN IF NOT EXISTS expires_at timestamptz,
    ADD COLUMN IF NOT EXISTS revoked_at timestamptz,
    ADD COLUMN IF NOT EXISTS revoked_by uuid,
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

UPDATE public."UserSession"
SET refresh_token_hash = encode(sha256(refresh_token::bytea), 'hex')
WHERE refresh_token_hash IS NULL
  AND refresh_token IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_session_refresh_hash
    ON public."UserSession" (refresh_token_hash)
    WHERE refresh_token_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_session_user_active_recent
    ON public."UserSession" (user_id, is_active, last_accessed DESC)
    WHERE deleted_at IS NULL;
