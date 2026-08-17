package sqlparse

import "github.com/tucats/ego/internal/sqlparse/ast"

// This file implements SELECT: the WITH prefix, compound (UNION/INTERSECT/
// EXCEPT) chaining, the FROM clause (table refs, subqueries, joins), and the
// trailing ORDER BY / LIMIT. The overall shape mirrors SQL's own statement
// structure, outermost to innermost:
//
//	parseSelectBody     WITH ..., then a compound chain, then ORDER BY / LIMIT
//	  parseWithClause     the "WITH [RECURSIVE] name AS (...), ..." prefix
//	  parseCompoundSelect   one or more cores joined by UNION/INTERSECT/EXCEPT
//	    parseSelectCore       one "SELECT ... FROM ... WHERE ..." (no compound op)
//	      parseResultColumnList  the comma-separated SELECT list
//	      parseFromClause        the comma-separated table-item list
//	        parseJoinChain          one item, possibly extended with JOINs
//	          parseTableItem            a table name, subquery, or nested join
//
// This is the same recursive-descent shape used throughout the package (see
// "How this parser works" in parser.go), just applied to SELECT's grammar
// specifically rather than to operator precedence.

// parseSelectStatement is the entry point parser.parseStatement calls when
// a statement begins with SELECT (or WITH, since a WITH clause can prefix a
// SELECT). It's a thin wrapper because the actual work — and the ability to
// reuse it for subqueries, not just top-level statements — lives in
// parseSelectBody; see parseSelectOrCompound just below for why there are
// two near-identical entry points into the same body.
func (p *parser) parseSelectStatement() (ast.Statement, error) {
	return p.parseSelectBody()
}

// parseSelectOrCompound parses a SELECT for use inside a subquery
// expression, CTE body, or INSERT ... SELECT source. It returns ast.Node
// (rather than ast.Statement) because it is only ever used as a child of
// another node, never as the parse result itself — but the concrete value
// is always a *ast.SelectStmt, which also happens to satisfy ast.Statement.
func (p *parser) parseSelectOrCompound() (ast.Node, error) {
	return p.parseSelectBody()
}

// parseSelectBody parses everything a SELECT statement can contain — the
// optional WITH prefix, the compound SELECT chain, and the trailing
// ORDER BY / LIMIT — and is shared by both parseSelectStatement (top-level
// statements) and parseSelectOrCompound (subqueries, CTE bodies, and
// INSERT ... SELECT sources), which is why it returns the concrete
// *ast.SelectStmt type rather than either of their return types directly:
// callers that need an ast.Statement or an ast.Node can each convert from
// the same concrete value without this function having to pick one.
func (p *parser) parseSelectBody() (*ast.SelectStmt, error) {
	start := p.here()

	var with *ast.WithClause

	if p.isKeyword("with") {
		w, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}

		with = w
	}

	core, err := p.parseCompoundSelect()
	if err != nil {
		return nil, err
	}

	var orderBy []*ast.OrderByTerm

	if p.isKeyword("order") {
		p.next()

		if err := p.expectKeyword("by"); err != nil {
			return nil, err
		}

		for {
			term, err := p.parseOrderByTerm()
			if err != nil {
				return nil, err
			}

			orderBy = append(orderBy, term)

			if !p.acceptPunct(",") {
				break
			}
		}
	}

	var limit *ast.LimitClause

	if p.isKeyword("limit") {
		l, err := p.parseLimitClause()
		if err != nil {
			return nil, err
		}

		limit = l
	}

	stmt := &ast.SelectStmt{With: with, Select: core, OrderBy: orderBy, Limit: limit}
	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseLimitClause parses "LIMIT n [OFFSET m]" — or sqlite3's alternate
