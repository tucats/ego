package sqlparse

import "github.com/tucats/ego/internal/sqlparse/ast"

// This file formats SELECT and everything specific to it: the WITH prefix,
// compound (UNION/INTERSECT/EXCEPT) chains, the FROM clause (table refs,
// subqueries, joins), and the trailing ORDER BY / LIMIT. It mirrors
// select.go's structure — see that file's parsing counterpart for each
// function here.

func (pr *printer) selectStmt(s *ast.SelectStmt) {
	if s.With != nil {
		pr.withClause(s.With)
	}

	pr.selectBody(s.Select)

	if len(s.OrderBy) > 0 {
		pr.newline()
		pr.write("ORDER BY ")

		for i, t := range s.OrderBy {
			if i > 0 {
				pr.write(", ")
			}

			pr.orderByTerm(t)
		}
	}

	if s.Limit != nil {
		pr.newline()
		pr.limitClause(s.Limit)
	}
}

// withClause formats a WITH prefix, one CTE per line, and leaves the
// printer positioned at the start of a new line ready for the statement
// that follows it.
func (pr *printer) withClause(w *ast.WithClause) {
	pr.write("WITH ")

	if w.Recursive {
		pr.write("RECURSIVE ")
	}

	for i, cte := range w.CTEs {
		if i > 0 {
			pr.write(",")
			pr.newline()
		}

		pr.ident(cte.Name)

		if len(cte.Columns) > 0 {
			pr.write(" (")

			for j, c := range cte.Columns {
				if j > 0 {
					pr.write(", ")
				}

				pr.ident(c)
			}

			pr.write(")")
		}

		pr.write(" AS (")
		pr.indent()
		pr.newline()
		pr.selectLike(cte.Select)
		pr.dedent()
		pr.newline()
		pr.write(")")
	}

	pr.newline()
}

// selectBody formats n, which is a *ast.SelectCore for a plain SELECT or a
// *ast.CompoundSelect for a UNION/INTERSECT/EXCEPT chain. The compound case
// recurses on its Left side — which is itself either a *SelectCore or a
// further-nested *CompoundSelect, for a chain of three or more — the same
// left-leaning tree shape parseCompoundSelect (select.go) builds it in, so
// walking it this way visits each core in its original left-to-right order.
func (pr *printer) selectBody(n ast.Node) {
	switch v := n.(type) {
	case *ast.SelectCore:
		pr.selectCore(v)
	case *ast.CompoundSelect:
		pr.selectBody(v.Left)
		pr.newline()
		pr.write(v.Op)
		pr.newline()
		pr.selectCore(v.Right)
	default:
		if n != nil {
			pr.write(n.String())
		}
	}
}

func (pr *printer) selectCore(c *ast.SelectCore) {
	pr.write("SELECT ")

	if c.Distinct {
		pr.write("DISTINCT ")
	}

	if c.All {
		pr.write("ALL ")
	}

	for i, col := range c.Columns {
		if i > 0 {
			pr.write(", ")
		}

		pr.resultColumn(col)
	}

	if len(c.From) > 0 {
		pr.newline()
		pr.write("FROM ")
		pr.fromList(c.From)
	}

	if c.Where != nil {
		pr.newline()
		pr.write("WHERE ")
		pr.expr(c.Where)
	}

	if len(c.GroupBy) > 0 {
		pr.newline()
		pr.write("GROUP BY ")

		for i, e := range c.GroupBy {
			if i > 0 {
				pr.write(", ")
			}

			pr.expr(e)
		}
	}

	if c.Having != nil {
		pr.newline()
		pr.write("HAVING ")
		pr.expr(c.Having)
	}
}

func (pr *printer) resultColumn(c *ast.ResultColumn) {
	pr.expr(c.Expr)

	if c.Alias != "" {
		pr.write(" AS ")
		pr.ident(c.Alias)
	}
}

// fromList formats the comma-separated table-item list following FROM (or
// UPDATE's FROM / DELETE's USING, which reuse it — see format_dml.go). Each
// top-level item is comma-joined on one line (an implicit cross join is
// rare enough not to warrant its own line), but an item that is itself a
// JOIN chain still gets one JOIN per line — see fromItem below.
func (pr *printer) fromList(items []ast.Node) {
	for i, item := range items {
		if i > 0 {
			pr.write(", ")
		}

		pr.fromItem(item)
	}
}

