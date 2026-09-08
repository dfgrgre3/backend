-- The learning UI exposes one quiz activity per lesson. Keep that invariant
-- in the database as well as in the HTTP handlers.
CREATE UNIQUE INDEX IF NOT EXISTS idx_course_quiz_one_per_lesson
  ON public."CourseQuiz" (course_id, lesson_id)
  WHERE lesson_id IS NOT NULL;
