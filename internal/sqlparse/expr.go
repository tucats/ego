package sqlparse

import (
	"strings"

	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file implements expression parsing — everything that can appear
// inside a WHERE clause, a SELECT column, a DEFAULT value, and so on: "a +
// b * c", "x BETWEEN 1 AND 10", "name LIKE 'a%'", function calls, CASE,
// nested subqueries.
//
// # The problem: operator precedence and associativity
//
// "1 + 2 * 3" must parse as "1 + (2 * 3)", not "(1 + 2) * 3" — multiplication
// binds tighter than addition. That's operator *precedence*. Separately,
// "10 - 3 - 2" must parse as "(10 - 3) - 2", not "10 - (3 - 2)" — operators
// at the same precedence, read left to right, group to the left. That's
// *associativity*. A naive recursive parser that just alternates "parse an
// operand, then an operator, then an operand" in one flat loop gets both of
// these wrong. This file uses the standard fix for a hand-written parser,
// usually called "precedence climbing".
//
// # The fix: one function per precedence level, ordered tightest-to-loosest
//
// Below, every precedence tier sqlite3 documents (see
// https://www.sqlite.org/lang_expr.html; PostgreSQL agrees on every operator
// the two dialects share) gets its own parsing function, and the functions
// are chained so each one's operands come from the *next tighter-binding*
// function:
//
//	parseExpr             (entry point)
//	  parseOrExpr          OR                        loosest — parsed last, so it ends up outermost
//	    parseAndExpr        AND
//	      parseNotExpr       NOT (prefix)
//	        parseComparisonExpr  = < > <= >= <> != IS IN LIKE BETWEEN ...
//	          parseBitOrExpr      << >> & |
//	            parseAdditiveExpr  + -
//	              parseMultiplicativeExpr  * / %
//	                parseConcatExpr  || -> ->>
//	                  parseUnaryExpr  unary - + ~ (prefix)
//	                    parseCollateExpr  COLLATE (postfix)
//	                      parsePrimary      tightest — literals, names, (...), CASE, CAST, ...
//
// Every one of these functions (parseOrExpr through parseConcatExpr) shares
// the same two-step shape, which is worth internalizing once so the
// individual functions below don't need their own repeated explanation:
//
//  1. Call the next function down the chain to parse a single operand.
//  2. Loop: as long as the current token is an operator belonging to *this*
//     tier, consume it, call the next function down again for the
//     right-hand operand, and wrap the operand parsed so far and the new
//     one in a BinaryExpr — then keep looping, using that BinaryExpr as the
//     new left-hand operand.
//
// Step 1 is what gives you correct precedence: parseAdditiveExpr never sees
// a "*" directly, because by the time control reaches it, parseConcatExpr
// (called via step 1) has already consumed and grouped any "*" into a
// single operand for parseAdditiveExpr to treat as one thing. Step 2's loop
// (rather than a recursive call for the right-hand side) is what gives you
// left-associativity: "10 - 3 - 2" builds first (10 - 3), then loops around
// and builds (that) - 2, so the earlier subtraction ends up nested on the
// *left*. Every BinaryExpr's position span is set with n.SetSpan(start,
// right.End()), where start is the left operand's own start position — so
// the span always covers the whole expression built so far, not just the
// newest operator.
//
// Two tiers below don't fit that shared shape, and it's worth knowing why
// before you reach them: parseNotExpr and parseUnaryExpr parse *prefix*
// operators (NOT, and unary - + ~), which are right-associative — "NOT NOT
// x" means "NOT (NOT x)" — so instead of the loop-and-wrap pattern above,
// they recurse into *themselves* for the operand, so nested prefixes nest
// correctly without a loop. And parseComparisonExpr's tier is unusually
// crowded: SQL packs relational operators, IS, IN, BETWEEN, and the LIKE
// family all into one precedence level, so its loop has many cases instead
// of the usual one small operator table — see its own comment below.
//
// parsePrimary, at the bottom, is the base case of the whole recursion: it
// has no further function to delegate to, and instead directly recognizes
// the "atoms" of an expression — a literal, a placeholder, a column
// reference, a function call, a parenthesized group, CASE, CAST, EXISTS —
// each of which may itself contain a full expression again (a function
// argument, a CASE branch, ...), which is why it and its helpers call all
// the way back up to parseExpr for those nested pieces. That's the
// "descent" and the "return" of recursive descent in miniature, entirely
// within this one file.

// parseExpr is the single entry point every other file in this package uses
// to parse an expression (a WHERE condition, a DEFAULT value, a function
// argument, ...). It simply starts the precedence chain at its loosest
// (outermost) tier — see the file comment above for the full chain.
func (p *parser) parseExpr() (ast.Node, error) {
	return p.parseOrExpr()
}

// parseOrExpr is the loosest-binding tier: logical OR. See the file comment
// above for the "parse one operand, then loop consuming same-tier operators"
// shape this and the next several functions share.
func (p *parser) parseOrExpr() (ast.Node, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}

	for p.isKeyword("or") {
		start := left.Pos()

		p.next()

		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.BinaryExpr{Op: "OR", X: left, Y: right}
		n.SetSpan(start, right.End())
		left = n
	}

	return left, nil
}