// "LIMIT m, n" spelling of the same thing, where the two numbers swap
// positions (see the comment on that case below).
func (p *parser) parseLimitClause() (*ast.LimitClause, error) {
	start := p.here()

	p.next() // "limit"

	limit, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	var offset ast.Node

	switch {
	case p.acceptKeyword("offset"):
		offset, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	case p.acceptPunct(","):
		// sqlite3's "LIMIT offset, count" form; note the arguments are
		// reversed relative to the "LIMIT count OFFSET offset" form.
		second, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		offset = limit
		limit = second
	}

	n := &ast.LimitClause{Limit: limit, Offset: offset}
	n.SetSpan(start, p.here())

	return n, nil
}

// parseWithClause parses "WITH [RECURSIVE] name AS (select), ...".
func (p *parser) parseWithClause() (*ast.WithClause, error) {
	start := p.here()

	p.next() // "with"

	recursive := p.acceptKeyword("recursive")

	var ctes []*ast.CTE

	for {
		cte, err := p.parseCTE()
		if err != nil {
			return nil, err
		}

		ctes = append(ctes, cte)

		if !p.acceptPunct(",") {
			break
		}
	}

	n := &ast.WithClause{Recursive: recursive, CTEs: ctes}
	n.SetSpan(start, p.here())

	return n, nil
}

// parseCTE parses one "name [(columns...)] AS (select)" entry of a WITH
// clause. Note the recursive call back into parseSelectOrCompound for the
// parenthesized select — a CTE body is a full SELECT (or compound SELECT),
// which may itself contain its own nested subqueries or even its own WITH,
// so this genuinely is the "descent" of recursive descent going back
// around a full loop.
func (p *parser) parseCTE() (*ast.CTE, error) {
	start := p.here()

	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	var columns []string

	if p.cur().isOp("(") {
		cols, err := p.parseNameList()
		if err != nil {
			return nil, err
		}

		columns = cols
	}

	if err := p.expectKeyword("as"); err != nil {
		return nil, err
	}

	if err := p.expectPunct("("); err != nil {
		return nil, err
	}

	sel, err := p.parseSelectOrCompound()
	if err != nil {
		return nil, err
	}

	end := p.here()

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	n := &ast.CTE{Name: name, Columns: columns, Select: sel}
	n.SetSpan(start, end)

	return n, nil
}

// parseCompoundSelect parses one or more SELECT cores joined by UNION,
// UNION ALL, INTERSECT, or EXCEPT. This is structurally the same
// "parse one operand, then loop consuming same-tier operators, building a
// left-leaning tree" pattern used throughout expr.go for arithmetic and
// logical operators (see the file comment at the top of expr.go) — here the
// "operator" is a compound keyword and the "operand" is a *ast.SelectCore
// instead of an expression, but the shape and the left-associativity
// argument are identical: "SELECT a UNION SELECT b UNION SELECT c" nests as
// (a UNION b) UNION c.
func (p *parser) parseCompoundSelect() (ast.Node, error) {
	left, err := p.parseSelectCore()
	if err != nil {
		return nil, err
	}

	var result ast.Node = left

	for {
		var op string

		switch {
		case p.isKeyword("union"):
			p.next()

			if p.acceptKeyword("all") {
				op = "UNION ALL"
			} else {
				op = "UNION"
			}
		case p.isKeyword("intersect"):
			p.next()

			op = "INTERSECT"
		case p.isKeyword("except"):
			p.next()

			op = "EXCEPT"
		default:
			return result, nil
		}

		right, err := p.parseSelectCore()
		if err != nil {
			return nil, err
		}

		n := &ast.CompoundSelect{Left: result, Op: op, Right: right}
		n.SetSpan(result.Pos(), right.End())
		result = n
	}
}

