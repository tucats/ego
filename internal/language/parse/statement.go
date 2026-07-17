package parse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file implements the statement grammar of docs/SYNTAX.md sections 3, 4,
// 8, 10, and 11, plus function definitions (section 7). Statement dispatch is
// data-driven through the statementKeywords and declKeywords tables in
// tables.go.

// parseStatement parses a single statement.
func (p *Parser) parseStatement() (ast.Node, error) {
	tok := p.t.Peek(1)

	// "{}" empty-block shorthand is a no-op statement.
	if tok.Is(tokenizer.EmptyBlockToken) {
		start := p.here()
		p.next()

		node := &ast.EmptyStmt{}
		node.SetSpan(start, p.here())

		return node, nil
	}

	// A "{" opens a nested block.
	if tok.Is(tokenizer.BlockBeginToken) {
		return p.parseBlock()
	}

	// Compiler directives / macros begin with "@".
	if tok.Is(tokenizer.DirectiveToken) {
		return p.parseDirective()
	}

	// Function definitions and function literals both begin with "func".
	if tok.Is(tokenizer.FuncToken) {
		if p.funcIsDeclaration() {
			return p.parseFuncDef()
		}
		// A function literal used as a statement (e.g. an immediately-invoked
		// closure) is an expression statement.
		return p.parseSimpleStatement()
	}

	// Declarations are legal at top level and inside blocks.
	if fn, ok := declKeywords[tok.Spelling()]; ok && tok.IsClass(tokenizer.ReservedTokenClass) {
		return fn(p)
	}

	// A labeled for-loop: IDENTIFIER ":" ... "for".
	if tok.IsIdentifier() && p.t.Peek(2).Is(tokenizer.ColonToken) && p.labelIsLoop() {
		return p.parseLabeledStmt()
	}

	// Keyword-led statements (if/for/switch/return/...).
	if fn, ok := statementKeywords[tok.Spelling()]; ok && tok.IsClass(tokenizer.ReservedTokenClass) {
		return fn(p)
	}

	// Everything else is a simple statement: assignment, channel op, inc/dec,
	// or an expression statement.
	return p.parseSimpleStatement()
}

