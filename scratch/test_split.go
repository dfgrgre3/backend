//go:build ignore

package main

import (
	"fmt"
	"strings"
)

func main() {
	// Read the migration file
	contents := `-- ==========================================
-- كود #2 (النسخة المحسنة): Full-Text Search for Subjects
-- ==========================================

-- 1. إضافة عمود البحث مع قيد NOT NULL وقيمة افتراضية
ALTER TABLE "Subject" 
    ADD COLUMN IF NOT EXISTS search_vector tsvector NOT NULL DEFAULT ''::tsvector;

COMMENT ON COLUMN "Subject".search_vector IS 'TSVector محسوب تلقائياً للبحث النصي الكامل - يدعم العربية والإنجليزية مع أوزان أولوية';

-- 2. إنشاء فهرس GIN جزئي وآمن للإنتاج
-- CONCURRENTLY لا يقفل الجدول (يتطلب عدم وجود transaction مفتوح)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subject_search_vector 
    ON "Subject" USING GIN(search_vector)
    WHERE deleted_at IS NULL; -- استبعاد السجلات المحذوفة ناعماً

-- 3. دالة محسنة مع أوزان ودعم للنصوص الطويلة
CREATE OR REPLACE FUNCTION update_subject_search_vector()
RETURNS trigger AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.code, ''), 100)), 'A') ||
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.name, ''), 500)), 'B') ||
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.name_ar, ''), 500)), 'B') ||
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.description, ''), 2000)), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION update_subject_search_vector() IS 'تحديث search_vector مع أوزان: Code(A), Name/NameAr(B), Description(D)';

-- 4. Trigger محدد النطاق
DROP TRIGGER IF EXISTS trg_subject_search_vector ON "Subject";
CREATE TRIGGER trg_subject_search_vector
    BEFORE INSERT OR UPDATE OF name, name_ar, description, code
    ON "Subject"
    FOR EACH ROW
    EXECUTE FUNCTION update_subject_search_vector();

-- 5. تعبئة البيانات الموجودة بشكل آمن (Batch-Safe)
-- يُنفذ فقط للسجلات التي لم تُحسب بعد أو كانت NULL سابقاً
UPDATE "Subject"
SET search_vector = 
    setweight(to_tsvector('simple', LEFT(COALESCE(code, ''), 100)), 'A') ||
    setweight(to_tsvector('simple', LEFT(COALESCE(name, ''), 500)), 'B') ||
    setweight(to_tsvector('simple', LEFT(COALESCE(name_ar, ''), 500)), 'B') ||
    setweight(to_tsvector('simple', LEFT(COALESCE(description, ''), 2000)), 'D')
WHERE search_vector = ''::tsvector 
   OR search_vector IS NULL;`

	statements := splitSQLStatements(contents)
	fmt.Printf("Total statements: %d\n\n", len(statements))

	// Show ALL statements including empty ones
	for i, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		// Check first 100 chars
		preview := trimmed
		if len(preview) > 100 {
			preview = preview[:100]
		}
		fmt.Printf("Statement %d (len=%d): %.100s\n", i+1, len(stmt), preview)
	}
}

func splitSQLStatements(contents string) []string {
	var statements []string
	var current strings.Builder
	var inDollarQuote bool
	var dollarQuoteTag string

	// Helper to remove single-line comments and check for semicolon
	hasTerminatingSemicolon := func(s string) bool {
		// Find the position of semicolon
		for i := 0; i < len(s); i++ {
			if s[i] == ';' {
				// Check if everything after semicolon is comment/whitespace
				rest := strings.TrimSpace(s[i+1:])
				if rest == "" || strings.HasPrefix(rest, "--") {
					return true
				}
			}
		}
		return false
	}

	lines := strings.Split(contents, "\n")
	for _, line := range lines {
		// Process the line character by character to properly handle dollar quotes
		i := 0
		for i < len(line) {
			ch := line[i]

			// Handle dollar quotes - $tag$ or $$
			if ch == '$' {
				if !inDollarQuote {
					// Not in a quote, look for opening tag
					j := i + 1
					var tag string
					// Check for $$ (empty tag)
					if j < len(line) && line[j] == '$' {
						tag = "$$"
						j++
					} else {
						// Find a named tag like $tag$
						for j < len(line) && (line[j] == '_' || (line[j] >= 'a' && line[j] <= 'z') || (line[j] >= 'A' && line[j] <= 'Z') || (line[j] >= '0' && line[j] <= '9')) {
							j++
						}
						if j < len(line) && line[j] == '$' {
							tag = line[i : j+1]
						} else {
							// Not a dollar quote, just write the '$' and continue
							current.WriteByte(ch)
							i++
							continue
						}
					}

					// Opening the quote
					current.WriteString(tag)
					dollarQuoteTag = tag
					inDollarQuote = true
					i = j
					continue
				} else {
					// In a quote, check for closing tag
					j := i + 1
					// $$ closes an empty-tag quote
					if dollarQuoteTag == "$$" && j < len(line) && line[j] == '$' {
						// $$ closes the current empty-tag quote
						current.WriteString("$$")
						inDollarQuote = false
						dollarQuoteTag = ""
						i = j + 1
						continue
					} else if dollarQuoteTag != "$$" {
						// For named tags, look for the matching closing tag
						if j < len(line) && line[j] == '$' {
							tag := line[i : j+1]
							if tag == dollarQuoteTag {
								current.WriteString(tag)
								inDollarQuote = false
								dollarQuoteTag = ""
								i = j + 1
								continue
							}
						}
					}
					// Not a closing tag, write the '$' and continue
					current.WriteByte(ch)
					i++
					continue
				}
			}

			current.WriteByte(ch)
			i++
		}

		current.WriteString("\n")

		// Only split on semicolon if not inside a dollar-quoted string
		// Check for semicolon before any comment
		if !inDollarQuote && hasTerminatingSemicolon(line) {
			stmt := current.String()
			statements = append(statements, stmt)
			current.Reset()
		}
	}

	// Handle any remaining content
	if current.Len() > 0 {
		stmt := current.String()
		if trimmed := strings.TrimSpace(stmt); trimmed != "" && !strings.HasSuffix(strings.TrimSpace(removeInlineComment(trimmed)), ";") {
			statements = append(statements, stmt)
		}
	}

	return statements
}

// removeInlineComment removes single-line comments from SQL
func removeInlineComment(s string) string {
	// Find first occurrence of --
	if idx := strings.Index(s, "--"); idx != -1 {
		return s[:idx]
	}
	return s
}
