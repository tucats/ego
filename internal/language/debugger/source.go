package debugger

import (
	"strings"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// indent is the number of spaces added per nesting level in the source
// listing produced by showSource.
const indent = 3

// showSource prints a range of source lines to the session writer with
// simple brace-based indentation.
//
// The optional argument after "show source" is a range spec:
//
//	show source             — list the entire program
//	show source 10          — list from line 10 to the end
//	show source 5:15        — list lines 5 through 15 (inclusive)
//
// tx is the tokenizer that holds the original source lines (tx.Source).
// tokens is positioned just before the "source" keyword; this function
// advances it to consume any range arguments.
//
// DEBUGGER-SHOW-1: the previous implementation received err by value and
// modified it locally when it detected an invalid integer argument.  Because
// Go passes parameters by value, that local change never reached the caller
// — invalid range specs were silently ignored.  The function now returns an
// error so the caller can report the problem to the user.
func showSource(tx *tokenizer.Tokenizer, tokens *tokenizer.Tokenizer, sessionContext *session) error {
	// Default: list every line in the source.
	start := 1
	end := len(tx.Source)
	nesting := 0

	// Skip past "show source" — the caller has already consumed "show", so we
	// advance past "source" here.
	tokens.Advance(2)

	// Parse optional start[:end] range.
	if tokens.Peek(1).IsNot(tokenizer.EndOfTokens) {
		var e2 error

		start, e2 = egostrings.Atoi(tokens.NextText())

		// Consume the optional colon separating start and end.
		_ = tokens.IsNext(tokenizer.ColonToken)

		if e2 == nil && tokens.Peek(1).IsNot(tokenizer.EndOfTokens) {
			end, e2 = egostrings.Atoi(tokens.NextText())
		}

		if e2 != nil {
			// DEBUGGER-SHOW-1: return the error instead of storing it in a
			// local variable that the caller can never see.
			return errors.New(errors.ErrInvalidInteger)
		}
	}

	// Emit the selected lines with simple brace-nesting indentation.
	//
	// Note (DEBUGGER-SHOW-2): brace/paren counting is performed on raw text,
	// so characters that appear inside string literals are counted too.  This
	// can produce incorrect indentation for lines like:
	//   fmt.Printf("value: %v {ok}\n", x)
	// That is a known cosmetic limitation documented in DEBUGGER_ISSUES.md.
	for i, t := range tx.Source {
		if i < start-1 || i > end-1 {
			continue
		}

		// Strip trailing semicolon (the tokenizer inserts these internally)
		// and leading/trailing whitespace before re-indenting.
		t = strings.TrimSpace(strings.TrimSuffix(t, ";"))

		opened, closed := identationCounts(t)
		if opened > closed {
			// More openers than closers on this line — indent this line at the
			// current level, then increase nesting for the next line.
			t = strings.Repeat(" ", nesting*indent) + t
			nesting++
		} else if closed > opened {
			// More closers than openers — decrease nesting first so the closing
			// brace/paren aligns with its matching opener.
			nesting--
			if nesting < 0 {
				nesting = 0
			}

			t = strings.Repeat(" ", nesting*indent) + t
		} else {
			// Balanced (or empty) line — indent at current level.
			t = strings.Repeat(" ", nesting*indent) + t
		}

		sessionContext.printf("%-5d %s\n", i+1, t)
	}

	return nil
}

// identationCounts returns the number of openers and closers in a line of
// source text.  Openers are '{' and '(', closers are '}' and ')'. This uses
// the tokenizer to avoid counting characters that appear inside string literals,
// which would produce incorrect indentation.
func identationCounts(line string) (int, int) {
	opened := 0
	closed := 0

	// Tokenize this line. The "is code" flag is true.
	tokens := tokenizer.New(line, true)

	// Scan the tokens, counting openers and closers.  The tokenizer will
	// ignore any openers/closers that appear inside string literals, so we
	// don't have to worry about that here.
	for tokens.Peek(1).IsNot(tokenizer.EndOfTokens) {
		t := tokens.Next()

		if t.Class() != tokenizer.SpecialTokenClass {
			continue
		}

		switch t.Spelling() {
		case "{", "(":
			opened++

		case "}", ")":
			closed++
		}
	}

	return opened, closed
}
