package parse

import (
	"strings"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file implements directive parsing (SYNTAX.md sections 12–13). Ego has
// roughly forty built-in directives plus open-ended user macros, so the AST
// models every directive with a single generic DirectiveStmt node rather than a
// bespoke node per directive. The parser's job here is therefore narrow:
// capture the directive name and consume its arguments so that outer parsing
// continues at the correct place.
//
// Argument capture is deliberately conservative. A directive's arguments run to
// the end of the logical line, except that:
//
//   - Brace-delimited block arguments (@capture, @compile, @json, @text, ...)
//     are consumed with balanced-brace tracking so an inserted semicolon inside
//     the block does not end the directive prematurely.
//   - A following "catch" or "else" clause (as used by @compile) continues the
//     directive rather than starting a new statement.
//
// The captured argument tokens are stored verbatim in DirectiveStmt.RawArgs.
//
// TRIVIA: A canonical formatter will eventually want a real sub-AST for the code
// inside block-bearing directives such as @compile and @capture, so it can
// reformat that code too. Today those blocks are captured as raw token
// spellings only. When that work begins, this is the place to recursively parse
// the block body into ast nodes and hang them off DirectiveStmt. Search for
// "TRIVIA:" across the package for the related breadcrumbs.
func (p *Parser) parseDirective() (ast.Node, error) {
	start := p.here()

	p.next() // consume "@"

	nameTok := p.t.Peek(1)
	if !nameTok.IsName() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	p.next()

	node := &ast.DirectiveStmt{Name: nameTok.Spelling()}
	node.RawArgs = p.collectDirectiveArgs()
	node.SetSpan(start, p.here())

	return node, nil
}

// collectDirectiveArgs consumes the tokens making up a directive's arguments and
// returns their spellings. It tracks brace/paren/bracket depth so that a block
// argument spanning multiple lines is captured whole, and it treats a trailing
// "catch"/"else" continuation as part of the same directive.
//
// One special form is recognized: the "@compile eof=STRING" flag (SYNTAX.md
// 12.2) delimits the compiled body with a text marker instead of braces, so
// once such a flag is seen, the body is captured up to and including the run of
// tokens whose spellings concatenate to that marker, after which normal
// brace/continuation handling resumes for any trailing "catch" clause.
func (p *Parser) collectDirectiveArgs() []string {
	var raw []string

	depth := 0

	for !p.t.AtEnd() {
		tok := p.t.Peek(1)

		// Recognize an "eof=STRING" flag and switch to marker-delimited capture.
		if depth == 0 && tok.Spelling() == "eof" &&
			p.t.Peek(2).Is(tokenizer.AssignToken) && p.t.Peek(3).IsString() {
			marker := p.t.Peek(3).Spelling()
			raw = append(raw, p.next().Spelling(), p.next().Spelling(), p.next().Spelling())
			raw = append(raw, p.consumeToMarker(marker)...)

			continue
		}

		// A depth-zero closing brace/bracket/paren belongs to an enclosing
		// construct (e.g. the block containing this directive), never to the
		// directive's own arguments — a directive's own braces are matched and
		// raise depth above zero. Stop before consuming it. Ego does not always
		// insert a semicolon before a "}" (for example when the directive's line
		// ends in a comment), so this check, not just the semicolon check below,
		// is what keeps the directive from swallowing its enclosing block.
		if depth == 0 && (tok.Is(tokenizer.DataEndToken) || tok.Is(tokenizer.BlockEndToken) ||
			tok.Is(tokenizer.EndOfListToken) || tok.Is(tokenizer.EndOfArrayToken)) {
			break
		}

		// At the top level, a statement boundary ends the directive — unless a
		// continuation keyword (catch/else) follows, in which case we keep going.
		if depth == 0 && (tok.Is(tokenizer.SemicolonToken) || tok.Is(tokenizer.EndOfTokens)) {
			if p.continuationFollows() {
				p.next() // consume the separating semicolon

				continue
			}

			break
		}

		switch {
		case tok.Is(tokenizer.BlockBeginToken), tok.Is(tokenizer.DataBeginToken),
			tok.Is(tokenizer.StartOfListToken), tok.Is(tokenizer.StartOfArrayToken):
			depth++
		case tok.Is(tokenizer.BlockEndToken), tok.Is(tokenizer.DataEndToken),
			tok.Is(tokenizer.EndOfListToken), tok.Is(tokenizer.EndOfArrayToken):
			if depth > 0 {
				depth--
			}
		}

		raw = append(raw, p.next().Spelling())
	}

	return raw
}

// consumeToMarker consumes tokens up to and including the run whose concatenated
// spellings equal marker (the @compile eof= delimiter), returning the spellings
// consumed. If the marker is never found, it consumes to end of input.
func (p *Parser) consumeToMarker(marker string) []string {
	var raw []string

	for !p.t.AtEnd() {
		if n := p.markerRunLength(marker); n > 0 {
			for i := 0; i < n; i++ {
				raw = append(raw, p.next().Spelling())
			}

			return raw
		}

		raw = append(raw, p.next().Spelling())
	}

	return raw
}

// markerRunLength reports how many upcoming tokens, concatenated, exactly equal
// marker, or 0 if the tokens at the cursor do not begin such a run.
func (p *Parser) markerRunLength(marker string) int {
	var b strings.Builder

	for offset := 1; b.Len() < len(marker); offset++ {
		tok := p.t.Peek(offset)
		if tok.Is(tokenizer.EndOfTokens) {
			return 0
		}

		b.WriteString(tok.Spelling())

		if b.String() == marker {
			return offset
		}

		if !strings.HasPrefix(marker, b.String()) {
			return 0
		}
	}

	return 0
}

// continuationFollows reports whether, skipping any run of semicolons at the
// cursor, the next token is a directive continuation keyword ("catch" or
// "else"). Such a keyword means the current directive is not yet complete.
func (p *Parser) continuationFollows() bool {
	for offset := 1; ; offset++ {
		tok := p.t.Peek(offset)
		if tok.Is(tokenizer.SemicolonToken) {
			continue
		}

		return tok.Is(tokenizer.CatchToken) || tok.Is(tokenizer.ElseToken)
	}
}
