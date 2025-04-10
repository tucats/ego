package tokenizer

import (
	"strconv"
	"strings"
	"text/scanner"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/util"
)

// lexer provides the lexical analysis of the source string, to break it into valid _Ego_ tokens.
// This starts by using the built-in Go scanner which handles most tokens in a fashion compatible
// with the Go language. This is followed by additional processing such as scanning for tokens
// that should be merged (crushed) together, and other special cases.
func (t *Tokenizer) lexer(src string, isCode bool) {
	var (
		s scanner.Scanner
	)

	s.Init(strings.NewReader(src))

	// Redirect any lexical scanning errors to the tokenizer log, if enabled.
	s.Error = func(s *scanner.Scanner, msg string) {
		ui.Log(ui.TokenLogger, "token.lexer", ui.A{
			"error": msg})
	}

	s.Filename = "Input"

	// Scan as long as there are tokens left.
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		// Based on classifying the spelling, decide what kind of token it is. This includes validating the
		// token against lists of known token types, or determining if the text is a valid constant of some type.
		nextToken := classifyTokenBySpelling(s.TokenText())
		nextToken.line = int32(s.Line)
		nextToken.pos = int32(s.Position.Column)

		t.Tokens = append(t.Tokens, nextToken)

		// See if this is one of the special cases convert multiple tokens into
		// a single token? We only do this when we know we are parsing _Ego_ code,
		// as opposed to a user-supplied string.
		if isCode {
			for _, crush := range crushedTokens {
				if len(crush.source) > len(t.Tokens) {
					continue
				}

				found := true

				for i, ch := range crush.source {
					if t.Tokens[len(t.Tokens)-len(crush.source)+i].IsNot(ch) {
						found = false

						break
					}
				}

				if found {
					nextToken.class = crush.result.class
					nextToken.spelling = crush.result.spelling

					offset := int32(len(crush.result.spelling) - 1)
					nextToken.pos -= offset

					// Remove the crushed tokens from the token queue.
					count := len(crush.source)

					t.Tokens = t.Tokens[:len(t.Tokens)-count]
					t.Tokens = append(t.Tokens, nextToken)

					break
				}
			}
		}
	}
}

// Given a string object, create a token of the correct class based on its spelling.
func classifyTokenBySpelling(text string) Token {
	var nextToken Token

	if TypeTokens[NewTypeToken(text)] {
		nextToken = NewTypeToken(text)
	} else if util.InList(text, "true", "false") {
		nextToken = Token{class: BooleanTokenClass, spelling: text}
	} else if tx := NewReservedToken(text); tx.IsReserved(true) {
		nextToken = tx
	} else if IsSymbol(text) {
		nextToken = NewIdentifierToken(text)
	} else if SpecialTokens[NewSpecialToken(text)] {
		nextToken = NewSpecialToken(text)
	} else if strings.HasPrefix(text, "\"") && strings.HasSuffix(text, "\"") {
		rawString := unQuote(text)
		nextToken = NewStringToken(rawString)
	} else if strings.HasPrefix(text, "`") && strings.HasSuffix(text, "`") {
		nextToken = NewStringToken(strings.TrimPrefix(strings.TrimSuffix(text, "`"), "`"))
	} else if _, err := strconv.ParseInt(text, 10, 64); err == nil {
		nextToken = Token{class: IntegerTokenClass, spelling: text}
	} else if _, err := strconv.ParseFloat(text, 64); err == nil {
		nextToken = Token{class: FloatTokenClass, spelling: text}
	} else {
		nextToken = Token{class: ValueTokenClass, spelling: text}
	}

	return nextToken
}