// parseAndExpr is the AND tier, one notch tighter-binding than OR.
func (p *parser) parseAndExpr() (ast.Node, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}

	for p.isKeyword("and") {
		start := left.Pos()

		p.next()

		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.BinaryExpr{Op: "AND", X: left, Y: right}
		n.SetSpan(start, right.End())
		left = n
	}

	return left, nil
}

// parseNotExpr handles the unary "NOT expr" prefix, except when NOT
// immediately precedes EXISTS — that combination is parsed as a single
// ExistsExpr with Not set, by parseUnaryExpr's caller chain reaching
// parsePrimary.
//
// Unlike its neighbors, this isn't a "parse one operand, then loop"
// function (see the file comment above) — NOT is a *prefix* operator, and
// SQL treats repeated NOTs as right-associative ("NOT NOT x" is "NOT (NOT
// x)"), so this recurses into itself for the operand instead of looping.
// parseUnaryExpr below does the same thing for unary -, +, and ~.
func (p *parser) parseNotExpr() (ast.Node, error) {
	if p.isKeyword("not") && !p.isKeywordAt(1, "exists") {
		start := p.here()

		p.next()

		x, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.UnaryExpr{Op: "NOT", X: x}
		n.SetSpan(start, x.End())

		return n, nil
	}

	return p.parseComparisonExpr()
}

// relOps lists the relational operator spellings recognized at this tier.
// Unlike lexer.go's multiCharOps, order doesn't matter here: by the time
// parseComparisonExpr runs, the lexer has already turned e.g. "<=" into one
// indivisible token (see lexer.go's own longest-match handling), so relOpAt
// only needs to check "is this token's text one of these", not worry about
// one spelling being a prefix of another.
var relOps = []string{"<=", ">=", "<>", "!=", "==", "=", "<", ">"}

// relOpAt reports whether t is one of the relational operators, returning
// its text for use as BinaryExpr.Op.
func relOpAt(t token) (string, bool) {
	if t.kind != tokOp {
		return "", false
	}

	for _, op := range relOps {
		if t.text == op {
			return op, true
		}
	}

	return "", false
}

var likeFamily = []string{"like", "glob", "regexp", "match", "ilike"}

// isLikeFamily reports whether the current token is one of the LIKE-family
// keywords (LIKE, GLOB, REGEXP, MATCH, ILIKE), all of which share one
// grammar shape: "X [NOT] op Pattern [ESCAPE Escape]".
func (p *parser) isLikeFamily() bool {
	for _, w := range likeFamily {
		if p.isKeyword(w) {
			return true
		}
	}

	return false
}

