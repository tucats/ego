package tokenizer

import (
	"fmt"
	"strings"
	"text/scanner"

	"github.com/tucats/ego/util"
)

// Tokenizer is an instance of a tokenized string.
type Tokenizer struct {
	Source []string
	Tokens []string
	TokenP int
	Line   []int
	Pos    []int
}

// EndOfTokens is a reserved token that means end of the buffer was reached.
const EndOfTokens = "<<end-of-tokens>>"

// New creates a tokenizer instance and breaks the string
// up into an array of tokens
func New(src string) *Tokenizer {
	var s scanner.Scanner

	t := Tokenizer{Source: splitLines(src), TokenP: 0}
	t.Tokens = make([]string, 0)

	s.Init(strings.NewReader(src))
	s.Error = func(s *scanner.Scanner, msg string) { /* suppress messaging */ }
	s.Filename = "Input"

	previousToken := ""
	dots := 0

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		// See if this is one of the special cases where we patch up the previous token
		nextToken := s.TokenText()

		// Make interface{} a single token
		if nextToken == "}" && len(t.Tokens) > 3 {
			tlen := len(t.Tokens)
			if t.Tokens[tlen-1] == "{" && t.Tokens[tlen-2] == "interface" {
				t.Tokens = append(t.Tokens[:tlen-2], "interface{}")

				continue
			}
		}

		// Make {} a single token
		if nextToken == "}" && previousToken == "{" {
			t.Tokens = append(t.Tokens[:len(t.Tokens)-1], "{}")

			continue
		}

		// Make "..." a single token
		if nextToken == "." {
			dots = dots + 1
			if dots == 3 {
				t.Tokens = append(t.Tokens[:len(t.Tokens)-2], "...")
				dots = 0

				continue
			}
		} else {
			dots = 0
		}

		// Handle compound relationships like ">=" or "!=""
		if nextToken == "=" {
			if util.InList(previousToken, "!", "<", ">", ":", "=") {
				t.Tokens[len(t.Tokens)-1] = previousToken + nextToken
				previousToken = ""

				continue
			}
		}

		// Handle channel route "<-" token
		if nextToken == "-" && previousToken == "<" {
			t.Tokens[len(t.Tokens)-1] = previousToken + nextToken
			previousToken = ""

			continue
		}

		if nextToken == ">" {
			if previousToken == "-" {
				t.Tokens[len(t.Tokens)-1] = previousToken + nextToken
				previousToken = ""

				continue
			}
		}

		previousToken = nextToken
		t.Tokens = append(t.Tokens, nextToken)
		t.Line = append(t.Line, s.Line)
		t.Pos = append(t.Pos, s.Column)
	}

	return &t
}

// PositionString reports the position of the current
// token in terms of line and column information.
func (t *Tokenizer) PositionString() string {
	p := t.TokenP
	if p >= len(t.Line) {
		p = len(t.Line) - 1
	}

	return fmt.Sprintf(util.LineColumnFormat, t.Line[p], t.Pos[p])
}

// Next gets the next token in the tokenizer
func (t *Tokenizer) Next() string {
	if t.TokenP >= len(t.Tokens) {
		return EndOfTokens
	}

	token := t.Tokens[t.TokenP]
	t.TokenP = t.TokenP + 1

	return token
}

// Peek looks ahead at the next token without advancing the pointer.
func (t *Tokenizer) Peek(offset int) string {
	if t.TokenP+(offset-1) >= len(t.Tokens) {
		return EndOfTokens
	}

	return t.Tokens[t.TokenP+(offset-1)]
}

// AtEnd indicates if we are at the end of the string
func (t *Tokenizer) AtEnd() bool {
	return t.TokenP >= len(t.Tokens)
}

// Advance moves the pointer
func (t *Tokenizer) Advance(p int) {
	t.TokenP = t.TokenP + p
	if t.TokenP < 0 {
		t.TokenP = 0
	}

	if t.TokenP >= len(t.Tokens) {
		t.TokenP = len(t.Tokens)
	}
}

// IsNext tests to see if the next token is the given token, and if so
// advances and returns true, else does not advance and returns false.
func (t *Tokenizer) IsNext(test string) bool {
	if t.Peek(1) == test {
		t.Advance(1)

		return true
	}

	return false
}

// AnyNext tests to see if the next token is in the given  list
// of tokens, and if so  advances and returns true, else does not
// advance and returns false.
func (t *Tokenizer) AnyNext(test ...string) bool {
	n := t.Peek(1)
	for _, v := range test {
		if n == v {
			t.Advance(1)

			return true
		}
	}

	return false
}

func stripComments(source string) string {
	var result strings.Builder

	ignore := false
	startOfLine := true

	for _, c := range source {
		// Is this a # on the start of a line? If so, start
		// ignoring characters. If it's the end of line, then
		// reset to end-of-line and resume processing characters.
		// Finally, if nothing else, copy character if not ignoring.
		if c == '#' && startOfLine {
			ignore = true
		} else if c == '\n' {
			ignore = false
			startOfLine = true
			result.WriteRune(c)
		} else if !ignore {
			result.WriteRune(c)
		}
	}

	return result.String()
}

// IsSymbol is a utility function to determine if a token is a symbol name.
func IsSymbol(s string) bool {
	for n, c := range s {
		if isLetter(c) {
			continue
		}

		if isDigit(c) && n > 0 {
			continue
		}

		return false
	}

	return true
}

func isLetter(c rune) bool {
	for _, d := range "_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		if c == d {
			return true
		}
	}

	return false
}

func isDigit(c rune) bool {
	for _, d := range "0123456789" {
		if c == d {
			return true
		}
	}

	return false
}

// GetLine returns a given line of text from the token stream.
// This actuals refers to the original line splits done when the
// source was first received.
func (t *Tokenizer) GetLine(line int) string {
	if line < 1 || line > len(t.Source) {
		return ""
	}

	return t.Source[line-1]
}

// splitLines splits a string by line endings, and returns the
// source as an array of strings.
func splitLines(src string) []string {
	// Are we seeing Windows-style line endings? If so, use that as
	// the split boundary.
	if strings.Index(src, "\r\n") > 0 {
		return strings.Split(src, "\r\n")
	}

	// Otherwise, simple split by new-line works fine.
	return strings.Split(src, "\n")
}

// GetSource returns the entire string of the tokenizer
func (t *Tokenizer) GetSource() string {
	r := ""
	for _, line := range t.Source {
		r = r + line + "\n"
	}

	return r
}

// GetTokens returns a string representing the tokens
// within the given range of tokens.
func (t *Tokenizer) GetTokens(pos1, pos2 int, spacing bool) string {
	p1 := pos1
	if p1 < 0 {
		p1 = 0
	} else {
		if p1 > len(t.Tokens) {
			p1 = len(t.Tokens)
		}
	}
	p2 := pos2
	if p2 < p1 {
		p2 = p1
	} else {
		if p2 > len(t.Tokens) {
			p2 = len(t.Tokens)
		}
	}

	var s strings.Builder

	for _, t := range t.Tokens[p1:p2] {
		s.WriteString(t)

		if spacing {
			s.WriteRune(' ')
		}
	}

	return s.String()
}
