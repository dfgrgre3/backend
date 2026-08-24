-- Migration 0151: Add LmsCertificateTemplate table.
--
-- A shared, global library of certificate templates a course can pick from
-- (Course.certificate_template already exists as a free-text column that was
-- never actually written by any UI as raw text — it is repurposed here to
-- hold a certificate_template_templates.id instead, no column rename needed).

CREATE TABLE IF NOT EXISTS "LmsCertificateTemplate" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "name"          TEXT NOT NULL,
    "template_html" TEXT NOT NULL,
    "is_default"    BOOLEAN NOT NULL DEFAULT false,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "deleted_at"    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "LmsCertificateTemplate_deleted_at_idx" ON "LmsCertificateTemplate" ("deleted_at");
