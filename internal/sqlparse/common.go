package sqlparse

import "github.com/tucats/ego/internal/sqlparse/ast"

// This file holds small parsing helpers shared by more than one statement
// parser: alias recognition, index/ORDER BY column-term lists, and the
// RETURNING clause (legal on INSERT, UPDATE, and DELETE).

// clauseStopKeywords lists the keywords that can legally follow an unaliased
// table or result-column expression. A bare word here is never treated as
// an implicit (AS-less) alias, so "SELECT a FROM t" parses "FROM" as the
// FROM clause rather than as a's alias, and "t1 JOIN t2" parses "JOIN" as a
// join keyword rather than t1's alias.
//
// Background: SQL famously lets you write a column or table alias with or
// without the AS keyword — "SELECT a AS x FROM t" and "SELECT a x FROM t"
// both name the column x. That's convenient for authors but genuinely
// ambiguous for a parser: having just parsed the expression "a", the next
// bare identifier might be its alias — or it might be an unrelated keyword
// that starts the next piece of the statement, like FROM in "SELECT a FROM
// t" or JOIN in "FROM t1 JOIN t2". Without special-casing, a parser that
// eagerly treats "the next bare word" as an alias would swallow FROM or
// JOIN by mistake. Every SQL parser has to solve this the same way: keep a
// list of keywords that can never be mistaken for an alias, and only treat
// a bare identifier as an implicit alias when it is *not* one of them. This
// map is that list, gathered from every place in this package that calls
// optionalAlias — see below.
var clauseStopKeywords = map[string]bool{
	"from": true, "where": true, "group": true, "having": true,
	"order": true, "limit": true, "union": true, "intersect": true,
	"except": true, "join": true, "inner": true, "left": true, "right": true,
	"full": true, "cross": true, "natural": true, "on": true, "using": true,
	"returning": true, "offset": true, "into": true, "values": true,
	"set": true, "do": true, "nothing": true, "conflict": true,
	"then": true, "else": true, "end": true, "when": true, "indexed": true,
	"not": true, "and": true, "or": true, "as": true, "collate": true,
	"asc": true, "desc": true, "nulls": true, "window": true,
	"default": true, "select": true, "with": true,
}

// optionalAlias parses "[AS] name" and reports whether an alias was present.
// A bare (AS-less) alias is only recognized when the current token is an
// identifier that is not one of clauseStopKeywords. It returns
// (alias, found, error): found is false — not an error — when there's
// simply no alias to parse, which is the common case; callers check it the
// same way they'd check the ok result of a Go map lookup.
func (p *parser) optionalAlias() (string, bool, error) {
	if p.acceptKeyword("as") {
		name, err := p.expectIdent()

		return name, true, err
	}

	// t.quoted (from a quoted identifier like "from" or [from]) always wins
	// over the stoplist: clauseStopKeywords exists to guess what a *bare*
	// word means, but quoting is the author explicitly saying "this is a
	// name, not a keyword" — SQL's own escape hatch for exactly this
	// ambiguity. See token.go's token.is, which similarly refuses to treat
	// a quoted "select" as the SELECT keyword.
	t := p.cur()
	if t.kind == tokIdent && (t.quoted || !clauseStopKeywords[toLower(t.text)]) {
		p.next()

		return t.text, true, nil
	}

	return "", false, nil
}

// toLower is a small ASCII-only lowercaser, used here instead of the
// standard library's strings.ToLower purely for consistency with
// equalFold in token.go (which the same underlying identifier comparison
// ultimately relies on) — SQL keywords are always ASCII, so there's no
// correctness difference, just a style one.
func toLower(s string) string {
	b := []byte(s)

	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}

	return string(b)
}

// parseNameList parses a parenthesized, comma-separated list of identifiers:
// "(a, b, c)".
func (p *parser) parseNameList() ([]string, error) {
	if err := p.expectPunct("("); err != nil {
		return nil, err
	}

	var names []string

	for {
		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		names = append(names, name)

		if !p.acceptPunct(",") {
			break
		}
	}

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	return names, nil
}

// parseIndexColumnList parses a parenthesized list of index/ORDER BY-style
// terms: "(Expr [COLLATE c] [ASC|DESC], ...)". Used by CREATE INDEX and by
// table-level PRIMARY KEY / UNIQUE constraints.
func (p *parser) parseIndexColumnList() ([]*ast.OrderByTerm, error) {
	if err := p.expectPunct("("); err != nil {
		return nil, err
	}

	var terms []*ast.OrderByTerm

	for {
		term, err := p.parseOrderByTerm()
		if err != nil {
			return nil, err
		}

		terms = append(terms, term)

		if !p.acceptPunct(",") {
			break
		}
	}

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	return terms, nil
}

