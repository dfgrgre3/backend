path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

# 1. Replace CREATE TABLE public.users with CREATE TABLE public."User"
content = content.replace("CREATE TABLE public.users (", "CREATE TABLE public.\"User\" (")

# 2. Replace FROM public.SubjectEnrollment with FROM public."SubjectEnrollment"
content = content.replace("FROM public.SubjectEnrollment", "FROM public.\"SubjectEnrollment\"")

with open(path, "w", encoding="utf-8") as f:
    f.write(content)

print("Baseline schema table names fixed successfully!")
