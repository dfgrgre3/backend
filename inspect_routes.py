import re
import pathlib
import collections

text = pathlib.Path('d:/backend/internal/router/admin_routes.go').read_text(encoding='utf-8')
routes = []
for line in text.splitlines():
    m = re.search(r'admin\.(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\("([^"]+)"', line)
    if m:
        routes.append((m.group(1), m.group(2)))
counts = collections.Counter(routes)
for route, count in counts.items():
    if count > 1:
        print(route[0], route[1], count)
