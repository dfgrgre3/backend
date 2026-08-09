-- Migration: 0062_add_parent_support_roles.sql
-- Description: Add PARENT and SUPPORT to UserRole enum, and create student_parents mapping table

-- We need to add PARENT and SUPPORT values to the existing UserRole enum.
ALTER TYPE public."UserRole" ADD VALUE IF NOT EXISTS 'PARENT';
ALTER TYPE public."UserRole" ADD VALUE IF NOT EXISTS 'SUPPORT';

-- Create student_parents relationship table for Parent-Student linkage
CREATE TABLE IF NOT EXISTS public.student_parents (
    student_id uuid NOT NULL,
    parent_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_student_parents PRIMARY KEY (student_id, parent_id),
    CONSTRAINT fk_student_parents_student FOREIGN KEY (student_id) REFERENCES public."User"(id) ON DELETE CASCADE,
    CONSTRAINT fk_student_parents_parent FOREIGN KEY (parent_id) REFERENCES public."User"(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_student_parents_parent_id ON public.student_parents(parent_id);
