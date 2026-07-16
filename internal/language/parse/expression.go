package parse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file implements the expression grammar of docs/SYNTAX.md section 9,
// plus the type-assertion and channel-receive forms of sections 14 and 15.
//
// The binary-operator layers of the grammar (logicalOr / logicalAnd /
// relational / additive / multiplicative) are collapsed into a single
// precedence-climbing loop (parseBinary) driven by the binaryPrecedence table
// in tables.go. Everything at or above unary precedence is a recursive-descent
// function.

// parseExpression parses a full expression (SYNTAX.md: expression ::= logicalOr).
func (p *Parser) parseExpression() (ast.Node, error) {
	return p.parseBinary(lowestBinaryPrecedence)
}

// parseBinary implements precedence climbing. It parses a unary expression and
// then, while the next token is a binary operator whose precedence exceeds
// minPrec, consumes the operator and the right-hand operand at the appropriate
// precedence. Because every binary operator in Ego is left-associative, the
// right operand is parsed at (operator precedence + 1).
func (p *Parser) parseBinary(minPrec int) (ast.Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		op := p.t.Peek(1)
		prec, ok := binaryPrecedence[op.Spelling()]

		// Only treat a special-class token as an operator; this avoids, e.g.,
		// an identifier that happens to match a spelling being misread. Continue
		// while the operator binds at least as tightly as minPrec; the right
		// operand is parsed at prec+1, giving left-associativity.
		if !ok || !op.IsClass(tokenizer.SpecialTokenClass) || prec < minPrec {
			return left, nil
		}

		start := left.Pos()
		p.next()

		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}

		bin := &ast.BinaryExpr{Op: op.Spelling(), X: left, Y: right}
		bin.SetSpan(start, right.End())
		left = bin
	}
}

// parseUnary handles the prefix unary operators "-" and "!" (SYNTAX.md: unary),
// then delegates to the reference/postfix layer.
func (p *Parser) parseUnary() (ast.Node, error) {
	tok := p.t.Peek(1)

	if tok.Is(tokenizer.SubtractToken) || tok.Is(tokenizer.NotToken) {
		start := p.here()
		p.next()

		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}

		node := &ast.UnaryExpr{Op: tok.Spelling(), X: operand}
		node.SetSpan(start, operand.End())

		return node, nil
	}

	return p.parseReference()
}

// parseReference parses an atom followed by any chain of reference suffixes:
// member access, type assertion, index, slice, call, and struct initializer
// (SYNTAX.md: reference / refSuffix). This is also where funcOrRef's trailing
// call arguments are handled, since a call is just a "(" suffix.
func (p *Parser) parseReference() (ast.Node, error) {
	expr, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.t.Peek(1)

		switch {
		case tok.Is(tokenizer.DotToken):
			expr, err = p.parseSelectorOrAssert(expr)
		case tok.Is(tokenizer.StartOfArrayToken):
			expr, err = p.parseIndexOrSlice(expr)
		case tok.Is(tokenizer.StartOfListToken):
			expr, err = p.parseCallSuffix(expr)
		case (tok.Is(tokenizer.DataBeginToken) || tok.Is(tokenizer.EmptyBlockToken)) &&
			p.exprLev >= 0 && compositeLitAllowed(expr):
			// Both "{" (a non-empty body, DataBeginToken) and the crushed "{}"
			// empty-composite token can suffix a type/name to form a composite
			// literal. parseCompositeBody handles the "{}" case.
			expr, err = p.parseCompositeSuffix(expr)
		default:
			return expr, nil
		}

		if err != nil {
			return nil, err
		}
	}
}

// compositeLitAllowed reports whether a "{" following expr should be read as a
// composite-literal suffix. Only a type-like or name-like operand can be the
// type of a composite literal; this guards against, e.g., misreading the body
// of a bare block.
func compositeLitAllowed(expr ast.Node) bool {
	switch expr.Kind() {
	case ast.KindIdent, ast.KindSelectorExpr, ast.KindQualifiedType, ast.KindNamedType,
		ast.KindSliceType, ast.KindMapType, ast.KindPrimitiveType, ast.KindStructType:
		return true
	default:
		return false
	}
}

// parseSelectorOrAssert handles the ".member" and ".(Type)" suffixes.
func (p *Parser) parseSelectorOrAssert(x ast.Node) (ast.Node, error) {
	p.next() // consume "."

	// Type assertion: ".(" Type ")".
	if p.at(tokenizer.StartOfListToken) {
		p.next()

		typ, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
			return nil, err
		}

		node := &ast.TypeAssertExpr{X: x, Type: typ}
		node.SetSpan(x.Pos(), p.here())

		return node, nil
	}

	// Member access: "." IDENTIFIER.
	nameTok := p.t.Peek(1)
	if !nameTok.IsIdentifier() {
		return nil, p.errorHere(errors.ErrInvalidIdentifier)
	}

	p.next()

	sel := &ast.Ident{Name: nameTok.Spelling()}
	sel.SetSpan(tokenPos(nameTok), tokenEnd(nameTok))

	node := &ast.SelectorExpr{X: x, Sel: sel}
	node.SetSpan(x.Pos(), sel.End())

	return node, nil
}

