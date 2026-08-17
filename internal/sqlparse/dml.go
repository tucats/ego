package sqlparse

import "github.com/tucats/ego/internal/sqlparse/ast"

// This file implements INSERT, UPDATE, and DELETE. All three follow the
// same overall recipe: consume the leading keyword(s), parse the target
// table (via the shared parseTableRef + optionalAlias helpers — see
// parser.go and common.go), parse the statement-specific middle part, then
// parse the clauses INSERT/UPDATE/DELETE all three support in the same
// form: an optional ON CONFLICT (INSERT only) and an optional trailing
// RETURNING.

// parseInsertStatement parses "INSERT [OR action] INTO table [(cols...)]
// source [ON CONFLICT ...] [RETURNING ...]".
func (p *parser) parseInsertStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "insert"

	orAction, err := p.parseConflictActionKeyword()
	if err != nil {
		return nil, err
	}

	if err := p.expectKeyword("into"); err != nil {
		return nil, err
	}

	table, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	// "if x, ok, err := f(); err != nil { ... } else if ok { ... }" is an if
	// statement with its own init statement (the "x, ok, err := f()" part,
	// scoped to just this if/else chain) feeding both branches. It's a
	// compact way to write "call f, bail out on error, otherwise act only if
	// it found something" without a separate named variable declared above
	// the if. The same three-line shape appears identically in
	// parseUpdateStatement and parseDeleteStatement below, and in ddl.go and
	// select.go wherever an optional alias is parsed.
	if alias, ok, err := p.optionalAlias(); err != nil {
		return nil, err
	} else if ok {
		table.Alias = alias
	}

	stmt := &ast.InsertStmt{OrAction: orAction, Table: table}

	if p.cur().isOp("(") {
		cols, err := p.parseNameList()
		if err != nil {
			return nil, err
		}

		stmt.Columns = cols
	}

	source, err := p.parseInsertSource()
	if err != nil {
		return nil, err
	}

	stmt.Source = source

	if p.isKeyword("on") {
		oc, err := p.parseOnConflictClause()
		if err != nil {
			return nil, err
		}

		stmt.OnConflict = oc
	}

	if p.isKeyword("returning") {
		r, err := p.parseReturningClause()
		if err != nil {
			return nil, err
		}

		stmt.Returning = r
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseInsertSource parses the part of INSERT that supplies the row data:
// "DEFAULT VALUES", "VALUES (...), (...), ...", or a SELECT. Exactly one of
// these three forms is legal, which is why this is a switch rather than a
// sequence of independent optional pieces (contrast with the clauses in
// parseInsertStatement above, several of which are genuinely independent
// and optional).
func (p *parser) parseInsertSource() (ast.Node, error) {
	start := p.here()

	switch {
	case p.isKeyword("default"):
		p.next()

		if err := p.expectKeyword("values"); err != nil {
			return nil, err
		}

		n := &ast.InsertDefaultValues{}
		n.SetSpan(start, p.here())

		return n, nil

	case p.isKeyword("values"):
		p.next()

		var rows [][]ast.Node

		for {
			row, err := p.parseValuesRow()
			if err != nil {
				return nil, err
			}

			rows = append(rows, row)

			if !p.acceptPunct(",") {
				break
			}
		}

		n := &ast.InsertValues{Rows: rows}
		n.SetSpan(start, p.here())

		return n, nil

	case p.selectFollows():
		sel, err := p.parseSelectOrCompound()
		if err != nil {
			return nil, err
		}

		n := &ast.InsertSelect{Select: sel}
		n.SetSpan(start, sel.End())

		return n, nil

	default:
		return nil, p.syntaxError("VALUES, DEFAULT VALUES, or SELECT")
	}
}

// parseValuesRow parses one parenthesized row of a VALUES list: "(1, 2, 3)".
func (p *parser) parseValuesRow() ([]ast.Node, error) {
	if err := p.expectPunct("("); err != nil {
		return nil, err
	}

	var row []ast.Node

	if !p.cur().isOp(")") {
		for {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}

			row = append(row, e)

			if !p.acceptPunct(",") {
				break
			}
		}
	}

	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}

	return row, nil
}

// parseUpdateStatement parses "UPDATE [OR action] table SET ... [FROM ...]
// [WHERE ...] [RETURNING ...]". FROM is a PostgreSQL extension; nothing here
// rejects it for sqlite3 source, since this parser only checks syntax, not
// per-dialect legality (see the package doc comment in parser.go).
func (p *parser) parseUpdateStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "update"

	orAction, err := p.parseConflictActionKeyword()
	if err != nil {
		return nil, err
	}

	table, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	if alias, ok, err := p.optionalAlias(); err != nil {
		return nil, err
	} else if ok {
		table.Alias = alias
	}

	if err := p.expectKeyword("set"); err != nil {
		return nil, err
	}

	set, err := p.parseSetClauseList()
	if err != nil {
		return nil, err
	}

	stmt := &ast.UpdateStmt{OrAction: orAction, Table: table, Set: set}

	if p.acceptKeyword("from") {
		from, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}

		stmt.From = from
	}

	if p.acceptKeyword("where") {
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		stmt.Where = w
	}

	if p.isKeyword("returning") {
		r, err := p.parseReturningClause()
		if err != nil {
			return nil, err
		}

		stmt.Returning = r
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}

// parseDeleteStatement parses "DELETE FROM table [USING ...] [WHERE ...]
// [RETURNING ...]". USING, like UPDATE's FROM above, is a PostgreSQL
// extension accepted here regardless of the requested dialect.
func (p *parser) parseDeleteStatement() (ast.Statement, error) {
	start := p.here()

	p.next() // "delete"

	if err := p.expectKeyword("from"); err != nil {
		return nil, err
	}

	table, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	if alias, ok, err := p.optionalAlias(); err != nil {
		return nil, err
	} else if ok {
		table.Alias = alias
	}

	stmt := &ast.DeleteStmt{Table: table}

	if p.acceptKeyword("using") {
		using, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}

		stmt.Using = using
	}

	if p.acceptKeyword("where") {
		w, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		stmt.Where = w
	}

	if p.isKeyword("returning") {
		r, err := p.parseReturningClause()
		if err != nil {
			return nil, err
		}

		stmt.Returning = r
	}

	stmt.SetSpan(start, p.here())

	return stmt, nil
}
