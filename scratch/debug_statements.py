path = "d:/backend/internal/db/migrations/0000_baseline_schema.sql"
with open(path, "r", encoding="utf-8") as f:
    content = f.read()

# Let's split by semicolon, taking care of comments and quotes
def split_sql(text):
    statements = []
    current = []
    in_quote = False
    quote_char = None
    in_dollar = False
    dollar_tag = ""
    i = 0
    runes = list(text)
    
    # We will do a simple scan
    while i < len(runes):
        ch = runes[i]
        
        # Simple comment handling (skip -- and /* */)
        if not in_quote and not in_dollar:
            if i + 1 < len(runes) and runes[i] == '-' and runes[i+1] == '-':
                # skip line comment
                while i < len(runes) and runes[i] != '\n':
                    i += 1
                continue
            if i + 1 < len(runes) and runes[i] == '/' and runes[i+1] == '*':
                # skip block comment
                i += 2
                while i + 1 < len(runes) and not (runes[i] == '*' and runes[i+1] == '/'):
                    i += 1
                i += 2
                continue
        
        if not in_quote and not in_dollar:
            if ch == "'" or ch == '"':
                in_quote = True
                quote_char = ch
            elif ch == '$':
                # Check dollar quote
                j = i + 1
                while j < len(runes) and (runes[j].isalnum() or runes[j] == '_'):
                    j += 1
                if j < len(runes) and runes[j] == '$':
                    in_dollar = True
                    dollar_tag = "".join(runes[i:j+1])
                    current.append(dollar_tag)
                    i = j + 1
                    continue
            elif ch == ';':
                current.append(';')
                stmt = "".join(current).strip()
                if stmt:
                    statements.append(stmt)
                current = []
                i += 1
                continue
        elif in_quote:
            if ch == quote_char:
                in_quote = False
        elif in_dollar:
            if ch == '$':
                tag = "".join(runes[i:i+len(dollar_tag)])
                if tag == dollar_tag:
                    in_dollar = False
                    current.append(dollar_tag)
                    i += len(dollar_tag)
                    continue
                    
        current.append(ch)
        i += 1
        
    if current:
        stmt = "".join(current).strip()
        if stmt:
            statements.append(stmt)
            
    return statements

stmts = split_sql(content)
print("Total statements parsed:", len(stmts))
print("Statement 45 (index 44):")
print(stmts[44][:300])
print("="*40)
print("Statement 44 (index 43):")
print(stmts[43][:300])

# Let's search where "UserRole" type creation statement is
for idx, stmt in enumerate(stmts):
    if "UserRole" in stmt:
        print(f"Statement {idx+1} (index {idx}) contains 'UserRole':")
        print(stmt[:300])
        print("-"*30)