// parseIndexOrSlice handles the "[index]" and "[low:high]" suffixes.
func (p *Parser) parseIndexOrSlice(x ast.Node) (ast.Node, error) {
	start := x.Pos()

	p.next() // consume "["

	savedLev := p.exprLev
	p.exprLev++

	defer func() { p.exprLev = savedLev }()

	var low ast.Node

	// "[:high]" form — low omitted.
	if !p.at(tokenizer.ColonToken) {
		var err error

		low, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	// Slice form: "[ low : high ]".
	if p.accept(tokenizer.ColonToken) {
		var high ast.Node

		if !p.at(tokenizer.EndOfArrayToken) {
			var err error

			high, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}

		if err := p.expect(tokenizer.EndOfArrayToken, errors.ErrMissingBracket); err != nil {
			return nil, err
		}

		node := &ast.SliceExpr{X: x, Low: low, High: high}
		node.SetSpan(start, p.here())

		return node, nil
	}

	// Index form: "[ index ]".
	if err := p.expect(tokenizer.EndOfArrayToken, errors.ErrMissingBracket); err != nil {
		return nil, err
	}

	node := &ast.IndexExpr{X: x, Index: low}
	node.SetSpan(start, p.here())

	return node, nil
}

// parseCallSuffix handles the "(args)" call suffix, including a trailing "..."
// on the final argument.
func (p *Parser) parseCallSuffix(fun ast.Node) (ast.Node, error) {
	start := fun.Pos()

	p.next() // consume "("

	savedLev := p.exprLev
	p.exprLev++

	args, ellipsis, err := p.parseArgList()
	p.exprLev = savedLev

	if err != nil {
		return nil, err
	}

	if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
		return nil, err
	}

	node := &ast.CallExpr{Fun: fun, Args: args, Ellipsis: ellipsis}
	node.SetSpan(start, p.here())

	return node, nil
}

// parseArgList parses a possibly-empty comma-separated argument list. It
// returns the arguments, whether the final argument had a "..." spread suffix,
// and any error. The closing ")" is not consumed.
func (p *Parser) parseArgList() ([]ast.Node, bool, error) {
	var (
		args     []ast.Node
		ellipsis bool
	)

	if p.at(tokenizer.EndOfListToken) {
		return nil, false, nil
	}

	for {
		arg, err := p.parseExpression()
		if err != nil {
			return nil, false, err
		}

		if p.accept(tokenizer.VariadicToken) {
			ellipsis = true
		}

		args = append(args, arg)

		if !p.accept(tokenizer.CommaToken) {
			break
		}

		// Allow a trailing comma before ")".
		if p.at(tokenizer.EndOfListToken) {
			break
		}
	}

	return args, ellipsis, nil
}

// parseCompositeSuffix handles the "{ ... }" composite-literal suffix on a
// type or name operand (struct/array/map element list).
func (p *Parser) parseCompositeSuffix(typ ast.Node) (ast.Node, error) {
	elts, err := p.parseCompositeBody()
	if err != nil {
		return nil, err
	}

	node := &ast.CompositeLit{Composite: ast.CompositeUnknown, Type: typ, Elts: elts}
	node.SetSpan(typ.Pos(), p.here())

	return node, nil
}

// parseCompositeBody parses the "{ elements }" body of a composite literal. Each
// element is either a "key: value" pair (KeyValueExpr) or a bare expression.
// The opening "{" is consumed here.
func (p *Parser) parseCompositeBody() ([]ast.Node, error) {
	// The tokenizer crushes "{}" into a single empty-block token.
	if p.accept(tokenizer.EmptyBlockToken) {
		return nil, nil
	}

	if err := p.expect(tokenizer.DataBeginToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	savedLev := p.exprLev
	p.exprLev++

	defer func() { p.exprLev = savedLev }()

	var elts []ast.Node

	for !p.at(tokenizer.DataEndToken) && !p.t.AtEnd() {
		elt, err := p.parseCompositeElement()
		if err != nil {
			return nil, err
		}

		elts = append(elts, elt)

		if !p.accept(tokenizer.CommaToken) {
			break
		}
	}

	if err := p.expect(tokenizer.DataEndToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	return elts, nil
}

// parseCompositeElement parses one composite-literal element, which is either
// "key: value" or a plain expression. A nested "{ ... }" value (an initializer
// without an explicit type) is parsed as a type-less composite literal.
func (p *Parser) parseCompositeElement() (ast.Node, error) {
	// A brace-delimited value with no leading type (nested initializer).
	if p.at(tokenizer.DataBeginToken) || p.at(tokenizer.EmptyBlockToken) {
		return p.parseUntypedComposite()
	}

	key, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.accept(tokenizer.ColonToken) {
		var value ast.Node

		if p.at(tokenizer.DataBeginToken) || p.at(tokenizer.EmptyBlockToken) {
			value, err = p.parseUntypedComposite()
		} else {
			value, err = p.parseExpression()
		}

		if err != nil {
			return nil, err
		}

		node := &ast.KeyValueExpr{Key: key, Value: value}
		node.SetSpan(key.Pos(), value.End())

		return node, nil
	}

	return key, nil
}

// parseUntypedComposite parses a "{ ... }" initializer that has no explicit
// type (its type is inferred from context, e.g. a nested struct value in an
// outer composite literal).
func (p *Parser) parseUntypedComposite() (ast.Node, error) {
	start := p.here()

	elts, err := p.parseCompositeBody()
	if err != nil {
		return nil, err
	}

	node := &ast.CompositeLit{Composite: ast.CompositeUnknown, Elts: elts}
	node.SetSpan(start, p.here())

	return node, nil
}

// tokenPos returns the AST position of a token.
func tokenPos(tok tokenizer.Token) ast.Position {
	line, col := tok.Location()

	return ast.Position{Line: line, Column: col}
}

// tokenEnd returns the AST position just past a token, approximated as the end
// of its spelling on the same line.
func tokenEnd(tok tokenizer.Token) ast.Position {
	line, col := tok.Location()

	return ast.Position{Line: line, Column: col + len(tok.Spelling())}
}
