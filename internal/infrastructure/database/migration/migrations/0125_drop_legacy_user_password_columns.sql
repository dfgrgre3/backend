-- Drop password fields retained on the user profile after credentials were migrated.
ALTER TABLE public."User"
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS password_changed_at,
    DROP COLUMN IF EXISTS password_expires_at;
