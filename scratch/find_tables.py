with open("d:/backend/internal/db/migrations/0000_baseline_schema.sql", "r", encoding="utf-8") as f:
    content = f.read()

import re
matches = re.findall(r"CREATE TABLE public\.(\"?\w+\"?)", content)
print("Created tables in baseline:", matches)

view_matches = re.findall(r"CREATE VIEW public\.(\w+) AS", content)
print("Created views in baseline:", view_matches)

# Let's search for occurrences of public.users, public."User", public.SubjectEnrollment, public."SubjectEnrollment"
print("Occurrences of public.users:", content.count("public.users"))
print("Occurrences of public.\"User\":", content.count("public.\"User\""))
print("Occurrences of public.SubjectEnrollment:", content.count("public.SubjectEnrollment"))
print("Occurrences of public.\"SubjectEnrollment\":", content.count("public.\"SubjectEnrollment\""))
