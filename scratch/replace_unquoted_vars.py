path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

# Replace :MEDIUM_LOWER with 'medium'
content = content.replace(":MEDIUM_LOWER", "'medium'")

# Replace :PENDING_LOWER with 'pending'
content = content.replace(":PENDING_LOWER", "'pending'")

with open(path, "w", encoding="utf-8") as f:
    f.write(content)

print("Unquoted variables replaced successfully!")
