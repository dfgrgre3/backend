package main

import (
	"strings"
)

// nonTransactionalStatements contains SQL patterns that cannot run inside a transaction.
// These require special handling (outside transaction).
var nonTransactionalStatements = []string{
	"CREATE INDEX CONCURRENTLY",
	"DROP INDEX CONCURRENTLY",
	"REINDEX CONCURRENTLY",
	"CLUSTER CONCURRENTLY",
	"ANALYZE CONCURRENTLY",
}

// needsNonTransactionalExecution checks if any statement in the migration
// requires execution outside a transaction block.
func needsNonTransactionalExecution(contents string) bool {
	upperContents := strings.ToUpper(contents)
	for _, pattern := range nonTransactionalStatements {
		if strings.Contains(upperContents, pattern) {
			return true
		}
	}
	return false
}

func splitSQLStatements(contents string) []string {
	var statements []string
	var current strings.Builder
	var inDollarQuote bool
	var dollarQuoteTag string
	var inSingleQuote bool

	n := len(contents)
	for i := 0; i < n; i++ {
		ch := contents[i]

		// Single quote handling (skip escaped quotes '')
		if ch == '\'' && !inDollarQuote {
			if inSingleQuote {
				if i+1 < n && contents[i+1] == '\'' {
					current.WriteByte('\'')
					current.WriteByte('\'')
					i++
					continue
				}
				inSingleQuote = false
			} else {
				inSingleQuote = true
			}
			current.WriteByte(ch)
			continue
		}

		// Dollar quote handling (e.g. $$ or $tag$)
		if ch == '$' && !inSingleQuote {
			if !inDollarQuote {
				j := i + 1
				var tag string
				if j < n && contents[j] == '$' {
					tag = "$$"
					j++
				} else {
					for j < n && (contents[j] == '_' || (contents[j] >= 'a' && contents[j] <= 'z') || (contents[j] >= 'A' && contents[j] <= 'Z') || (contents[j] >= '0' && contents[j] <= '9')) {
						j++
					}
					if j < n && contents[j] == '$' {
						tag = contents[i : j+1]
						j++
					}
				}
				if tag != "" {
					current.WriteString(tag)
					dollarQuoteTag = tag
					inDollarQuote = true
					i = j - 1
					continue
				}
			} else {
				if dollarQuoteTag == "$$" && i+1 < n && contents[i+1] == '$' {
					current.WriteString("$$")
					inDollarQuote = false
					dollarQuoteTag = ""
					i++
					continue
				} else if dollarQuoteTag != "$$" && strings.HasPrefix(contents[i:], dollarQuoteTag) {
					current.WriteString(dollarQuoteTag)
					inDollarQuote = false
					dollarQuoteTag = ""
					i += len(dollarQuoteTag) - 1
					continue
				}
			}
		}

		// Comments handling when outside quotes: skip them without appending to statement
		if !inSingleQuote && !inDollarQuote {
			if ch == '-' && i+1 < n && contents[i+1] == '-' {
				// Single line comment: skip until end of line
				eol := strings.IndexByte(contents[i:], '\n')
				if eol == -1 {
					// End of content
					break
				}
				i += eol
				continue
			}
			if ch == '/' && i+1 < n && contents[i+1] == '*' {
				// Multi-line comment: skip until */
				end := strings.Index(contents[i+2:], "*/")
				if end != -1 {
					i += end + 3
					continue
				}
			}
		}

		// Semicolon delimiter
		if ch == ';' && !inSingleQuote && !inDollarQuote {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" {
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
