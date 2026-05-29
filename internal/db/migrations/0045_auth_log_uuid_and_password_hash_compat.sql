DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'User'
          AND column_name = 'passwordHash'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'User'
          AND column_name = 'password_hash'
    ) THEN
        ALTER TABLE public."User" RENAME COLUMN "passwordHash" TO password_hash;
    ELSIF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'User'
          AND column_name = 'passwordHash'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'User'
          AND column_name = 'password_hash'
    ) THEN
        UPDATE public."User"
        SET password_hash = COALESCE(NULLIF(password_hash, ''), "passwordHash")
        WHERE "passwordHash" IS NOT NULL;

        ALTER TABLE public."User" DROP COLUMN "passwordHash";
    END IF;
END $$;

ALTER TABLE IF EXISTS public."SecurityLog"
    ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE IF EXISTS public."AuditLog"
    ALTER COLUMN user_id DROP NOT NULL;