// parseComparisonExpr is the comparison tier — but "comparison" undersells
// it: SQL groups relational operators (=, <, ...), IS [NOT] [NULL|DISTINCT
// FROM ...], [NOT] BETWEEN, [NOT] IN, and the LIKE family all into this one
// precedence level, which is why this function's loop has many cases
// instead of the one small operator table its neighbors have. Each
// construct still follows the shared "parse operand at the next tier down,
// then loop" shape from the file comment — there's just more than one kind
// of thing the loop can match here.
//
// The trickiest part is the four constructs that can be negated with a
// leading NOT written *between* the left operand and the keyword ("a NOT
// BETWEEN 1 AND 10", "a NOT IN (...)", "a NOT LIKE ...") rather than NOT
// being a prefix on the whole expression. Because this parser never
// backtracks (see "How this parser works" in parser.go), it can't just
// consume "NOT" and then discover there's no BETWEEN/IN/LIKE after it and
// give the token back — so it peeks ahead first with isKeywordAt to confirm
// one of those three keywords actually follows before committing to consume
// the NOT. The "if not { return ... syntaxError }" at the bottom of the
// loop is a safety net for a case that should be unreachable precisely
// because of that upfront check — see its own comment below.
func (p *parser) parseComparisonExpr() (ast.Node, error) {
	left, err := p.parseBitOrExpr()
	if err != nil {
		return nil, err
	}

	for {
		start := left.Pos()

		if op, ok := relOpAt(p.cur()); ok {
			p.next()

			right, err := p.parseBitOrExpr()
			if err != nil {
				return nil, err
			}

			n := &ast.BinaryExpr{Op: op, X: left, Y: right}
			n.SetSpan(start, right.End())
			left = n

			continue
		}

		if p.isKeyword("is") {
			p.next()

			not := p.acceptKeyword("not")

			switch {
			case p.isKeyword("null"):
				end := p.here()
				p.next()

				n := &ast.IsNullExpr{X: left, Not: not}
				n.SetSpan(start, end)
				left = n
			case p.isKeyword("distinct"):
				p.next()

				if err := p.expectKeyword("from"); err != nil {
					return nil, err
				}

				right, err := p.parseBitOrExpr()
				if err != nil {
					return nil, err
				}

				n := &ast.IsExpr{X: left, Not: not, Distinct: true, Y: right}
				n.SetSpan(start, right.End())
				left = n
			default:
				right, err := p.parseBitOrExpr()
				if err != nil {
					return nil, err
				}

				n := &ast.IsExpr{X: left, Not: not, Y: right}
				n.SetSpan(start, right.End())
				left = n
			}

			continue
		}

		if p.isKeyword("isnull") {
			end := p.here()
			p.next()

			n := &ast.IsNullExpr{X: left}
			n.SetSpan(start, end)
			left = n

			continue
		}

		if p.isKeyword("notnull") {
			end := p.here()
			p.next()

			n := &ast.IsNullExpr{X: left, Not: true}
			n.SetSpan(start, end)
			left = n

			continue
		}

		not := false
		if p.isKeyword("not") && (p.isKeywordAt(1, "between") || p.isKeywordAt(1, "in") || likeFamilyAt(p, 1)) {
			not = true

			p.next()
		}

		if p.isKeyword("between") {
			p.next()

			low, err := p.parseBitOrExpr()
			if err != nil {
				return nil, err
			}

			if err := p.expectKeyword("and"); err != nil {
				return nil, err
			}

			high, err := p.parseBitOrExpr()
			if err != nil {
				return nil, err
			}

			n := &ast.BetweenExpr{X: left, Not: not, Low: low, High: high}
			n.SetSpan(start, high.End())
			left = n

			continue
		}

		if p.isKeyword("in") {
			n, err := p.parseInExpr(left, not, start)
			if err != nil {
				return nil, err
			}

			left = n

			continue
		}

		if p.isLikeFamily() {
			opTok := p.next()
			op := strings.ToUpper(opTok.text)

			pattern, err := p.parseBitOrExpr()
			if err != nil {
				return nil, err
			}

			var escape ast.Node

			end := pattern.End()

			if p.acceptKeyword("escape") {
				escape, err = p.parseBitOrExpr()
				if err != nil {
					return nil, err
				}

				end = escape.End()
			}

			n := &ast.LikeExpr{X: left, Not: not, Op: op, Pattern: pattern, Escape: escape}
			n.SetSpan(start, end)
			left = n

			continue
		}

		if not {
			// A "NOT" was consumed above speculatively (only after confirming
			// one of BETWEEN/IN/the LIKE family follows), so reaching here
			// would be a parser bug rather than user input; report it as a
			// syntax error naming what was actually found.
			return nil, p.syntaxError("BETWEEN, IN, or LIKE")
		}

		return left, nil
	}
}