func (p *parser) parseOrderByTerm() (*ast.OrderByTerm, error) {
	start := p.here()

	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	term := &ast.OrderByTerm{Expr: expr}

	if p.acceptKeyword("collate") {
		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}

		term.Collation = name
	}

	// This is Go's "tagless switch" (no value after the switch keyword),
	// which behaves like a chain of "if / else if": each case is a boolean
	// expression, tried in order, and the first true one runs. Note that
	// p.acceptKeyword itself has a side effect (it consumes a token if it
	// matches) — so this isn't just picking a branch based on a value, it's
	// actively trying ASC first and only trying DESC if ASC didn't match,
	// same as writing "if p.acceptKeyword(\"asc\") { } else if
	// p.acceptKeyword(\"desc\") { term.Desc = true }" longhand. Both cases
	// are optional — if neither keyword is present, the switch matches
	// nothing and falls through with term.Desc left at its zero value
	// (false, meaning ascending). This same tagless-switch-over-acceptKeyword
	// shape shows up again below and in ddl.go/dml.go wherever a handful of
	// mutually exclusive optional keywords need to be tried in order.
	switch {
	case p.acceptKeyword("asc"):
	case p.acceptKeyword("desc"):
		term.Desc = true
	}

	if p.isKeyword("nulls") {
		p.next()

		switch {
		case p.acceptKeyword("first"):
			first := true
			term.NullsFirst = &first
		case p.acceptKeyword("last"):
			last := false
			term.NullsFirst = &last
		default:
			return nil, p.syntaxError("FIRST or LAST")
		}
	}

	term.SetSpan(start, p.here())

	return term, nil
}

// parseReturningClause parses a trailing "RETURNING col [AS alias], ..." on
// INSERT, UPDATE, or DELETE.
func (p *parser) parseReturningClause() (*ast.ReturningClause, error) {
	start := p.here()

	p.next() // "returning"

	cols, err := p.parseResultColumnList()
	if err != nil {
		return nil, err
	}

	n := &ast.ReturningClause{Columns: cols}
	n.SetSpan(start, p.here())

	return n, nil
}

// parseConflictAction parses the trailing "ON CONFLICT [(target...)]
// [WHERE targetWhere] DO NOTHING | DO UPDATE SET ... [WHERE ...]" clause
// shared, with identical syntax, by sqlite3's UPSERT and PostgreSQL.
func (p *parser) parseOnConflictClause() (*ast.OnConflictClause, error) {
	start := p.here()

	p.next() // "on"

	if err := p.expectKeyword("conflict"); err != nil {
		return nil, err
	}

	n := &ast.OnConflictClause{}

	if p.cur().isOp("(") {
		terms, err := p.parseIndexColumnList()
		if err != nil {
			return nil, err
		}

		names := make([]string, 0, len(terms))

		// t.Expr.(*ast.ColumnRef) is a Go "type assertion": t.Expr is
		// statically typed as the ast.Node interface, but at runtime it
		// might concretely be any node type parseOrderByTerm's underlying
		// parseExpr call could produce. The ", ok" form (as opposed to a
		// bare assertion) never panics — it just reports ok=false when
		// t.Expr isn't actually a *ast.ColumnRef, which is how this loop
		// silently skips any target-list entry that turned out not to be a
		// plain column name (a conflict target can only be one, so ON
		// CONFLICT syntax like an expression there is simply not
		// represented in OnConflictClause.Target — this parser doesn't
		// currently reject that as a syntax error, it just drops it).
		for _, t := range terms {
			if ref, ok := t.Expr.(*ast.ColumnRef); ok {
				names = append(names, ref.Column)
			}
		}

		n.Target = names

		if p.acceptKeyword("where") {
			w, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			n.TargetWhere = w
		}
	}

	if err := p.expectKeyword("do"); err != nil {
		return nil, err
	}

	if p.acceptKeyword("nothing") {
		n.DoNothing = true
		n.SetSpan(start, p.here())

		return n, nil
	}

	if err := p.expectKeyword("update"); err != nil {
		return nil, err
	}

	if err := p.expectKeyword("set"); err != nil {
		return nil, err
	}

	set, err := p.parseSetClauseList()
	if err != nil {
		return nil, err
	}

	n.UpdateSet = set

	if p.acceptKeyword("where") {
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		n.UpdateWhere = w
	}

	n.SetSpan(start, p.here())

	return n, nil
}

// parseSetClauseList parses an UPDATE (or ON CONFLICT DO UPDATE) SET list:
// "col = expr, ..." with the row-value form "(col1, col2) = (expr1, expr2)"
// also accepted.
func (p *parser) parseSetClauseList() ([]*ast.SetClause, error) {
	var clauses []*ast.SetClause

	for {
		start := p.here()

		var columns []string

		if p.cur().isOp("(") {
			names, err := p.parseNameList()
			if err != nil {
				return nil, err
			}

			columns = names
		} else {
			name, err := p.expectIdent()
			if err != nil {
				return nil, err
			}

			columns = []string{name}
		}

		if err := p.expectOp("="); err != nil {
			return nil, err
		}

		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		c := &ast.SetClause{Columns: columns, Value: value}
		c.SetSpan(start, value.End())
		clauses = append(clauses, c)

		if !p.acceptPunct(",") {
			break
		}
	}

	return clauses, nil
}

// parseConflictActionKeyword parses sqlite3's optional "OR action" conflict
// resolution keyword on INSERT/UPDATE ("OR REPLACE", "OR IGNORE", etc.),
// returning the normalized action word, or "" when no OR clause is present.
func (p *parser) parseConflictActionKeyword() (string, error) {
	if !p.acceptKeyword("or") {
		return "", nil
	}

	switch {
	case p.acceptKeyword("replace"):
		return "REPLACE", nil
	case p.acceptKeyword("ignore"):
		return "IGNORE", nil
	case p.acceptKeyword("abort"):
		return "ABORT", nil
	case p.acceptKeyword("fail"):
		return "FAIL", nil
	case p.acceptKeyword("rollback"):
		return "ROLLBACK", nil
	default:
		return "", p.syntaxError("REPLACE, IGNORE, ABORT, FAIL, or ROLLBACK")
	}
}
