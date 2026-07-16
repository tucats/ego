package parse

import (
	"strings"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file implements the atom production of docs/SYNTAX.md section 9.1 — the
// leaf operands of the expression grammar and the composite/type-led forms that
// begin an operand.

// parseAtom parses a single atom (SYNTAX.md: atom). Reference suffixes
// (.member, [index], (args), {init}) are applied by the caller, parseReference.
func (p *Parser) parseAtom() (ast.Node, error) {
	tok := p.t.Peek(1)

	// Literal scalar values, classified by token class.
	if lit, ok := p.literalKind(tok); ok {
		p.next()

		node := &ast.BasicLit{LitKind: lit, Value: tok.Spelling()}
		node.SetSpan(tokenPos(tok), tokenEnd(tok))

		return node, nil
	}

	switch {
	case tok.Is(tokenizer.NilToken):
		p.next()

		node := &ast.BasicLit{LitKind: ast.LitNil, Value: "nil"}
		node.SetSpan(tokenPos(tok), tokenEnd(tok))

		return node, nil

	case tok.Is(tokenizer.StartOfListToken):
		return p.parseParen()

	case tok.Is(tokenizer.FuncToken):
		return p.parseFuncLit()

	case tok.Is(tokenizer.AddressToken):
		return p.parseUnaryAtom(tokenizer.AddressToken)

	case tok.Is(tokenizer.PointerToken):
		return p.parseUnaryAtom(tokenizer.PointerToken)

	case tok.Is(tokenizer.ChannelReceiveToken):
		return p.parseUnaryAtom(tokenizer.ChannelReceiveToken)

	case tok.Is(tokenizer.StartOfArrayToken):
		return p.parseBracketAtom()

	case tok.Is(tokenizer.DataBeginToken) || tok.Is(tokenizer.EmptyBlockToken):
		return p.parseUntypedComposite()

	case tok.Is(tokenizer.DirectiveToken):
		return p.parseMacroAtom()

	case tok.Is(tokenizer.IfToken):
		return p.parseIfExpr()

	case tok.Is(tokenizer.OptionalToken):
		return p.parseOptionalExpr()

	case tok.Is(tokenizer.MakeToken), tok.Is(tokenizer.RecoverToken):
		// Built-in reserved words that are used like functions in expression
		// position (make(...), recover()). Treat the keyword as a name; the
		// reference-suffix loop supplies the "(...)" call.
		p.next()

		node := &ast.Ident{Name: tok.Spelling()}
		node.SetSpan(tokenPos(tok), tokenEnd(tok))

		return node, nil

	case p.startsType(tok):
		// A type-led atom: primitive/map/struct/interface/chan type name. The
		// reference-suffix loop turns a trailing "(...)" into a cast and a
		// trailing "{...}" into a composite literal.
		return p.parseTypeSpec()

	case tok.IsIdentifier():
		p.next()

		node := &ast.Ident{Name: tok.Spelling()}
		node.SetSpan(tokenPos(tok), tokenEnd(tok))

		return node, nil
	}

	return nil, p.errorHere(errors.ErrMissingTerm)
}

// literalKind maps a token's class to a BasicLit LitKind, reporting whether the
// token is a scalar literal.
func (p *Parser) literalKind(tok tokenizer.Token) (ast.LitKind, bool) {
	switch tok.Class() {
	case tokenizer.IntegerTokenClass:
		return ast.LitInt, true
	case tokenizer.FloatTokenClass:
		return ast.LitFloat, true
	case tokenizer.ComplexTokenClass:
		return ast.LitImaginary, true
	case tokenizer.BooleanTokenClass:
		return ast.LitBool, true
	case tokenizer.StringTokenClass:
		return ast.LitString, true
	case tokenizer.ValueTokenClass:
		// The lexer's catch-all class covers literal forms it does not classify
		// as base-10 int/float: hex/octal/binary integer literals (0x, 0o, 0b)
		// and rune literals ('a'). Distinguish them by spelling.
		return valueLitKind(tok.Spelling()), true
	default:
		return ast.LitInvalid, false
	}
}

// valueLitKind classifies a ValueTokenClass spelling into a literal kind.
func valueLitKind(spelling string) ast.LitKind {
	if strings.HasPrefix(spelling, "'") {
		return ast.LitRune
	}

	return ast.LitInt
}

// startsType reports whether tok begins a type spec that can appear as an atom
// (a cast or typed composite literal). The "[", "*", and "func" forms are
// handled by dedicated atom cases, so they are intentionally excluded here.
func (p *Parser) startsType(tok tokenizer.Token) bool {
	if tok.Is(tokenizer.MapToken) || tok.Is(tokenizer.StructToken) ||
		tok.Is(tokenizer.InterfaceToken) || tok.Is(tokenizer.ChanToken) ||
		tok.Is(tokenizer.AnyToken) {
		return true
	}

	// A primitive type keyword (int, string, float64, ...) leads a cast.
	return tok.IsClass(tokenizer.TypeTokenClass) && primitiveTypeTokens[tok.Spelling()]
}

// parseParen parses a parenthesized expression "( expr )".
func (p *Parser) parseParen() (ast.Node, error) {
	start := p.here()

	p.next() // consume "("

	savedLev := p.exprLev
	p.exprLev++

	inner, err := p.parseExpression()
	p.exprLev = savedLev

	if err != nil {
		return nil, err
	}

	if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
		return nil, err
	}

	node := &ast.ParenExpr{X: inner}
	node.SetSpan(start, p.here())

	return node, nil
}