func likeFamilyAt(p *parser, offset int) bool {
	for _, w := range likeFamily {
		if p.isKeywordAt(offset, w) {
			return true
		}
	}

	return false
}

func (p *parser) parseInExpr(left ast.Node, not bool, start ast.Position) (ast.Node, error) {
	p.next() // "in"

	if err := p.expectPunct("("); err != nil {
		return nil, err
	}

	if p.selectFollows() {
		sel, err := p.parseSelectOrCompound()
		if err != nil {
			return nil, err
		}

		end := p.here()

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}

		sub := &ast.Subquery{Select: sel}
		sub.SetSpan(start, end)

		n := &ast.InExpr{X: left, Not: not, Sub: sub}
		n.SetSpan(start, end)

		return n, nil
	}

	var list []ast.Node

	if !p.cur().isOp(")") {
		for {
			item, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			list = append(list, item)

			if !p.acceptPunct(",") {
				break
			}
		}
	}

	end := p.here()

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	n := &ast.InExpr{X: left, Not: not, List: list}
	n.SetSpan(start, end)

	return n, nil
}

// selectFollows reports whether the current token starts a SELECT (or a
// WITH-prefixed SELECT), used to disambiguate "(SELECT ...)" from a plain
// parenthesized expression list.
func (p *parser) selectFollows() bool {
	return p.isKeyword("select") || p.isKeyword("with")
}

var bitOrOps = []string{"<<", ">>", "&", "|"}

// parseBitOrExpr is the bitwise shift/AND/OR tier.
func (p *parser) parseBitOrExpr() (ast.Node, error) {
	left, err := p.parseAdditiveExpr()
	if err != nil {
		return nil, err
	}

	for isOpIn(p.cur(), bitOrOps) {
		start := left.Pos()
		op := p.next().text

		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.BinaryExpr{Op: op, X: left, Y: right}
		n.SetSpan(start, right.End())
		left = n
	}

	return left, nil
}

func isOpIn(t token, ops []string) bool {
	if t.kind != tokOp {
		return false
	}

	for _, op := range ops {
		if t.text == op {
			return true
		}
	}

	return false
}

// parseAdditiveExpr is the + / - tier.
func (p *parser) parseAdditiveExpr() (ast.Node, error) {
	left, err := p.parseMultiplicativeExpr()
	if err != nil {
		return nil, err
	}

	for p.cur().isOp("+") || p.cur().isOp("-") {
		start := left.Pos()
		op := p.next().text

		right, err := p.parseMultiplicativeExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.BinaryExpr{Op: op, X: left, Y: right}
		n.SetSpan(start, right.End())
		left = n
	}

	return left, nil
}

// parseMultiplicativeExpr is the * / % tier.
func (p *parser) parseMultiplicativeExpr() (ast.Node, error) {
	left, err := p.parseConcatExpr()
	if err != nil {
		return nil, err
	}

	for p.cur().isOp("*") || p.cur().isOp("/") || p.cur().isOp("%") {
		start := left.Pos()
		op := p.next().text

		right, err := p.parseConcatExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.BinaryExpr{Op: op, X: left, Y: right}
		n.SetSpan(start, right.End())
		left = n
	}

	return left, nil
}

