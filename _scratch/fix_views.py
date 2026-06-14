path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

# Define the new view for ActiveEnrollments
new_active_enrollments = """CREATE VIEW public.ActiveEnrollments AS
 SELECT id,
    user_id,
    subject_id,
    created_at AS "createdAt",
    updated_at AS "updatedAt",
    progress,
    deleted_at AS "deletedAt",
    enrolled_at AS "enrolledAt"
   FROM public."SubjectEnrollment"
  WHERE (deleted_at IS NULL);"""

# Define the new view for ActiveUsers
new_active_users = """CREATE VIEW public.ActiveUsers AS
 SELECT id,
    email,
    name,
    username,
    avatar,
    created_at,
    updated_at,
    wake_up_time,
    sleep_time,
    focus_strategy,
    email_notifications,
    email_verification_token,
    email_verification_expires,
    email_verified,
    last_login,
    phone,
    phone_verified,
    phone_verification_otp,
    phone_verification_expires,
    phone_verification_attempts,
    phone_verification_last_sent,
    reset_token,
    reset_token_expires,
    recovery_codes,
    sms_notifications,
    biometric_enabled,
    magic_link_token,
    magic_link_expires,
    google_id,
    github_id,
    password_changed_at,
    password_expires_at,
    password_expiration_warning_sent,
    role,
    status,
    country,
    date_of_birth,
    gender,
    alternative_phone,
    section,
    interested_subjects,
    study_goal,
    subjects_taught,
    classes_taught,
    experience_years,
    bio,
    permissions,
    school,
    referral_code,
    referred_by_id,
    additional_ai_credits,
    additional_exam_credits,
    is_deleted,
    last_usage_reset,
    monthly_ai_message_count,
    monthly_exam_count,
    total_xp,
    level,
    balance,
    ai_credits,
    exam_credits,
    reset_password_token,
    reset_password_expires,
    verification_token,
    verification_expires,
    deleted_at,
    current_streak,
    longest_streak,
    total_study_time,
    tasks_completed,
    exams_passed,
    study_xp,
    task_xp,
    exam_xp,
    challenge_xp,
    quest_xp,
    season_xp,
    "archiveReason",
    password_hash
   FROM public."User"
  WHERE (deleted_at IS NULL);"""

# Locate and replace ActiveEnrollments view in content
# The original view spans from line 427: "CREATE VIEW public.ActiveEnrollments AS" to line 441: "  WHERE (deleted_at IS NULL);"
import re

enrollments_pattern = r"CREATE VIEW public\.ActiveEnrollments AS.*?WHERE \(deleted_at IS NULL\);"
content = re.sub(enrollments_pattern, new_active_enrollments, content, flags=re.DOTALL)

# Locate and replace ActiveUsers view in content
# The original view spans from "CREATE VIEW public.ActiveUsers AS" to "WHERE (deleted_at IS NULL);"
users_pattern = r"CREATE VIEW public\.ActiveUsers AS.*?WHERE \(deleted_at IS NULL\);"
content = re.sub(users_pattern, new_active_users, content, flags=re.DOTALL)

with open(path, "w", encoding="utf-8") as f:
    f.write(content)

print("Views updated successfully in 0000_baseline_schema.sql!")