// parseBlock parses "{ statements }" (SYNTAX.md: block), including the "{}"
// empty-block shorthand.
func (p *Parser) parseBlock() (*ast.Block, error) {
	start := p.here()

	if p.accept(tokenizer.EmptyBlockToken) {
		node := &ast.Block{Empty: true}
		node.SetSpan(start, p.here())

		return node, nil
	}

	if err := p.expect(tokenizer.BlockBeginToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	block := &ast.Block{}

	for !p.at(tokenizer.DataEndToken) && !p.t.AtEnd() {
		if p.t.AnyNext(tokenizer.SemicolonToken) {
			continue
		}

		if p.at(tokenizer.DataEndToken) {
			break
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}

		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
	}

	if err := p.expect(tokenizer.DataEndToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	// Use the closing brace's own position (not the next token) so the block's
	// end line is exactly the "}" line — the boundary the formatter uses when
	// flushing comments that fall inside the block.
	block.SetSpan(start, p.end())

	return block, nil
}

// parseSimpleStatement parses the non-keyword statement forms (SYNTAX.md:
// assignStmt and the funcCallStmt/expression-statement case): assignment,
// channel send, post-increment/decrement, and expression statement.
func (p *Parser) parseSimpleStatement() (ast.Node, error) {
	start := p.here()

	first, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	// Build the left-hand list for a possible multi-target assignment.
	lhs := []ast.Node{first}

	for p.accept(tokenizer.CommaToken) {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		lhs = append(lhs, expr)
	}

	next := p.t.Peek(1)

	// Assignment: "=", ":=", "+=", "-=", "*=", "/=".
	if next.IsClass(tokenizer.SpecialTokenClass) && assignOps[next.Spelling()] {
		op := p.next().Spelling()

		rhs, err := p.parseExprList()
		if err != nil {
			return nil, err
		}

		node := &ast.AssignStmt{Lhs: lhs, Op: op, Rhs: rhs}
		node.SetSpan(start, p.here())

		return node, nil
	}

	// Post-increment / decrement (single target only).
	if next.Is(tokenizer.IncrementToken) || next.Is(tokenizer.DecrementToken) {
		p.next()

		node := &ast.IncDecStmt{X: first, Op: next.Spelling()}
		node.SetSpan(start, p.here())

		return node, nil
	}

	// Channel send: "chan <- value".
	if next.Is(tokenizer.ChannelReceiveToken) {
		p.next()

		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		node := &ast.SendStmt{Chan: first, Value: value}
		node.SetSpan(start, value.End())

		return node, nil
	}

	// Otherwise it is an expression statement (typically a function call).
	node := &ast.ExprStmt{X: first}
	node.SetSpan(start, first.End())

	return node, nil
}

// parseExprList parses a comma-separated list of expressions (the right-hand
// side of an assignment).
func (p *Parser) parseExprList() ([]ast.Node, error) {
	var list []ast.Node

	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		list = append(list, expr)

		if !p.accept(tokenizer.CommaToken) {
			break
		}
	}

	return list, nil
}

// labelIsLoop reports whether an "IDENTIFIER :" at the cursor labels a for loop.
// It looks past the label and any inserted semicolons for the "for" keyword.
func (p *Parser) labelIsLoop() bool {
	// peek(1)=IDENT, peek(2)=":". Look for "for" after the colon, skipping any
	// statement separators.
	for offset := 3; ; offset++ {
		tok := p.t.Peek(offset)
		if tok.Is(tokenizer.SemicolonToken) {
			continue
		}

		return tok.Is(tokenizer.ForToken)
	}
}

// parseLabeledStmt parses "IDENTIFIER : forStmt".
func (p *Parser) parseLabeledStmt() (ast.Node, error) {
	start := p.here()

	label := p.next().Spelling() // identifier
	p.next()                     // ":"

	// Skip separators between the label and the for statement.
	for p.t.AnyNext(tokenizer.SemicolonToken) {
	}

	stmt, err := p.parseFor()
	if err != nil {
		return nil, err
	}

	node := &ast.LabeledStmt{Label: label, Stmt: stmt}
	node.SetSpan(start, stmt.End())

	return node, nil
}

// ------------------------------------------------------------------
// Function definitions and literals (SYNTAX.md section 7)
// ------------------------------------------------------------------

// funcIsDeclaration reports whether the "func" at the cursor introduces a named
// function definition (as opposed to a function literal). A declaration is
// "func Name(...)" or a method "func (recv) Name(...)".
func (p *Parser) funcIsDeclaration() bool {
	// peek(1) is "func".
	second := p.t.Peek(2)

	// "func Name ..." — named function.
	if second.IsIdentifier() {
		return true
	}

	// "func (recv) Name ..." — method. Scan to the matching ")" and check
	// whether an identifier (the method name) follows.
	if second.Is(tokenizer.StartOfListToken) {
		depth := 0

		for offset := 2; ; offset++ {
			tok := p.t.Peek(offset)
			if tok.Is(tokenizer.EndOfTokens) {
				return false
			}

			if tok.Is(tokenizer.StartOfListToken) {
				depth++
			} else if tok.Is(tokenizer.EndOfListToken) {
				depth--

				if depth == 0 {
					return p.t.Peek(offset + 1).IsIdentifier()
				}
			}
		}
	}

	return false
}

// parseFuncDef parses a named function or method definition (SYNTAX.md:
// funcDef).
func (p *Parser) parseFuncDef() (ast.Node, error) {
	start := p.here()

	p.next() // consume "func"

	node := &ast.FuncDecl{}

	// Optional method receiver.
	if p.at(tokenizer.StartOfListToken) {
		recv, err := p.parseReceiver()
		if err != nil {
			return nil, err
		}

		node.Recv = recv
	}

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	node.Name = p.ident()

	sig, err := p.parseFuncSignature(start)
	if err != nil {
		return nil, err
	}

	node.Type = sig

	p.funcDepth++

	body, err := p.parseBlock()

	p.funcDepth--

	if err != nil {
		return nil, err
	}

	node.Body = body
	node.SetSpan(start, body.End())

	return node, nil
}

// parseReceiver parses a method receiver "( Name [*] Type )" (SYNTAX.md:
// receiver).
func (p *Parser) parseReceiver() (*ast.Receiver, error) {
	start := p.here()

	p.next() // consume "("

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	name := p.ident()
	pointer := p.accept(tokenizer.PointerToken)

	typeTok := p.t.Peek(1)
	if !typeTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	typ := p.ident()

	if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
		return nil, err
	}

	node := &ast.Receiver{Name: name, Type: typ, Pointer: pointer}
	node.SetSpan(start, p.here())

	return node, nil
}

// parseFuncLit parses an anonymous function literal (SYNTAX.md: funcLiteral).
// Immediate invocation ("func(){}()") is handled by the reference-suffix loop
// in parseReference, so it is not parsed here.
func (p *Parser) parseFuncLit() (ast.Node, error) {
	start := p.here()

	p.next() // consume "func"

	sig, err := p.parseFuncSignature(start)
	if err != nil {
		return nil, err
	}

	p.funcDepth++

	body, err := p.parseBlock()

	p.funcDepth--

	if err != nil {
		return nil, err
	}

	node := &ast.FuncLit{Type: sig, Body: body}
	node.SetSpan(start, body.End())

	return node, nil
}