var concatOps = []string{"||", "->>", "->"}

// parseConcatExpr is the string-concatenation / JSON-arrow tier ("||", the
// PostgreSQL JSON operators "->" and "->>").
func (p *parser) parseConcatExpr() (ast.Node, error) {
	left, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}

	for isOpIn(p.cur(), concatOps) {
		start := left.Pos()
		op := p.next().text

		right, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.BinaryExpr{Op: op, X: left, Y: right}
		n.SetSpan(start, right.End())
		left = n
	}

	return left, nil
}

// parseUnaryExpr is the unary -, +, ~ tier — and also, perhaps
// surprisingly, where "NOT EXISTS" actually gets parsed. Here's the chain of
// reasoning: parseNotExpr (above, several tiers up) is deliberately written
// to *not* treat a "NOT" as its generic prefix operator when it's followed
// by EXISTS ("!p.isKeywordAt(1, "exists")" in its condition), so that
// "NOT EXISTS (...)" can fall all the way down through every tier in
// between — untouched — to be recognized here as a single unit and built
// directly into one ExistsExpr node with Not: true, rather than as a
// generic UnaryExpr{Op: "NOT"} wrapping a separate ExistsExpr. Both would
// be semantically fine, but the direct form is what parsePrimary's own
// plain "EXISTS (...)" case (with Not: false) produces too, so this way the
// two cases produce a symmetric, easy-to-consume tree shape instead of two
// different shapes for what's conceptually the same construct.
//
// Like parseNotExpr, the actual unary +/-/~ case recurses into itself
// rather than looping, since prefix operators are right-associative (see
// the file comment above).
func (p *parser) parseUnaryExpr() (ast.Node, error) {
	if p.isKeyword("not") && p.isKeywordAt(1, "exists") {
		start := p.here()

		p.next() // not
		p.next() // exists

		return p.parseExistsBody(true, start)
	}

	if p.cur().isOp("-") || p.cur().isOp("+") || p.cur().isOp("~") {
		start := p.here()
		op := p.next().text

		x, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}

		n := &ast.UnaryExpr{Op: op, X: x}
		n.SetSpan(start, x.End())

		return n, nil
	}

	return p.parseCollateExpr()
}

// parseCollateExpr is the COLLATE tier. Unlike every tier above it, COLLATE
// is a *postfix* keyword ("expr COLLATE name"), not an infix operator
// between two operands — so this loops re-wrapping the same operand in
// successive CollateExpr nodes (for the rare "x COLLATE a COLLATE b") rather
// than parsing a right-hand operand at each step.
func (p *parser) parseCollateExpr() (ast.Node, error) {
	x, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for p.acceptKeyword("collate") {
		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		n := &ast.CollateExpr{X: x, Collation: name}
		n.SetSpan(x.Pos(), p.here())
		x = n
	}

	return x, nil
}

