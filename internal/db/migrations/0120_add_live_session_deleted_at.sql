ALTER TABLE IF EXISTS "LiveSession"
    ADD COLUMN IF NOT EXISTS "deleted_at" TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS "LiveSession_deleted_at_idx"
    ON "LiveSession" ("deleted_at");
