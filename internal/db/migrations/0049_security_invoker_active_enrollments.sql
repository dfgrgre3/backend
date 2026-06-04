-- Drop existing view
DROP VIEW IF EXISTS public."ActiveEnrollments";
DROP VIEW IF EXISTS public.ActiveEnrollments;
DROP VIEW IF EXISTS public.activeenrollments;

-- Recreate view with security_invoker = true
CREATE VIEW public."ActiveEnrollments" WITH (security_invoker = true) AS
 SELECT id,
    user_id,
    subject_id,
    created_at AS "createdAt",
    updated_at AS "updatedAt",
    progress,
    deleted_at AS "deletedAt",
    enrolled_at AS "enrolledAt"
   FROM public."SubjectEnrollment"
  WHERE (deleted_at IS NULL);
