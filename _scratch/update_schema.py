import re

SCHEMA_PATH = r"d:\backend\prisma\schema.prisma"
ADDITIONS_PATH = r"d:\backend\_scratch\auth_schema_additions.prisma"

with open(SCHEMA_PATH, 'r', encoding='utf-8') as f:
    schema = f.read()

with open(ADDITIONS_PATH, 'r', encoding='utf-8') as f:
    additions = f.read()

# Append relations to the User model
user_relations = """
  UserRole           UserRole[]
  AuthSession        AuthSession[]
  OAuthAccount       OAuthAccount[]
  VerificationCode   VerificationCode[]
  LoginHistory       LoginHistory[]
"""

# Find the end of the User model and insert relations before the last closing brace
user_model_pattern = r"(model User\s*\{.*?\n)(\s*@@index.*?\n\})"
match = re.search(user_model_pattern, schema, re.DOTALL)
if match:
    new_schema = schema[:match.end(1)] + user_relations + match.group(2) + schema[match.end(2):]
else:
    # If not found using that pattern, find just the end of the model
    user_end_pattern = r"(model User\s*\{.*?)(\n\})"
    match = re.search(user_end_pattern, schema, re.DOTALL)
    if match:
        new_schema = schema[:match.end(1)] + "\n" + user_relations + match.group(2) + schema[match.end(2):]
    else:
        print("User model not found")
        exit(1)

new_schema += "\n\n" + additions

with open(SCHEMA_PATH, 'w', encoding='utf-8') as f:
    f.write(new_schema)

print("Schema updated successfully.")
