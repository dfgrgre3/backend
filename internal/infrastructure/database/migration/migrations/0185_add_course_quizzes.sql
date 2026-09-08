CREATE TABLE IF NOT EXISTS public."CourseQuiz" (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id uuid NOT NULL REFERENCES public."Subject"(id) ON DELETE CASCADE,
    lesson_id uuid REFERENCES public."SubTopic"(id) ON DELETE SET NULL,
    title text NOT NULL,
    description text,
    instructions text,
    time_limit_minutes integer,
    passing_score numeric NOT NULL DEFAULT 60,
    max_attempts integer NOT NULL DEFAULT 1,
    shuffle_questions boolean NOT NULL DEFAULT false,
    shuffle_options boolean NOT NULL DEFAULT false,
    show_results_immediately boolean NOT NULL DEFAULT true,
    show_correct_answers boolean NOT NULL DEFAULT false,
    allow_review boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'draft',
    questions jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by uuid NOT NULL REFERENCES public."User"(id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_course_quiz_course_id ON public."CourseQuiz"(course_id);
CREATE INDEX IF NOT EXISTS idx_course_quiz_lesson_id ON public."CourseQuiz"(lesson_id);

CREATE TABLE IF NOT EXISTS public."CourseQuizAttempt" (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id uuid NOT NULL REFERENCES public."CourseQuiz"(id) ON DELETE CASCADE,
    course_id uuid NOT NULL REFERENCES public."Subject"(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES public."User"(id) ON DELETE CASCADE,
    answers jsonb NOT NULL DEFAULT '[]'::jsonb,
    score numeric NOT NULL DEFAULT 0,
    max_score numeric NOT NULL DEFAULT 0,
    percentage numeric NOT NULL DEFAULT 0,
    passed boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'graded',
    time_spent_seconds integer NOT NULL DEFAULT 0,
    started_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    submitted_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    graded_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_course_quiz_attempt_quiz_user ON public."CourseQuizAttempt"(quiz_id, user_id);
ALTER TABLE public."CourseQuiz" DISABLE ROW LEVEL SECURITY;
ALTER TABLE public."CourseQuizAttempt" DISABLE ROW LEVEL SECURITY;