// parseUnaryAtom parses the prefix atom forms "&X", "*X", and "<-X". The operand
// is a reference (an atom plus its suffix chain), so the prefix binds looser
// than member/index/call — matching Go (e.g. "*p.x" is "*(p.x)").
func (p *Parser) parseUnaryAtom(prefix tokenizer.Token) (ast.Node, error) {
	start := p.here()

	p.next() // consume the prefix

	operand, err := p.parseReference()
	if err != nil {
		return nil, err
	}

	var node ast.Node

	switch {
	case prefix.Is(tokenizer.AddressToken):
		n := &ast.AddrExpr{X: operand}
		n.SetSpan(start, operand.End())
		node = n
	case prefix.Is(tokenizer.PointerToken):
		n := &ast.StarExpr{X: operand}
		n.SetSpan(start, operand.End())
		node = n
	default: // ChannelReceiveToken
		n := &ast.RecvExpr{X: operand}
		n.SetSpan(start, operand.End())
		node = n
	}

	return node, nil
}

// parseBracketAtom parses the "[" -led atom forms (SYNTAX.md: arrayLiteral):
//
//	"[" "]" typeSpec          → a slice type (a "{...}" suffix makes an array literal)
//	"[" ":" expr "]"          → range literal, low omitted
//	"[" expr ":" expr "]"     → range literal
//	"[" expressionList "]"    → inferred-type array literal
func (p *Parser) parseBracketAtom() (ast.Node, error) {
	start := p.here()

	p.next() // consume "["

	// "[]T" — a slice type. Return the type; a following "{...}" becomes an
	// array composite literal via the reference-suffix loop.
	if p.accept(tokenizer.EndOfArrayToken) {
		elem, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		node := &ast.SliceType{Elem: elem}
		node.SetSpan(start, elem.End())

		return node, nil
	}

	savedLev := p.exprLev
	p.exprLev++

	defer func() { p.exprLev = savedLev }()

	// "[:high]" — range with omitted low.
	if p.accept(tokenizer.ColonToken) {
		high, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if err := p.expect(tokenizer.EndOfArrayToken, errors.ErrMissingBracket); err != nil {
			return nil, err
		}

		node := &ast.RangeLit{High: high}
		node.SetSpan(start, p.here())

		return node, nil
	}

	first, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	// "[low:high]" — range literal.
	if p.accept(tokenizer.ColonToken) {
		high, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if err := p.expect(tokenizer.EndOfArrayToken, errors.ErrMissingBracket); err != nil {
			return nil, err
		}

		node := &ast.RangeLit{Low: first, High: high}
		node.SetSpan(start, p.here())

		return node, nil
	}

	// "[e1, e2, ...]" — inferred-type array literal.
	elts := []ast.Node{first}

	for p.accept(tokenizer.CommaToken) {
		if p.at(tokenizer.EndOfArrayToken) {
			break
		}

		elt, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		elts = append(elts, elt)
	}

	if err := p.expect(tokenizer.EndOfArrayToken, errors.ErrMissingBracket); err != nil {
		return nil, err
	}

	node := &ast.CompositeLit{Composite: ast.CompositeArray, Elts: elts}
	node.SetSpan(start, p.here())

	return node, nil
}

