// Package parse converts Ego source text into an Abstract Syntax Tree (AST)
// as defined in the sibling package
// github.com/tucats/ego/internal/language/parse/ast.
//
// It reuses the Ego tokenizer (github.com/tucats/ego/internal/language/tokenizer)
// as its lexical front end and implements a hand-written recursive-descent
// parser whose structure follows the grammar in docs/SYNTAX.md. The parts of
// the grammar that change most often — operator precedence, statement-keyword
// dispatch, and type keywords — are data-driven (see tables.go) so that
// extending the language is, in the common case, a small localized edit.
//
// Two entry points are provided:
//
//   - ParseProgram parses a complete Ego program: a sequence of top-level
//     declarations and function definitions (SYNTAX.md section 2).
//   - ParseStatements parses a bare statement sequence, the fragment form
//     accepted by the REPL and by ego test's @test blocks, where executable
//     statements are legal at the top level.
//
// Both return an *ast.File. On a syntax error the returned error carries the
// offending token and its source location; a partial tree may also be returned
// for tooling that wants to inspect what parsed.
//
// Comments are preserved: the tokenizer captures them into a side list and the
// parser copies them onto File.Comments in source order (see collectComments),
// where the formatter can reproduce them. See ast/node.go for the model.
package parse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// Parser holds the state for a single parse. It is not safe for concurrent use;
// construct a new Parser (via New) per source string.
type Parser struct {
	t *tokenizer.Tokenizer

	// bare is true when parsing a statement fragment (ParseStatements) rather
	// than a full program (ParseProgram). It relaxes the rule that executable
	// statements may only appear inside a function body.
	bare bool

	// funcDepth tracks nesting inside function bodies. It is > 0 whenever the
	// parser is inside a function body, which (together with bare) governs where
	// executable statements are permitted.
	funcDepth int

	// exprLev disambiguates "X { ... }" between a composite literal and the
	// start of a control-flow block, following the same technique as Go's
	// parser. A "{" is treated as a composite-literal suffix only when
	// exprLev >= 0. Parsing a control-flow header (if/for/switch condition)
	// sets exprLev to -1 so the header's trailing "{" opens the loop/if body
	// rather than being swallowed as a struct initializer. It is incremented
	// inside parentheses and brackets, where composite literals are always
	// unambiguous.
	exprLev int
}

// New creates a Parser for the given source text. The source is tokenized in
// code mode (semicolon insertion and multi-character operator crushing enabled),
// which is required for Ego source.
func New(source string) *Parser {
	return &Parser{
		t: tokenizer.New(source, true),
	}
}

// ParseProgram parses source as a complete Ego program and returns its AST.
func ParseProgram(source string) (*ast.File, error) {
	return New(source).parseFile(false)
}

// ParseStatements parses source as a bare statement sequence (fragment form)
// and returns its AST. Executable statements are permitted at the top level.
func ParseStatements(source string) (*ast.File, error) {
	return New(source).parseFile(true)
}

// parseFile is the shared driver for both entry points. It walks the token
// stream, parsing one statement at a time until the tokens are exhausted.
func (p *Parser) parseFile(bare bool) (*ast.File, error) {
	p.bare = bare

	file := &ast.File{Bare: bare}
	file.Start = p.here()

	for !p.t.AtEnd() {
		// Consume statement separators between statements.
		if p.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
			continue
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return file, err
		}

		if stmt != nil {
			file.Decls = append(file.Decls, stmt)
		}
	}

	file.Finish = p.here()
	file.Comments = p.collectComments()

	return file, nil
}

// collectComments converts the comments captured by the tokenizer into ast
// comments, preserving source order.
func (p *Parser) collectComments() []ast.Comment {
	raw := p.t.Comments
	if len(raw) == 0 {
		return nil
	}

	comments := make([]ast.Comment, len(raw))
	for i, c := range raw {
		comments[i] = ast.Comment{
			Text:   c.Text,
			Line:   c.Line,
			Column: c.Column,
			Block:  c.Block,
		}
	}

	return comments
}

// ------------------------------------------------------------------
// Token cursor helpers
// ------------------------------------------------------------------

// peek returns the token at the given 1-based offset from the current position
// without consuming it. peek(1) is the next token.
func (p *Parser) peek(offset int) tokenizer.Token {
	return p.t.Peek(offset)
}

// next consumes and returns the next token.
func (p *Parser) next() tokenizer.Token {
	return p.t.Next()
}

// at reports whether the next token equals tok.
func (p *Parser) at(tok tokenizer.Token) bool {
	return p.t.Peek(1).Is(tok)
}

// accept consumes the next token and returns true if it equals tok; otherwise
// it leaves the cursor unchanged and returns false.
func (p *Parser) accept(tok tokenizer.Token) bool {
	return p.t.IsNext(tok)
}

// expect consumes the next token if it equals tok, or returns a located error
// built from errConst if it does not.
func (p *Parser) expect(tok tokenizer.Token, errConst error) error {
	if p.t.IsNext(tok) {
		return nil
	}

	return p.errorHere(errConst)
}

// here returns the source position of the next token.
func (p *Parser) here() ast.Position {
	tok := p.t.Peek(1)
	line, col := tok.Location()

	return ast.Position{Line: line, Column: col}
}

// end returns the source position just past the most recently consumed token.
// Unlike here (which points at the next, not-yet-consumed token), this reflects
// where the just-parsed construct actually ended, which matters for correlating
// trailing comments and for a block's closing-brace line.
func (p *Parser) end() ast.Position {
	tok := p.t.Peek(0)
	line, col := tok.Location()

	return ast.Position{Line: line, Column: col + len(tok.Spelling())}
}

// errorHere builds a located parse error from errConst, attaching the spelling
// and location of the next token as context.
func (p *Parser) errorHere(errConst error) error {
	tok := p.t.Peek(1)
	line, col := tok.Location()

	return errors.New(errConst).Context(tok.Spelling()).At(line, col)
}

// isEndOfStatement reports whether the cursor is at a statement boundary (end of
// tokens or a semicolon), without consuming anything.
func (p *Parser) isEndOfStatement() bool {
	return p.t.EndOfStatement()
}