// fromItem formats one FROM-clause item. A *ast.JoinClause recurses on its
// Left side and then emits its own JOIN on a fresh line — the same
// left-leaning-tree walk selectBody uses above for compound SELECTs,
// applied to parseJoinChain's join tree (select.go) instead.
func (pr *printer) fromItem(n ast.Node) {
	switch v := n.(type) {
	case *ast.JoinClause:
		pr.fromItem(v.Left)
		pr.newline()

		if v.JoinType != "" {
			pr.write(v.JoinType)
			pr.write(" ")
		}

		pr.write("JOIN ")
		pr.fromItem(v.Right)

		switch {
		case v.On != nil:
			pr.write(" ON ")
			pr.expr(v.On)
		case len(v.Using) > 0:
			pr.write(" USING (")

			for i, u := range v.Using {
				if i > 0 {
					pr.write(", ")
				}

				pr.ident(u)
			}

			pr.write(")")
		}
	case *ast.TableRef:
		pr.tableRef(v)
	case *ast.SubqueryRef:
		pr.subqueryRef(v)
	default:
		if n != nil {
			pr.write(n.String())
		}
	}
}

func (pr *printer) tableRef(t *ast.TableRef) {
	if t.Schema != "" {
		pr.ident(t.Schema)
		pr.write(".")
	}

	pr.ident(t.Name)

	if t.Alias != "" {
		pr.write(" AS ")
		pr.ident(t.Alias)
	}

	switch {
	case t.IndexedBy != "":
		pr.write(" INDEXED BY ")
		pr.ident(t.IndexedBy)
	case t.NotIndexed:
		pr.write(" NOT INDEXED")
	}
}

// subqueryRef formats a parenthesized SELECT used as a FROM-clause item,
// indenting its body one level deeper — see "Format renders" in format.go's
// file comment for this package's general subquery-indentation rule.
func (pr *printer) subqueryRef(s *ast.SubqueryRef) {
	pr.write("(")
	pr.indent()
	pr.newline()
	pr.selectLike(s.Sub.Select)
	pr.dedent()
	pr.newline()
	pr.write(")")

	if s.Alias != "" {
		pr.write(" AS ")
		pr.ident(s.Alias)

		if len(s.Columns) > 0 {
			pr.write(" (")

			for i, c := range s.Columns {
				if i > 0 {
					pr.write(", ")
				}

				pr.ident(c)
			}

			pr.write(")")
		}
	}
}

func (pr *printer) orderByTerm(t *ast.OrderByTerm) {
	pr.expr(t.Expr)

	if t.Collation != "" {
		pr.write(" COLLATE ")
		pr.ident(t.Collation)
	}

	// ASC is the default and is never written explicitly — only an
	// explicit DESC changes the output, matching how OrderByTerm.Desc
	// itself can't distinguish "written as ASC" from "not written at all"
	// (see the comment on parseOrderByTerm in common.go).
	if t.Desc {
		pr.write(" DESC")
	}

	if t.NullsFirst != nil {
		if *t.NullsFirst {
			pr.write(" NULLS FIRST")
		} else {
			pr.write(" NULLS LAST")
		}
	}
}

// indexColumnList formats a parenthesized-list's worth of OrderByTerm
// values without the parentheses themselves — used for CREATE INDEX's
// column list and a table-level PRIMARY KEY/UNIQUE constraint's column
// list (format_ddl.go), both of which reuse ast.OrderByTerm the same way
// their parsers do (see parseIndexColumnList in common.go).
func (pr *printer) indexColumnList(terms []*ast.OrderByTerm) {
	for i, t := range terms {
		if i > 0 {
			pr.write(", ")
		}

		pr.orderByTerm(t)
	}
}

func (pr *printer) limitClause(l *ast.LimitClause) {
	pr.write("LIMIT ")
	pr.expr(l.Limit)

	if l.Offset != nil {
		pr.write(" OFFSET ")
		pr.expr(l.Offset)
	}
}
