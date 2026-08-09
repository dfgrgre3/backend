-- Mail queue schema is managed by the migration process, never by a runtime worker.
CREATE TABLE IF NOT EXISTS "MailTask" (
    "id" BIGSERIAL PRIMARY KEY,
    "to" VARCHAR(255),
    "subject" VARCHAR(255),
    "body" TEXT,
    "status" VARCHAR(50) NOT NULL DEFAULT 'pending',
    "attempts" INTEGER NOT NULL DEFAULT 0,
    "max_retry" INTEGER NOT NULL DEFAULT 3,
    "last_error" TEXT,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "deleted_at" TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS "idx_mail_task_to" ON "MailTask" ("to");
CREATE INDEX IF NOT EXISTS "idx_mail_task_status" ON "MailTask" ("status");
CREATE INDEX IF NOT EXISTS "idx_mail_task_deleted_at" ON "MailTask" ("deleted_at");