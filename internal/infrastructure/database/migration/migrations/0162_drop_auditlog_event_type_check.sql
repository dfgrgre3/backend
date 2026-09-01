-- 0162: Drop the legacy AuditLog event_type CHECK constraint.
--
-- The CHECK added in migration 0021 only allowed the legacy security event
-- vocabulary ('login', 'logout', 'data_modified', ...). The application
-- however writes its own richer vocabulary (auth.login, auth.register,
-- admin.action, user.profile_update, payment.*, exam.*,
-- instructor_document_review, view/create/update/delete, ...), so every
-- audit insert was rejected. The application layer now owns the event
-- vocabulary, so the constraint is removed instead of being kept in sync.

ALTER TABLE public."AuditLog" DROP CONSTRAINT IF EXISTS "AuditLog_event_type_check";
