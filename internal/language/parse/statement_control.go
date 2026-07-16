package parse

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file implements the control-flow statements (SYNTAX.md section 10) and
// the extension statements (section 11). Each function is the target of an
// entry in the statementKeywords dispatch table.

// parseIf parses an if statement (SYNTAX.md: ifStmt).
func (p *Parser) parseIf() (ast.Node, error) {
	start := p.here()

	p.next() // consume "if"

	node := &ast.IfStmt{}

	// Optional init statement followed by ";". We detect it by parsing a simple
	// statement and seeing whether a ";" follows; if not, what we parsed is the
	// condition expression. To keep the two cases clean we look ahead for a
	// ";" at statement-header depth.
	init, cond, err := p.parseIfHeader()
	if err != nil {
		return nil, err
	}

	node.Init = init
	node.Cond = cond

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	node.Body = body

	// Optional else / else-if.
	if p.accept(tokenizer.ElseToken) {
		if p.at(tokenizer.IfToken) {
			elseIf, err := p.parseIf()
			if err != nil {
				return nil, err
			}

			node.Else = elseIf
		} else {
			elseBlock, err := p.parseBlock()
			if err != nil {
				return nil, err
			}

			node.Else = elseBlock
		}
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseIfHeader parses the "[init;] cond" portion of an if statement. Composite
// literals are suppressed (exprLev = -1) so the trailing "{" opens the body.
// Like Go, the header is one or two clauses separated by ";": parse the first
// clause as a simple statement; if a ";" follows it was the init and the
// condition is the next clause, otherwise the first clause is the condition.
func (p *Parser) parseIfHeader() (initStmt ast.Node, cond ast.Node, err error) {
	saved := p.exprLev
	p.exprLev = -1

	defer func() { p.exprLev = saved }()

	first, err := p.parseSimpleStatement()
	if err != nil {
		return nil, nil, err
	}

	if p.accept(tokenizer.SemicolonToken) {
		initStmt = first

		cond, err = p.parseExpression()
		if err != nil {
			return nil, nil, err
		}

		return initStmt, cond, nil
	}

	// No ";" — the single clause is the condition expression.
	if expr, ok := exprStmtValue(first); ok {
		return nil, expr, nil
	}

	return nil, first, nil
}

// exprStmtValue returns the underlying expression of an *ast.ExprStmt, reporting
// whether the node was in fact an expression statement. It is used to turn a
// parsed simple statement back into a bare expression when a control-flow
// header clause is a condition/tag rather than an init statement.
func exprStmtValue(node ast.Node) (ast.Node, bool) {
	if es, ok := node.(*ast.ExprStmt); ok {
		return es.X, true
	}

	return nil, false
}

// headerHasInit reports whether the control-flow header at the cursor contains
// an init clause, i.e. a ";" appears before the opening "{" of the body (at
// brace/paren depth zero).
func (p *Parser) headerHasInit() bool {
	depth := 0

	for offset := 1; ; offset++ {
		tok := p.t.Peek(offset)

		switch {
		case tok.Is(tokenizer.EndOfTokens):
			return false
		case tok.Is(tokenizer.StartOfListToken), tok.Is(tokenizer.StartOfArrayToken):
			depth++
		case tok.Is(tokenizer.EndOfListToken), tok.Is(tokenizer.EndOfArrayToken):
			depth--
		case depth == 0 && (tok.Is(tokenizer.BlockBeginToken) || tok.Is(tokenizer.EmptyBlockToken)):
			return false
		case depth == 0 && tok.Is(tokenizer.SemicolonToken):
			return true
		}
	}
}

// parseFor parses all four for-loop forms (SYNTAX.md: forStmt).
func (p *Parser) parseFor() (ast.Node, error) {
	start := p.here()

	p.next() // consume "for"

	node := &ast.ForStmt{}

	saved := p.exprLev
	p.exprLev = -1

	// Infinite loop: "for { ... }".
	if p.at(tokenizer.BlockBeginToken) || p.at(tokenizer.EmptyBlockToken) {
		p.exprLev = saved

		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}

		node.Body = body
		node.SetSpan(start, body.End())

		return node, nil
	}

	if err := p.parseForHeader(node); err != nil {
		p.exprLev = saved

		return nil, err
	}

	p.exprLev = saved

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	node.Body = body
	node.SetSpan(start, body.End())

	return node, nil
}

// parseForHeader fills in the loop-control fields of node for the conditional,
// three-clause, and range forms.
func (p *Parser) parseForHeader(node *ast.ForStmt) error {
	// Range form: "for k [, v] := range expr" or "for k [, v] = range expr".
	if p.isRangeLoop() {
		return p.parseRangeHeader(node)
	}

	// Three-clause form has two ";" separators at depth zero.
	if p.headerHasInit() {
		init, err := p.parseSimpleStatement()
		if err != nil {
			return err
		}

		node.Init = init

		if err := p.expect(tokenizer.SemicolonToken, errors.ErrMissingSemicolon); err != nil {
			return err
		}

		if !p.at(tokenizer.SemicolonToken) {
			cond, err := p.parseExpression()
			if err != nil {
				return err
			}

			node.Cond = cond
		}

		if err := p.expect(tokenizer.SemicolonToken, errors.ErrMissingSemicolon); err != nil {
			return err
		}

		if !p.at(tokenizer.BlockBeginToken) && !p.at(tokenizer.EmptyBlockToken) {
			post, err := p.parseSimpleStatement()
			if err != nil {
				return err
			}

			node.Post = post
		}

		return nil
	}

	// Conditional form: "for cond { ... }".
	cond, err := p.parseExpression()
	if err != nil {
		return err
	}

	node.Cond = cond

	return nil
}

// isRangeLoop reports whether the for header is a range loop by scanning for the
// "range" keyword before the body "{" at depth zero.
func (p *Parser) isRangeLoop() bool {
	depth := 0

	for offset := 1; ; offset++ {
		tok := p.t.Peek(offset)

		switch {
		case tok.Is(tokenizer.EndOfTokens):
			return false
		case tok.Is(tokenizer.StartOfListToken), tok.Is(tokenizer.StartOfArrayToken):
			depth++
		case tok.Is(tokenizer.EndOfListToken), tok.Is(tokenizer.EndOfArrayToken):
			depth--
		case depth == 0 && (tok.Is(tokenizer.BlockBeginToken) || tok.Is(tokenizer.EmptyBlockToken)):
			return false
		case depth == 0 && tok.Is(tokenizer.RangeToken):
			return true
		}
	}
}

// parseRangeHeader parses "k [, v] (:=|=) range expr".
func (p *Parser) parseRangeHeader(node *ast.ForStmt) error {
	key, err := p.parseExpression()
	if err != nil {
		return err
	}

	node.Key = key

	if p.accept(tokenizer.CommaToken) {
		value, err := p.parseExpression()
		if err != nil {
			return err
		}

		node.Value = value
	}

	if p.accept(tokenizer.DefineToken) {
		node.Define = true
	} else if !p.accept(tokenizer.AssignToken) {
		return p.errorHere(errors.ErrMissingEqual)
	}

	if !p.accept(tokenizer.RangeToken) {
		return p.errorHere(errors.ErrUnexpectedToken)
	}

	rangeExpr, err := p.parseExpression()
	if err != nil {
		return err
	}

	node.Range = rangeExpr

	return nil
}

// parseSwitch parses a switch statement (SYNTAX.md: switchStmt).
func (p *Parser) parseSwitch() (ast.Node, error) {
	start := p.here()

	p.next() // consume "switch"

	node := &ast.SwitchStmt{}

	saved := p.exprLev
	p.exprLev = -1

	// Parse the switch header. Following Go, it is up to two clauses separated
	// by ";": an optional init statement and an optional tag. Each clause may be
	// a full simple statement (e.g. the type-switch guard "v := x.(type)") or a
	// bare expression. We parse clause 1; if a ";" follows it was the init and
	// clause 2 is the tag; otherwise clause 1 itself is the tag (or, for an
	// assignment form like "v := x.(type)", the init with no separate tag).
	if !p.at(tokenizer.BlockBeginToken) && !p.at(tokenizer.EmptyBlockToken) {
		first, err := p.parseSimpleStatement()
		if err != nil {
			p.exprLev = saved

			return nil, err
		}

		if p.accept(tokenizer.SemicolonToken) {
			node.Init = first

			if !p.at(tokenizer.BlockBeginToken) && !p.at(tokenizer.EmptyBlockToken) {
				second, err := p.parseSimpleStatement()
				if err != nil {
					p.exprLev = saved

					return nil, err
				}

				if expr, ok := exprStmtValue(second); ok {
					node.Tag = expr
				} else {
					node.Tag = second
				}
			}
		} else if expr, ok := exprStmtValue(first); ok {
			node.Tag = expr
		} else {
			// An assignment guard such as "v := x.(type)" with no separate tag.
			node.Init = first
		}
	}

	p.exprLev = saved

	// An empty switch body.
	if p.accept(tokenizer.EmptyBlockToken) {
		node.SetSpan(start, p.here())

		return node, nil
	}

	if err := p.expect(tokenizer.BlockBeginToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	for !p.at(tokenizer.DataEndToken) && !p.t.AtEnd() {
		if p.t.AnyNext(tokenizer.SemicolonToken) {
			continue
		}

		if p.at(tokenizer.DataEndToken) {
			break
		}

		clause, err := p.parseCaseClause()
		if err != nil {
			return nil, err
		}

		node.Body = append(node.Body, clause)
	}

	if err := p.expect(tokenizer.DataEndToken, errors.ErrMissingBlock); err != nil {
		return nil, err
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseCaseClause parses one "case exprs:" or "default:" clause (SYNTAX.md:
// caseClause / defaultClause).
func (p *Parser) parseCaseClause() (*ast.CaseClause, error) {
	start := p.here()

	clause := &ast.CaseClause{}

	switch {
	case p.accept(tokenizer.DefaultToken):
		clause.Default = true
	case p.accept(tokenizer.CaseToken):
		exprs, err := p.parseExprList()
		if err != nil {
			return nil, err
		}

		clause.Exprs = exprs
	default:
		return nil, p.errorHere(errors.ErrUnexpectedToken)
	}

	if err := p.expect(tokenizer.ColonToken, errors.ErrMissingColon); err != nil {
		return nil, err
	}

	// Case body: statements until the next case/default/"}".
	for !p.at(tokenizer.CaseToken) && !p.at(tokenizer.DefaultToken) &&
		!p.at(tokenizer.DataEndToken) && !p.t.AtEnd() {
		if p.t.AnyNext(tokenizer.SemicolonToken) {
			continue
		}

		if p.at(tokenizer.FallthroughToken) {
			p.next()

			clause.Fallthrough = true
			p.accept(tokenizer.SemicolonToken)

			break
		}

		if p.at(tokenizer.CaseToken) || p.at(tokenizer.DefaultToken) || p.at(tokenizer.DataEndToken) {
			break
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}

		clause.Body = append(clause.Body, stmt)
	}

	clause.SetSpan(start, p.here())

	return clause, nil
}

// parseReturn parses "return [exprs]" (SYNTAX.md: returnStmt).
func (p *Parser) parseReturn() (ast.Node, error) {
	start := p.here()

	p.next() // consume "return"

	node := &ast.ReturnStmt{}

	if !p.isEndOfStatement() && !p.at(tokenizer.DataEndToken) {
		results, err := p.parseExprList()
		if err != nil {
			return nil, err
		}

		node.Results = results
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseBreak parses "break [label]" (SYNTAX.md: breakStmt).
func (p *Parser) parseBreak() (ast.Node, error) {
	start := p.here()

	p.next() // consume "break"

	node := &ast.BreakStmt{}

	if p.t.Peek(1).IsIdentifier() && !p.isEndOfStatement() {
		node.Label = p.next().Spelling()
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseContinue parses "continue [label]" (SYNTAX.md: continueStmt).
func (p *Parser) parseContinue() (ast.Node, error) {
	start := p.here()

	p.next() // consume "continue"

	node := &ast.ContinueStmt{}

	if p.t.Peek(1).IsIdentifier() && !p.isEndOfStatement() {
		node.Label = p.next().Spelling()
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseDefer parses "defer call" (SYNTAX.md: deferStmt).
func (p *Parser) parseDefer() (ast.Node, error) {
	start := p.here()

	p.next() // consume "defer"

	call, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	node := &ast.DeferStmt{Call: call}
	node.SetSpan(start, call.End())

	return node, nil
}

// parseGo parses "go call" (SYNTAX.md: goStmt).
func (p *Parser) parseGo() (ast.Node, error) {
	start := p.here()

	p.next() // consume "go"

	call, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	node := &ast.GoStmt{Call: call}
	node.SetSpan(start, call.End())

	return node, nil
}

// parseTry parses "try block [catch [(ident)] block]" (SYNTAX.md: tryStmt).
func (p *Parser) parseTry() (ast.Node, error) {
	start := p.here()

	p.next() // consume "try"

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	node := &ast.TryStmt{Body: body}

	if p.accept(tokenizer.CatchToken) {
		if p.accept(tokenizer.StartOfListToken) {
			nameTok := p.t.Peek(1)
			if !nameTok.IsIdentifier() {
				return nil, p.errorHere(errors.ErrInvalidIdentifier)
			}

			node.CatchVar = p.next().Spelling()

			if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
				return nil, err
			}
		}

		catch, err := p.parseBlock()
		if err != nil {
			return nil, err
		}

		node.Catch = catch
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parsePanicStmt parses "panic(expr)" (SYNTAX.md: panicStmt).
func (p *Parser) parsePanicStmt() (ast.Node, error) {
	start := p.here()

	p.next() // consume "panic"

	if err := p.expect(tokenizer.StartOfListToken, errors.ErrMissingParenthesis); err != nil {
		return nil, err
	}

	node := &ast.PanicStmt{}

	if !p.at(tokenizer.EndOfListToken) {
		arg, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		node.Arg = arg
	}

	if err := p.expect(tokenizer.EndOfListToken, errors.ErrMissingParenthesis); err != nil {
		return nil, err
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parsePrint parses "print [exprs] [,]" (SYNTAX.md: printStmt).
func (p *Parser) parsePrint() (ast.Node, error) {
	start := p.here()

	p.next() // consume "print"

	node := &ast.PrintStmt{}

	for !p.isEndOfStatement() && !p.at(tokenizer.DataEndToken) {
		arg, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		node.Args = append(node.Args, arg)

		if !p.accept(tokenizer.CommaToken) {
			break
		}

		// A trailing comma suppresses the newline.
		if p.isEndOfStatement() || p.at(tokenizer.DataEndToken) {
			node.NoNewline = true

			break
		}
	}

	node.SetSpan(start, p.here())

	return node, nil
}

// parseCall parses "call funcCall" (SYNTAX.md: callStmt).
func (p *Parser) parseCall() (ast.Node, error) {
	start := p.here()

	p.next() // consume "call"

	call, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	node := &ast.CallStmt{Call: call}
	node.SetSpan(start, call.End())

	return node, nil
}

// parseThrow parses "throw expr" (SYNTAX.md: throwStmt).
func (p *Parser) parseThrow() (ast.Node, error) {
	start := p.here()

	p.next() // consume "throw"

	x, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	node := &ast.ThrowStmt{X: x}
	node.SetSpan(start, x.End())

	return node, nil
}

// parseExit parses "exit [expr]" (SYNTAX.md: exitStmt).
func (p *Parser) parseExit() (ast.Node, error) {
	start := p.here()

	p.next() // consume "exit"

	node := &ast.ExitStmt{}

	if !p.isEndOfStatement() && !p.at(tokenizer.DataEndToken) {
		code, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		node.Code = code
	}

	node.SetSpan(start, p.here())

	return node, nil
}
