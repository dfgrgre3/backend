path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

# Locate and remove set_config('search_path' lines
lines = content.splitlines()
cleaned_lines = []
for line in lines:
    if "set_config('search_path'" in line:
        print("Removing line:", line)
        continue
    cleaned_lines.append(line)

with open(path, "w", encoding="utf-8") as f:
    f.write("\n".join(cleaned_lines) + "\n")

print("Removed search_path configurations!")
