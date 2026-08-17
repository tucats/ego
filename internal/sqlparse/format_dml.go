package sqlparse

import "github.com/tucats/ego/internal/sqlparse/ast"

// This file formats INSERT, UPDATE, and DELETE — the counterpart to dml.go.
//
// All three statement types carry a With field (a WithClause, same as
// SelectStmt's) for the dialects that allow "WITH ... INSERT/UPDATE/DELETE
// ...", but dml.go's parser functions never populate it today — only
// parseSelectBody (select.go) does. The checks below are future-proofing
// for whenever that's added, not dead code covering a case that already
// happens; With is always nil on these three statement types as things
// stand.

func (pr *printer) insertStmt(s *ast.InsertStmt) {
	if s.With != nil {
		pr.withClause(s.With)
	}

	pr.write("INSERT")

	if s.OrAction != "" {
		pr.write(" OR ")
		pr.write(s.OrAction)
	}

	pr.write(" INTO ")
	pr.tableRef(s.Table)

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

	pr.newline()
	pr.insertSource(s.Source)

	if s.OnConflict != nil {
		pr.newline()
		pr.onConflictClause(s.OnConflict)
	}

	if s.Returning != nil {
		pr.newline()
		pr.returningClause(s.Returning)
	}
}

// insertSource formats whichever of the three mutually exclusive INSERT
// source forms n is — see parseInsertSource in dml.go, which builds exactly
// one of these three node types.
func (pr *printer) insertSource(n ast.Node) {
	switch v := n.(type) {
	case *ast.InsertValues:
		pr.write("VALUES ")

		for i, row := range v.Rows {
			if i > 0 {
				pr.write(", ")
			}

			pr.write("(")

			for j, e := range row {
				if j > 0 {
					pr.write(", ")
				}

				pr.expr(e)
			}

			pr.write(")")
		}
	case *ast.InsertSelect:
		pr.selectLike(v.Select)
	case *ast.InsertDefaultValues:
		pr.write("DEFAULT VALUES")
	}
}

func (pr *printer) onConflictClause(o *ast.OnConflictClause) {
	pr.write("ON CONFLICT")

	if len(o.Target) > 0 {
		pr.write(" (")

		for i, c := range o.Target {
			if i > 0 {
				pr.write(", ")
			}

			pr.ident(c)
		}

		pr.write(")")
	}

	if o.TargetWhere != nil {
		pr.write(" WHERE ")
		pr.expr(o.TargetWhere)
	}

	if o.DoNothing {
		pr.write(" DO NOTHING")

		return
	}

	pr.write(" DO UPDATE SET ")
	pr.setClauseList(o.UpdateSet)

	if o.UpdateWhere != nil {
		pr.write(" WHERE ")
		pr.expr(o.UpdateWhere)
	}
}

// setClauseList formats an UPDATE (or ON CONFLICT DO UPDATE) SET list.
func (pr *printer) setClauseList(set []*ast.SetClause) {
	for i, c := range set {
		if i > 0 {
			pr.write(", ")
		}

		pr.setClause(c)
	}
}

func (pr *printer) setClause(c *ast.SetClause) {
	if len(c.Columns) == 1 {
		pr.ident(c.Columns[0])
	} else {
		pr.write("(")

		for i, name := range c.Columns {
			if i > 0 {
				pr.write(", ")
			}

			pr.ident(name)
		}

		pr.write(")")
	}

	pr.write(" = ")
	pr.expr(c.Value)
}

// returningClause formats a trailing RETURNING clause, shared by INSERT,
// UPDATE, and DELETE. It reuses resultColumn (format_select.go) since
// RETURNING's column list has the same "expression with an optional alias,
// or a star" shape as a SELECT list — the same reuse parseReturningClause
// (common.go) makes on the parsing side.
func (pr *printer) returningClause(r *ast.ReturningClause) {
	pr.write("RETURNING ")

	for i, c := range r.Columns {
		if i > 0 {
			pr.write(", ")
		}

		pr.resultColumn(c)
	}
}

func (pr *printer) updateStmt(s *ast.UpdateStmt) {
	if s.With != nil {
		pr.withClause(s.With)
	}

	pr.write("UPDATE")

	if s.OrAction != "" {
		pr.write(" OR ")
		pr.write(s.OrAction)
	}

	pr.write(" ")
	pr.tableRef(s.Table)
	pr.newline()
	pr.write("SET ")
	pr.setClauseList(s.Set)

	if len(s.From) > 0 {
		pr.newline()
		pr.write("FROM ")
		pr.fromList(s.From)
	}

	if s.Where != nil {
		pr.newline()
		pr.write("WHERE ")
		pr.expr(s.Where)
	}

	if s.Returning != nil {
		pr.newline()
		pr.returningClause(s.Returning)
	}
}

func (pr *printer) deleteStmt(s *ast.DeleteStmt) {
	if s.With != nil {
		pr.withClause(s.With)
	}

	pr.write("DELETE FROM ")
	pr.tableRef(s.Table)

	if len(s.Using) > 0 {
		pr.newline()
		pr.write("USING ")
		pr.fromList(s.Using)
	}

	if s.Where != nil {
		pr.newline()
		pr.write("WHERE ")
		pr.expr(s.Where)
	}

	if s.Returning != nil {
		pr.newline()
		pr.returningClause(s.Returning)
	}
}
