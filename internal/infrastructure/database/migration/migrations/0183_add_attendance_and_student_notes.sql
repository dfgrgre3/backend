-- 0183: Add Attendance and StudentNote tables
--
-- Attendance: per-user, per-subject attendance records (for live/offline
-- sessions). StudentNote: free-text notes an admin/teacher attaches to a
-- student's profile (not visible to the student).

CREATE TABLE public."Attendance" (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    subject_id uuid,
    session_date timestamp with time zone NOT NULL,
    status text DEFAULT 'present'::text NOT NULL,
    notes text,
    recorded_by uuid,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    CONSTRAINT "Attendance_pkey" PRIMARY KEY (id),
    CONSTRAINT "Attendance_user_id_fkey" FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE CASCADE,
    CONSTRAINT "Attendance_subject_id_fkey" FOREIGN KEY (subject_id) REFERENCES public."Subject"(id) ON DELETE SET NULL,
    CONSTRAINT "Attendance_recorded_by_fkey" FOREIGN KEY (recorded_by) REFERENCES public."User"(id) ON DELETE SET NULL,
    CONSTRAINT chk_attendance_status_valid CHECK ((status = ANY (ARRAY['present'::text, 'absent'::text, 'late'::text, 'excused'::text])))
);

CREATE INDEX idx_attendance_user_id ON public."Attendance" (user_id);
CREATE INDEX idx_attendance_subject_id ON public."Attendance" (subject_id);
CREATE INDEX idx_attendance_session_date ON public."Attendance" (session_date);

ALTER TABLE public."Attendance" DISABLE ROW LEVEL SECURITY;

CREATE TABLE public."StudentNote" (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    author_id uuid,
    category text DEFAULT 'general'::text NOT NULL,
    note text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    CONSTRAINT "StudentNote_pkey" PRIMARY KEY (id),
    CONSTRAINT "StudentNote_user_id_fkey" FOREIGN KEY (user_id) REFERENCES public."User"(id) ON DELETE CASCADE,
    CONSTRAINT "StudentNote_author_id_fkey" FOREIGN KEY (author_id) REFERENCES public."User"(id) ON DELETE SET NULL
);

CREATE INDEX idx_student_note_user_id ON public."StudentNote" (user_id);
CREATE INDEX idx_student_note_created_at ON public."StudentNote" (created_at);

ALTER TABLE public."StudentNote" DISABLE ROW LEVEL SECURITY;
