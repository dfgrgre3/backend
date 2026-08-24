package migration

import (
	"strings"
	"unicode"
)

// splitSQLStatements splits a SQL migration file into individual statements.
// It properly handles:
// - Dollar-quoted strings ($$...$$, $tag$...$tag$)
// - Single and double quoted strings
// - Line and block comments
// - Statements ending with semicolons
func splitSQLStatements(contents string) []string {
	splitter := &sqlSplitter{
		runes: []rune(contents),
	}
	return splitter.split()
}

type sqlSplitter struct {
	runes          []rune
	i              int
	statements     []string
	current        strings.Builder
	inSingleQuote  bool
	inDoubleQuote  bool
	inDollarQuote  bool
	dollarTag      string
	inLineComment  bool
	inBlockComment bool
}

func (s *sqlSplitter) split() []string {
	for s.i < len(s.runes) {
		if s.handleComments() {
			continue
		}
		if s.handleQuotes() {
			continue
		}
		if s.handleDollarQuotes() {
			continue
		}
		if s.handleTerminator() {
			continue
		}

		s.current.WriteRune(s.runes[s.i])
		s.i++
	}

	s.finalize()
	return s.statements
}

func (s *sqlSplitter) inAnyQuote() bool {
	return s.inSingleQuote || s.inDoubleQuote || s.inDollarQuote
}

func (s *sqlSplitter) handleComments() bool {
	if s.inAnyQuote() {
		return false
	}

	if s.inLineComment {
		return s.continueLineComment()
	}

	if s.inBlockComment {
		return s.continueBlockComment()
	}

	return s.startComment()
}

func (s *sqlSplitter) continueLineComment() bool {
	if s.runes[s.i] == '\n' {
		s.inLineComment = false
	}
	s.i++
	return true
}

func (s *sqlSplitter) continueBlockComment() bool {
	if s.i+1 < len(s.runes) && s.runes[s.i] == '*' && s.runes[s.i+1] == '/' {
		s.inBlockComment = false
		s.i += 2
	} else {
		s.i++
	}
	return true
}

func (s *sqlSplitter) startComment() bool {
	if s.i+1 >= len(s.runes) {
		return false
	}

	if s.runes[s.i] == '-' && s.runes[s.i+1] == '-' {
		s.inLineComment = true
		s.i += 2
		return true
	}

	if s.runes[s.i] == '/' && s.runes[s.i+1] == '*' {
		s.inBlockComment = true
		s.i += 2
		return true
	}

	return false
}

func (s *sqlSplitter) handleQuotes() bool {
	ch := s.runes[s.i]
	if ch == '\'' && !s.inDoubleQuote && !s.inDollarQuote {
		if s.inSingleQuote && s.i+1 < len(s.runes) && s.runes[s.i+1] == '\'' {
			s.current.WriteRune(ch)
			s.i += 2
			return true
		}
		s.inSingleQuote = !s.inSingleQuote
		s.current.WriteRune(ch)
		s.i++
		return true
	}

	if ch == '"' && !s.inSingleQuote && !s.inDollarQuote {
		s.inDoubleQuote = !s.inDoubleQuote
		s.current.WriteRune(ch)
		s.i++
		return true
	}

	return false
}

func (s *sqlSplitter) handleDollarQuotes() bool {
	if s.inSingleQuote || s.inDoubleQuote {
		return false
	}

	ch := s.runes[s.i]
	if ch != '$' {
		return false
	}

	if s.inDollarQuote {
		if strings.HasPrefix(string(s.runes[s.i:]), s.dollarTag) {
			s.current.WriteString(s.dollarTag)
			s.i += len([]rune(s.dollarTag))
			s.inDollarQuote = false
			s.dollarTag = ""
			return true
		}
		return false
	}

	// Look for the end of this dollar tag
	j := s.i + 1
	for j < len(s.runes) && (unicode.IsLetter(s.runes[j]) || unicode.IsDigit(s.runes[j]) || s.runes[j] == '_') {
		j++
	}
	if j < len(s.runes) && s.runes[j] == '$' {
		s.dollarTag = string(s.runes[s.i : j+1])
		s.inDollarQuote = true
		s.current.WriteString(s.dollarTag)
		s.i = j + 1
		return true
	}

	return false
}

func (s *sqlSplitter) handleTerminator() bool {
	if s.inAnyQuote() {
		return false
	}

	if s.runes[s.i] == ';' {
		s.current.WriteRune(';')
		s.addStatement()
		s.current.Reset()
		s.i++
		return true
	}

	return false
}

func (s *sqlSplitter) addStatement() {
	stmt := strings.TrimSpace(s.current.String())
	if stmt != "" {
		s.statements = append(s.statements, stmt)
	}
}

func (s *sqlSplitter) finalize() {
	s.addStatement()
}