// parseSelectCore parses one non-compound SELECT: the keyword, the optional
// DISTINCT/ALL, the result column list, and the optional FROM/WHERE/GROUP
// BY/HAVING clauses. It does not handle ORDER BY, LIMIT, or the compound
// operators (UNION and friends) — those belong to the statement as a whole
// and are parsed by parseSelectBody and parseCompoundSelect respectively,
// one level up, because "SELECT a FROM t1 UNION SELECT b FROM t2 ORDER BY a"
// has exactly one ORDER BY governing the combined result, not one per core.
func (p *parser) parseSelectCore() (*ast.SelectCore, error) {
	start := p.here()

	if err := p.expectKeyword("select"); err != nil {
		return nil, err
	}

	core := &ast.SelectCore{}

	switch {
	case p.acceptKeyword("distinct"):
		core.Distinct = true
	case p.acceptKeyword("all"):
		core.All = true
	}

	cols, err := p.parseResultColumnList()
	if err != nil {
		return nil, err
	}

	core.Columns = cols

	if p.acceptKeyword("from") {
		from, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}

		core.From = from
	}

	if p.acceptKeyword("where") {
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		core.Where = w
	}

	if p.acceptKeyword("group") {
		if err := p.expectKeyword("by"); err != nil {
			return nil, err
		}

		for {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			core.GroupBy = append(core.GroupBy, e)

			if !p.acceptPunct(",") {
				break
			}
		}

		if p.acceptKeyword("having") {
			h, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			core.Having = h
		}
	}

	core.SetSpan(start, p.here())

	return core, nil
}

// parseResultColumnList parses the comma-separated SELECT list: "a, b AS x,
// t.*". It's also reused by parseReturningClause in common.go, since
// RETURNING has the same "expression with an optional alias, or a star"
// shape as a SELECT list.
func (p *parser) parseResultColumnList() ([]*ast.ResultColumn, error) {
	var cols []*ast.ResultColumn

	for {
		col, err := p.parseResultColumn()
		if err != nil {
			return nil, err
		}

		cols = append(cols, col)

		if !p.acceptPunct(",") {
			break
		}
	}

	return cols, nil
}

func (p *parser) parseResultColumn() (*ast.ResultColumn, error) {
	start := p.here()

	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	col := &ast.ResultColumn{Expr: expr}

	// A star ("*" or "t.*") can never take an alias.
	if _, ok := expr.(*ast.StarExpr); !ok {
		alias, ok, err := p.optionalAlias()
		if err != nil {
			return nil, err
		}

		if ok {
			col.Alias = alias
		}
	}

	col.SetSpan(start, p.here())

	return col, nil
}

// parseFromClause parses the comma-separated table-item list following
// FROM. Each item may itself be a chain of explicit JOINs; a comma between
// top-level items is an implicit cross join.
func (p *parser) parseFromClause() ([]ast.Node, error) {
	var items []ast.Node

	for {
		item, err := p.parseJoinChain()
		if err != nil {
			return nil, err
		}

		items = append(items, item)

		if !p.acceptPunct(",") {
			break
		}
	}

	return items, nil
}

// parseJoinChain parses one FROM-clause item together with any explicit
// JOINs directly chained onto it: "a", "a JOIN b ON ...", or
// "a JOIN b ON ... JOIN c USING (...)". Like parseCompoundSelect above (and
// the binary-operator tiers in expr.go), this is "parse one operand, then
// loop consuming same-tier operators, building a left-leaning tree" — here
// the operand is a table item and the operator is a JOIN — so
// "a JOIN b JOIN c" builds as (a JOIN b) JOIN c, left-associatively, the
// same way "a + b + c" does in expr.go.
func (p *parser) parseJoinChain() (ast.Node, error) {
	left, err := p.parseTableItem()
	if err != nil {
		return nil, err
	}

	for p.joinFollows() {
		joinType, err := p.parseJoinKeywords()
		if err != nil {
			return nil, err
		}

		right, err := p.parseTableItem()
		if err != nil {
			return nil, err
		}

		n := &ast.JoinClause{Left: left, JoinType: joinType, Right: right}

		switch {
		case p.acceptKeyword("on"):
			on, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			n.On = on
		case p.acceptKeyword("using"):
			names, err := p.parseNameList()
			if err != nil {
				return nil, err
			}

			n.Using = names
		}

		n.SetSpan(left.Pos(), p.here())
		left = n
	}

	return left, nil
}