// parseIfExpr parses the extension "if Cond { Then } else { Else }" expression
// atom (SYNTAX.md: ifExpression).
func (p *Parser) parseIfExpr() (ast.Node, error) {
	start := p.here()

	p.next() // consume "if"

	// The condition must not swallow the "{" that opens the then-branch.
	savedLev := p.exprLev
	p.exprLev = -1

	cond, err := p.parseExpression()
	p.exprLev = savedLev

	if err != nil {
		return nil, err
	}

	then, err := p.parseBracedExpr()
	if err != nil {
		return nil, err
	}

	if err := p.expect(tokenizer.ElseToken, errors.ErrUnexpectedToken); err != nil {
		return nil, err
	}

	elseExpr, err := p.parseBracedExpr()
	if err != nil {
		return nil, err
	}

	node := &ast.IfExpr{Cond: cond, Then: then, Else: elseExpr}
	node.SetSpan(start, elseExpr.End())

	return node, nil
}

// parseBracedExpr parses "{ expr }" as used by the if-expression branches.
func (p *Parser) parseBracedExpr() (ast.Node, error) {
	if err := p.expect(tokenizer.DataBeginToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	savedLev := p.exprLev
	p.exprLev++

	inner, err := p.parseExpression()
	p.exprLev = savedLev

	if err != nil {
		return nil, err
	}

	if err := p.expect(tokenizer.DataEndToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	return inner, nil
}

// parseOptionalExpr parses the extension "? X : Default" atom (SYNTAX.md:
// optionalExpression). Both operands are unary expressions.
func (p *Parser) parseOptionalExpr() (ast.Node, error) {
	start := p.here()

	p.next() // consume "?"

	x, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	if err := p.expect(tokenizer.ColonToken, errors.ErrMissingColon); err != nil {
		return nil, err
	}

	def, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	node := &ast.OptionalExpr{X: x, Default: def}
	node.SetSpan(start, def.End())

	return node, nil
}

// parseMacroAtom parses a directive/macro invocation used in expression
// position (SYNTAX.md: macroInvocation). The AST models it as a DirectiveStmt
// node (which also satisfies ast.Node in expression contexts); its Name is the
// macro identifier and RawArgs collects the remaining token spellings on the
// line, since macro arguments are arbitrary token sequences.
func (p *Parser) parseMacroAtom() (ast.Node, error) {
	start := p.here()

	p.next() // consume "@"

	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	p.next()

	node := &ast.DirectiveStmt{Name: nameTok.Spelling()}
	node.RawArgs = p.collectRawArgsToEndOfStatement()
	node.SetSpan(start, p.here())

	return node, nil
}

// collectRawArgsToEndOfStatement consumes and returns the spellings of every
// token up to the next statement boundary. It is used for directive/macro
// arguments that are not ordinary expressions.
func (p *Parser) collectRawArgsToEndOfStatement() []string {
	var raw []string

	for !p.isEndOfStatement() && !p.t.AtEnd() {
		raw = append(raw, p.next().Spelling())
	}

	return raw
}