// parsePrimary is the base case of the precedence chain (see the file
// comment above): it has no tighter tier to delegate to, and instead
// dispatches purely on what kind of token is under the cursor to recognize
// one of the "atomic" forms an expression can start with. Note this is a
// different dispatch style than parser.parseStatement's — that one only
// ever needs to look at one leading keyword, but here a expression can
// start with a wide variety of token kinds (an operator like "*", a
// literal, an identifier, "(", or one of several keywords), so the switch
// below mixes token-kind checks (t.kind == tokNumber) and keyword-text
// checks (t.is("case")) as needed case by case.
func (p *parser) parsePrimary() (ast.Node, error) {
	start := p.here()
	t := p.cur()

	switch {
	case t.isOp("*"):
		p.next()

		n := &ast.StarExpr{}
		n.SetSpan(start, p.here())

		return n, nil

	case t.kind == tokNumber:
		p.next()

		n := &ast.Literal{LitKind: classifyNumber(t.text), Value: t.text}
		n.SetSpan(start, p.here())

		return n, nil

	case t.kind == tokString:
		p.next()

		n := &ast.Literal{LitKind: ast.LitString, Value: t.text}
		n.SetSpan(start, p.here())

		return n, nil

	case t.kind == tokBlob:
		p.next()

		n := &ast.Literal{LitKind: ast.LitBlob, Value: t.text}
		n.SetSpan(start, p.here())

		return n, nil

	case t.kind == tokPlaceholder:
		p.next()

		n := &ast.Placeholder{Style: placeholderStyle(t.text), Text: t.text}
		n.SetSpan(start, p.here())

		return n, nil

	case t.is("null"):
		p.next()

		n := &ast.Literal{LitKind: ast.LitNull, Value: "null"}
		n.SetSpan(start, p.here())

		return n, nil

	case t.is("true"), t.is("false"):
		p.next()

		n := &ast.Literal{LitKind: ast.LitBool, Value: strings.ToLower(t.text)}
		n.SetSpan(start, p.here())

		return n, nil

	case t.is("case"):
		return p.parseCaseExpr()

	case t.is("cast"):
		return p.parseCastExpr()

	case t.is("exists"):
		p.next()

		return p.parseExistsBody(false, start)

	case t.isOp("("):
		return p.parseParenGroup(start)

	case t.kind == tokIdent:
		return p.parseIdentPrimary(start)

	default:
		return nil, p.syntaxError("expression")
	}
}

// classifyNumber decides whether a tokNumber's raw spelling (e.g. "0x1F",
// "42", "3.14", "1e10") represents an integer or floating-point literal, so
// the parser doesn't have to re-scan the digits itself — the lexer (see
// scanNumber in lexer.go) already validated the spelling is well-formed;
// this just categorizes it.
func classifyNumber(text string) ast.LitKind {
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		return ast.LitInteger
	}

	if strings.ContainsAny(text, ".eE") {
		return ast.LitFloat
	}

	return ast.LitInteger
}

// placeholderStyle classifies a tokPlaceholder's raw text (which still
// includes its leading marker character) into one of the three bind
// parameter styles ast.Placeholder distinguishes. See ast.PlaceholderStyle
// in ast/expr.go for what each style means.
func placeholderStyle(text string) ast.PlaceholderStyle {
	if text == "?" {
		return ast.PlaceholderAnonymous
	}

	switch text[0] {
	case '?', '$':
		return ast.PlaceholderNumbered
	default:
		return ast.PlaceholderNamed
	}
}

// parseExistsBody parses "(Subquery)" following an already-consumed EXISTS
// keyword (both call sites consume EXISTS, and NOT EXISTS's NOT, before
// calling this) and wraps it as an ExistsExpr with the given Not flag and
// start position.
func (p *parser) parseExistsBody(not bool, start ast.Position) (ast.Node, error) {
	if !p.cur().isOp("(") {
		return nil, p.syntaxError("'('")
	}

	p.next()

	sel, err := p.parseSelectOrCompound()
	if err != nil {
		return nil, err
	}

	end := p.here()

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	sub := &ast.Subquery{Select: sel}
	sub.SetSpan(start, end)

	n := &ast.ExistsExpr{Not: not, Sub: sub}
	n.SetSpan(start, end)

	return n, nil
}

// parseParenGroup parses a parenthesized primary: a scalar subquery
// "(SELECT ...)", a plain grouped expression "(expr)", or a tuple
// "(expr, expr, ...)".
func (p *parser) parseParenGroup(start ast.Position) (ast.Node, error) {
	p.next() // "("

	if p.selectFollows() {
		sel, err := p.parseSelectOrCompound()
		if err != nil {
			return nil, err
		}

		end := p.here()

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}

		n := &ast.Subquery{Select: sel}
		n.SetSpan(start, end)

		return n, nil
	}

	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	if p.cur().isOp(",") {
		items := []ast.Node{first}

		for p.acceptPunct(",") {
			item, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			items = append(items, item)
		}

		end := p.here()

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}

		n := &ast.ExprList{Items: items}
		n.SetSpan(start, end)

		return n, nil
	}

	end := p.here()

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	n := &ast.ParenExpr{X: first}
	n.SetSpan(start, end)

	return n, nil
}

