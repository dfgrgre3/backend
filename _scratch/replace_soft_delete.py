path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

# Replace :'SOFT_DELETE_COMMENT' with 'Soft delete timestamp - NULL means active'
content = content.replace(":'SOFT_DELETE_COMMENT'", "'Soft delete timestamp - NULL means active'")

with open(path, "w", encoding="utf-8") as f:
    f.write(content)

print("SOFT_DELETE_COMMENT replaced successfully!")
