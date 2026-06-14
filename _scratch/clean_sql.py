import os

def clean_baseline():
    path = r"d:\backend\internal\db\migrations\0000_baseline_schema.sql"
    with open(path, "r", encoding="utf-8") as f:
        content = f.read()

    replacements = {
        ":'DRAFT_STATUS'": "'DRAFT'",
        ":'MEDIUM_VAL'": "'MEDIUM'",
        ":'PENDING_STATUS'": "'PENDING'",
        ":'ACTIVE_STATUS'": "'ACTIVE'",
        ":'INACTIVE_STATUS'": "'INACTIVE'",
        ":'CANCELLED_STATUS'": "'CANCELLED'",
        ":'COMPLETED_STATUS'": "'COMPLETED'",
        ":COURSE_TYPE": "'COURSE'",
        ":'MEDIUM_LOWER'": "'medium'",
        ":'PENDING_LOWER'": "'pending'",
        ":'DRAFT_STATUS_LOWER'": "'draft'",
        ":SOFT_DELETE_COMMENT": "'Soft delete timestamp - NULL means active'"
    }

    for var, val in replacements.items():
        content = content.replace(var, val)

    lines = content.splitlines()
    cleaned_lines = []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("\\set ") or stripped.startswith("\\restrict ") or stripped.startswith("\\unrestrict "):
            continue
        cleaned_lines.append(line)

    with open(path, "w", encoding="utf-8") as f:
        f.write("\n".join(cleaned_lines) + "\n")
    print("Cleaned 0000_baseline_schema.sql")

def clean_check_constraints():
    path = r"d:\backend\internal\db\migrations\0024_add_check_constraints.sql"
    with open(path, "r", encoding="utf-8") as f:
        content = f.read()

    replacements = {
        ":FAILED_STATUS": "'failed'",
        ":CANCELLED_STATUS": "'cancelled'",
        ":PENDING_STATUS": "'pending'",
        ":ACTIVE_STATUS": "'active'",
        ":COMPLETED_STATUS": "'completed'",
        ":DRAFT_STATUS": "'draft'",
        ":RESOLVED_STATUS": "'resolved'",
        ":MONTHLY_INTERVAL": "'monthly'",
        ":WEEKLY_INTERVAL": "'weekly'"
    }

    for var, val in replacements.items():
        content = content.replace(var, val)

    lines = content.splitlines()
    cleaned_lines = []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("\\set ") or stripped.startswith("\\restrict ") or stripped.startswith("\\unrestrict "):
            continue
        cleaned_lines.append(line)

    with open(path, "w", encoding="utf-8") as f:
        f.write("\n".join(cleaned_lines) + "\n")
    print("Cleaned 0024_add_check_constraints.sql")

clean_baseline()
clean_check_constraints()