// parseIdentPrimary parses an identifier chain (up to schema.table.column)
// and disambiguates a trailing "(" as a function call, and a trailing
// ".*" as a table-qualified star.
func (p *parser) parseIdentPrimary(start ast.Position) (ast.Node, error) {
	parts := []string{p.next().text}

	for p.cur().isOp(".") {
		if p.peek(1).isOp("*") {
			p.next() // "."
			p.next() // "*"

			n := &ast.StarExpr{Table: strings.Join(parts, ".")}
			n.SetSpan(start, p.here())

			return n, nil
		}

		if p.peek(1).kind != tokIdent {
			break
		}

		p.next() // "."
		parts = append(parts, p.next().text)
	}

	if p.cur().isOp("(") {
		return p.parseFuncCall(start, strings.Join(parts, "."))
	}

	var n *ast.ColumnRef

	switch len(parts) {
	case 1:
		n = &ast.ColumnRef{Column: parts[0]}
	case 2:
		n = &ast.ColumnRef{Table: parts[0], Column: parts[1]}
	case 3:
		n = &ast.ColumnRef{Schema: parts[0], Table: parts[1], Column: parts[2]}
	default:
		return nil, p.syntaxError("column reference")
	}

	n.SetSpan(start, p.here())

	return n, nil
}

func (p *parser) parseFuncCall(start ast.Position, name string) (ast.Node, error) {
	p.next() // "("

	call := &ast.FuncCall{Name: name}

	if p.cur().isOp("*") && p.peek(1).isOp(")") {
		p.next()

		call.Star = true
	} else if !p.cur().isOp(")") {
		if p.acceptKeyword("distinct") {
			call.Distinct = true
		}

		for {
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			call.Args = append(call.Args, arg)

			if !p.acceptPunct(",") {
				break
			}
		}
	}

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	end := p.here()

	if p.isKeyword("filter") {
		p.next()

		if err := p.expectPunct("("); err != nil {
			return nil, err
		}

		if err := p.expectKeyword("where"); err != nil {
			return nil, err
		}

		filter, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		call.Filter = filter
		end = p.here()

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}
	}

	call.SetSpan(start, end)

	return call, nil
}

func (p *parser) parseCaseExpr() (ast.Node, error) {
	start := p.here()

	p.next() // "case"

	n := &ast.CaseExpr{}

	if !p.isKeyword("when") {
		operand, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		n.Operand = operand
	}

	if !p.isKeyword("when") {
		return nil, p.syntaxError("WHEN")
	}

	for p.isKeyword("when") {
		whenStart := p.here()

		p.next()

		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		if err := p.expectKeyword("then"); err != nil {
			return nil, err
		}

		result, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		w := &ast.WhenClause{Cond: cond, Result: result}
		w.SetSpan(whenStart, result.End())
		n.Whens = append(n.Whens, w)
	}

	if p.acceptKeyword("else") {
		elseExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		n.Else = elseExpr
	}

	end := p.here()

	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}

	n.SetSpan(start, end)

	return n, nil
}

func (p *parser) parseCastExpr() (ast.Node, error) {
	start := p.here()

	p.next() // "cast"

	if err := p.expectPunct("("); err != nil {
		return nil, err
	}

	x, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	if err := p.expectKeyword("as"); err != nil {
		return nil, err
	}

	typ, err := p.parseTypeName()
	if err != nil {
		return nil, err
	}

	end := p.here()

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	n := &ast.CastExpr{X: x, Type: typ}
	n.SetSpan(start, end)

	return n, nil
}
