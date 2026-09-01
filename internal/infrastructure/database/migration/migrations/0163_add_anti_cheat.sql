-- Migration 0163: Add Anti-Cheat tables.
--
-- Backs the admin "مكافحة الغش" (anti-cheat) monitoring page. Two tables:
--   * AntiCheatEvent  — raw proctoring events reported by the exam client
--                       (tab switch, blur, copy/paste, screenshots, ...).
--   * AntiCheatFlag   — aggregated review case per (user, exam, attempt):
--                       risk score, status, evidence, review outcome.

-- ─────────────────────────────────────────────
-- Raw proctoring events
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS "AntiCheatEvent" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id"       UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "exam_id"       UUID REFERENCES "Exam"("id") ON DELETE CASCADE,
    "attempt_id"    UUID,
    "event_type"    TEXT NOT NULL,
    "severity"      TEXT NOT NULL DEFAULT 'LOW',
    "detail"        TEXT,
    "metadata"      JSONB,
    "ip_address"    TEXT,
    "user_agent"    TEXT,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Aggregated anti-cheat flags / review cases
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS "AntiCheatFlag" (
    "id"            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id"       UUID NOT NULL REFERENCES "User"("id") ON DELETE CASCADE,
    "exam_id"       UUID REFERENCES "Exam"("id") ON DELETE CASCADE,
    "attempt_id"    UUID,
    "risk_score"    INTEGER NOT NULL DEFAULT 0,
    "status"        TEXT NOT NULL DEFAULT 'OPEN',
    "reason"        TEXT,
    "evidence"      JSONB,
    "event_count"   INTEGER NOT NULL DEFAULT 0,
    "ip_address"    TEXT,
    "reviewer_id"   UUID REFERENCES "User"("id") ON DELETE SET NULL,
    "reviewed_at"   TIMESTAMPTZ,
    "review_note"   TEXT,
    "created_at"    TIMESTAMPTZ NOT NULL DEFAULT now(),
    "updated_at"    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS "AntiCheatEvent_user_created_idx" ON "AntiCheatEvent" ("user_id", "created_at");
CREATE INDEX IF NOT EXISTS "AntiCheatEvent_exam_created_idx" ON "AntiCheatEvent" ("exam_id", "created_at");
CREATE INDEX IF NOT EXISTS "AntiCheatEvent_type_idx" ON "AntiCheatEvent" ("event_type");
CREATE INDEX IF NOT EXISTS "AntiCheatEvent_severity_idx" ON "AntiCheatEvent" ("severity");
CREATE INDEX IF NOT EXISTS "AntiCheatEvent_created_at_idx" ON "AntiCheatEvent" ("created_at");

CREATE INDEX IF NOT EXISTS "AntiCheatFlag_user_created_idx" ON "AntiCheatFlag" ("user_id", "created_at");
CREATE INDEX IF NOT EXISTS "AntiCheatFlag_exam_idx" ON "AntiCheatFlag" ("exam_id");
CREATE INDEX IF NOT EXISTS "AntiCheatFlag_status_idx" ON "AntiCheatFlag" ("status");
CREATE INDEX IF NOT EXISTS "AntiCheatFlag_risk_idx" ON "AntiCheatFlag" ("risk_score");
CREATE INDEX IF NOT EXISTS "AntiCheatFlag_created_at_idx" ON "AntiCheatFlag" ("created_at");
