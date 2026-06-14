path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

import re
# Find all occurrences of : followed by uppercase variable name or mixed case
matches = re.findall(r":\w+", content)
print("Found variables with colon prefix:", set(matches))
