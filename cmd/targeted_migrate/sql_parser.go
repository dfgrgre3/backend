package main

import (
	"fmt"
	"strings"
)

type sqlParser struct {
	content        []rune
	pos            int
	inSingle       bool
	inDouble       bool
	inDollar       bool
	dollarTag      string
	inLineComment  bool
	inBlockComment bool
}

func newSQLParser(content string) *sqlParser {
	return &sqlParser{content: []rune(content)}
}

func (p *sqlParser) done() bool {
	return p.pos >= len(p.content)
}

func (p *sqlParser) peek() rune {
	return p.content[p.pos]
}

func (p *sqlParser) peekAt(offset int) (rune, bool) {
	idx := p.pos + offset
	if idx < len(p.content) {
		return p.content[idx], true
	}
	return 0, false
}

func (p *sqlParser) skip(n int) {
	p.pos += n
}

func (p *sqlParser) handleLineComment() bool {
	next, ok := p.peekAt(1)
	if !ok {
		return false
	}
	if !p.inSingle && !p.inDouble && !p.inDollar && p.peek() == '-' && next == '-' {
		p.inLineComment = true
		p.skip(2)
		return true
	}
	return false
}

func (p *sqlParser) handleBlockComment() bool {
	next, ok := p.peekAt(1)
	if !ok {
		return false
	}
	if !p.inSingle && !p.inDouble && !p.inDollar && p.peek() == '/' && next == '*' {
		p.inBlockComment = true
		p.skip(2)
		return true
	}
	return false
}

func (p *sqlParser) handleDollarQuote() bool {
	if p.inSingle || p.inDouble || p.inDollar {
		return false
	}
	if p.peek() != '$' {
		return false
	}
	j := p.pos + 1
	for j < len(p.content) && (p.content[j] == '_' ||
		(p.content[j] >= 'a' && p.content[j] <= 'z') ||
		(p.content[j] >= 'A' && p.content[j] <= 'Z') ||
		(p.content[j] >= '0' && p.content[j] <= '9')) {
		j++
	}
	if j >= len(p.content) || p.content[j] != '$' {
		return false
	}
	p.dollarTag = string(p.content[p.pos : j+1])
	p.inDollar = true
	return true
}

func splitSQL(content string) []string {
	var stmts []string
	var cur strings.Builder
	p := newSQLParser(content)

	for !p.done() {
		ch := p.peek()

		if p.tryProcessToken(&cur, &stmts, ch) {
			continue
		}

		cur.WriteRune(ch)
		p.skip(1)
	}

	if stmt := strings.TrimSpace(cur.String()); stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
}

// tryProcessToken attempts to handle the current character with one of the
// token-processing strategies (comments, quotes, dollar-quotes, semicolons).
// Returns true if the token was consumed and the caller should continue the loop.
func (p *sqlParser) tryProcessToken(cur *strings.Builder, stmts *[]string, ch rune) bool {
	if processCommentState(p, ch) {
		return true
	}
	if p.handleUnquotedState(cur, stmts, ch) {
		return true
	}
	if processQuoteStates(p, cur, ch) {
		return true
	}
	if p.handleDollarQuoteClose(cur) {
		return true
	}
	return false
}

func (p *sqlParser) handleUnquotedState(cur *strings.Builder, stmts *[]string, ch rune) bool {
	if p.inSingle || p.inDouble || p.inDollar {
		return false
	}
	if p.handleLineComment() {
		return true
	}
	if p.handleBlockComment() {
		return true
	}
	if p.handleDollarQuote() {
		cur.WriteString(p.dollarTag)
		p.skip(len([]rune(p.dollarTag)))
		return true
	}
	if ch == ';' {
		cur.WriteRune(ch)
		if stmt := strings.TrimSpace(cur.String()); stmt != "" {
			*stmts = append(*stmts, stmt)
		}
		cur.Reset()
		p.skip(1)
		return true
	}
	return false
}

func (p *sqlParser) handleDollarQuoteClose(cur *strings.Builder) bool {
	if p.inDollar && strings.HasPrefix(string(p.content[p.pos:]), p.dollarTag) {
		cur.WriteString(p.dollarTag)
		p.skip(len([]rune(p.dollarTag)))
		p.inDollar = false
		p.dollarTag = ""
		return true
	}
	return false
}

// processCommentState handles inline and block comment advancement.
// Returns true if the caller should continue the loop.
func processCommentState(p *sqlParser, ch rune) bool {
	if p.inLineComment {
		if ch == '\n' {
			p.inLineComment = false
		}
		p.skip(1)
		return true
	}

	if p.inBlockComment {
		next, ok := p.peekAt(1)
		if ok && ch == '*' && next == '/' {
			p.inBlockComment = false
			p.skip(2)
		} else {
			p.skip(1)
		}
		return true
	}
	return false
}

// processQuoteStates handles single and double quote advancement.
// Returns true if the caller should continue the loop.
func processQuoteStates(p *sqlParser, cur *strings.Builder, ch rune) bool {
	if ch == '\'' && !p.inDouble && !p.inDollar {
		next, ok := p.peekAt(1)
		if p.inSingle && ok && next == '\'' {
			cur.WriteRune(ch)
			cur.WriteRune(ch)
			p.skip(2)
			return true
		}
		p.inSingle = !p.inSingle
		cur.WriteRune(ch)
		p.skip(1)
		return true
	}

	if ch == '"' && !p.inSingle && !p.inDollar {
		p.inDouble = !p.inDouble
		cur.WriteRune(ch)
		p.skip(1)
		return true
	}
	return false
}

// suppress unused-import linter warning if fmt is unused
var _ = fmt.Sprintf
