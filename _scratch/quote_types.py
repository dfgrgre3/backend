path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

types_list = [
    "AchievementCategory",
    "AddonType",
    "CategoryType",
    "ContestStatus",
    "Difficulty",
    "DiscountType",
    "FocusStrategy",
    "InvoiceStatus",
    "LessonType",
    "Level",
    "NotificationType",
    "PaymentStatus",
    "PlanInterval",
    "SubscriptionStatus",
    "TaskStatus",
    "UserRole",
    "UserStatus",
    "WalletTransactionStatus",
    "WalletTransactionType"
]

for t in types_list:
    # Replace CREATE TYPE public.TypeName with CREATE TYPE public."TypeName"
    old_str = f"CREATE TYPE public.{t} AS ENUM"
    new_str = f"CREATE TYPE public.\"{t}\" AS ENUM"
    content = content.replace(old_str, new_str)
    
    # Also replace TYPE public.TypeName with TYPE public."TypeName" in comments or other references if any
    old_cmt = f"Type: TYPE; Schema: public; Owner: -"
    # (Comments are fine, but let's be safe)

with open(path, "w", encoding="utf-8") as f:
    f.write(content)

print("Baseline schema custom types successfully quoted!")
