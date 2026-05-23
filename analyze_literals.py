import re
from collections import Counter

with open('internal/db/migrations/0024_add_check_constraints.sql', 'r') as f:
    content = f.read()

literals = re.findall(r"'([a-zA-Z_]+)'", content)
counts = Counter(literals)

for lit, count in sorted(counts.items(), key=lambda x: -x[1]):
    if count >= 5:
        print(f"'{lit}': {count} occurrences")