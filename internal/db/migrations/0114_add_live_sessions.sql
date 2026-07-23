CREATE TABLE IF NOT EXISTS "LiveSession" (
    "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "subject_id" UUID,
    "title" TEXT NOT NULL,
    "description" TEXT,
    "provider" TEXT NOT NULL DEFAULT 'ZOOM',
    "join_url" TEXT,
    "start_url" TEXT,
    "host_email" TEXT NOT NULL DEFAULT '',
    "scheduled_at" TIMESTAMPTZ NOT NULL,
    "duration_min" INTEGER NOT NULL DEFAULT 60 CHECK ("duration_min" > 0),
    "status" TEXT NOT NULL DEFAULT 'SCHEDULED' CHECK ("status" IN ('SCHEDULED', 'LIVE', 'ENDED', 'CANCELLED')),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS "LiveSession_subject_id_idx" ON "LiveSession" ("subject_id");
CREATE INDEX IF NOT EXISTS "LiveSession_scheduled_at_idx" ON "LiveSession" ("scheduled_at");
CREATE INDEX IF NOT EXISTS "LiveSession_status_idx" ON "LiveSession" ("status");
