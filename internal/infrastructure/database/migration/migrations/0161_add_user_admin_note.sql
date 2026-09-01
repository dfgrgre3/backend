-- Migration 0161: Add UserAdminNote table.
--
-- Internal, admin-only notes attached to a user profile (never shown to the
-- student/teacher themselves). Backs the "AdminNotes" tab on the admin
-- users detail page, which was calling /admin/users/:id/notes with no
-- server-side support at all.

CREATE TABLE IF NOT EXISTS "UserAdminNote" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id"       UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "content"       TEXT NOT NULL,
    "created_by"    UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "updated_by"    UUID REFERENCES "User"("id") ON DELETE SET NULL,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "UserAdminNote_user_id_idx" ON "UserAdminNote" ("user_id");
CREATE INDEX IF NOT EXISTS "UserAdminNote_created_at_idx" ON "UserAdminNote" ("created_at");
CREATE INDEX IF NOT EXISTS "UserAdminNote_deleted_at_idx" ON "UserAdminNote" ("deleted_at");
