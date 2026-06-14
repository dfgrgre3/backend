path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

import re
matches = re.findall(r".*SOFT_DELETE.*", content)
print("Occurrences of SOFT_DELETE:")
for m in matches:
    print(m)