// joinFollows reports whether the current token could start a JOIN —
// either the bare JOIN keyword, or one of the qualifier keywords
// (INNER/LEFT/RIGHT/FULL/CROSS/NATURAL) that precede it. This lookahead
// exists so parseJoinChain's loop condition doesn't need to (and can't,
// without backtracking — see "How this parser works" in parser.go) commit
// to consuming any of those keywords just to find out whether a join is
// actually there.
func (p *parser) joinFollows() bool {
	return p.isKeyword("join") || p.isKeyword("inner") || p.isKeyword("left") ||
		p.isKeyword("right") || p.isKeyword("full") || p.isKeyword("cross") ||
		p.isKeyword("natural")
}

// parseJoinKeywords consumes the qualifier keyword(s) before JOIN (if any)
// and the JOIN keyword itself, and returns a normalized, space-joined,
// upper-case spelling of the qualifiers for ast.JoinClause.JoinType — "",
// "INNER", "LEFT OUTER", "NATURAL LEFT", "CROSS", and so on. Building the
// string with a manual loop (rather than strings.Join) avoids importing
// strings just for this one call site; parts is at most two or three
// elements, so the loop costs nothing.
func (p *parser) parseJoinKeywords() (string, error) {
	var parts []string

	if p.acceptKeyword("natural") {
		parts = append(parts, "NATURAL")
	}

	switch {
	case p.acceptKeyword("inner"):
		parts = append(parts, "INNER")
	case p.acceptKeyword("left"):
		parts = append(parts, "LEFT")

		if p.acceptKeyword("outer") {
			parts = append(parts, "OUTER")
		}
	case p.acceptKeyword("right"):
		parts = append(parts, "RIGHT")

		if p.acceptKeyword("outer") {
			parts = append(parts, "OUTER")
		}
	case p.acceptKeyword("full"):
		parts = append(parts, "FULL")

		if p.acceptKeyword("outer") {
			parts = append(parts, "OUTER")
		}
	case p.acceptKeyword("cross"):
		parts = append(parts, "CROSS")
	}

	if err := p.expectKeyword("join"); err != nil {
		return "", err
	}

	joined := ""

	for i, part := range parts {
		if i > 0 {
			joined += " "
		}

		joined += part
	}

	return joined, nil
}

// parseTableItem parses one FROM-clause item: a parenthesized subquery, a
// parenthesized nested join, or a plain (possibly schema-qualified,
// possibly aliased) table reference.
func (p *parser) parseTableItem() (ast.Node, error) {
	start := p.here()

	if p.cur().isOp("(") {
		p.next()

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

			n := &ast.SubqueryRef{Sub: sub}

			alias, ok, err := p.optionalAlias()
			if err != nil {
				return nil, err
			}

			if ok {
				n.Alias = alias

				if p.cur().isOp("(") {
					cols, err := p.parseNameList()
					if err != nil {
						return nil, err
					}

					n.Columns = cols
				}
			}

			n.SetSpan(start, p.here())

			return n, nil
		}

		// A parenthesized join group, e.g. "(a JOIN b ON ...)" used to
		// control join order. Note this returns the inner JoinClause as-is
		// rather than wrapping it in anything that records the parentheses
		// — unlike a parenthesized scalar expression (see ParenExpr in
		// expr.go), which does keep a node for its own parens, a
		// parenthesized join simply reuses the same JoinClause shape it
		// would have without the parens. A future formatter wanting to
		// reproduce the original grouping exactly would need to add that;
		// today the parser only guarantees the same *tree structure*, not
		// the exact source punctuation, for this one case.
		inner, err := p.parseJoinChain()
		if err != nil {
			return nil, err
		}

		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}

		return inner, nil
	}

	ref, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	alias, ok, err := p.optionalAlias()
	if err != nil {
		return nil, err
	}

	if ok {
		ref.Alias = alias
	}

	switch {
	case p.acceptKeyword("indexed"):
		if err := p.expectKeyword("by"); err != nil {
			return nil, err
		}

		idx, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		ref.IndexedBy = idx
	case p.isKeyword("not") && p.isKeywordAt(1, "indexed"):
		p.next()
		p.next()

		ref.NotIndexed = true
	}

	ref.SetSpan(start, p.here())

	return ref, nil
}
