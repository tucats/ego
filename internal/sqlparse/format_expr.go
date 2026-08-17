package sqlparse

import (
	"strings"

	"github.com/tucats/ego/internal/sqlparse/ast"
)

// This file formats expressions — the counterpart to expr.go. Unlike
// expr.go, which is organized as a ladder of functions, one per precedence
// level (see its file comment), formatting doesn't need to rediscover
// precedence: the AST already recorded exactly which parts were
// parenthesized in the source (see ParenExpr below), so reproducing the
// tree structure as written — X, then a space, then the operator, then a
// space, then Y — is always correct, regardless of what the operator is or
// how tightly it binds. That's why this file is one flat type switch
// instead of expr.go's chain of tiers.

// expr formats any expression node. It is the single dispatch point every
// other format_*.go file calls into whenever it needs to render an
// expression — a WHERE condition, a DEFAULT value, a function argument, and
// so on — the same role parseExpr plays on the parsing side (expr.go).
func (pr *printer) expr(n ast.Node) {
	switch v := n.(type) {
	case *ast.ColumnRef:
		pr.columnRef(v)
	case *ast.StarExpr:
		if v.Table != "" {
			pr.ident(v.Table)
			pr.write(".")
		}

		pr.write("*")
	case *ast.Literal:
		pr.literal(v)
	case *ast.Placeholder:
		pr.write(v.Text)
	case *ast.UnaryExpr:
		pr.unaryExpr(v)
	case *ast.BinaryExpr:
		pr.expr(v.X)
		pr.write(" ")
		pr.write(v.Op)
		pr.write(" ")
		pr.expr(v.Y)
	case *ast.BetweenExpr:
		pr.expr(v.X)

		if v.Not {
			pr.write(" NOT")
		}

		pr.write(" BETWEEN ")
		pr.expr(v.Low)
		pr.write(" AND ")
		pr.expr(v.High)
	case *ast.InExpr:
		pr.inExpr(v)
	case *ast.LikeExpr:
		pr.likeExpr(v)
	case *ast.IsNullExpr:
		pr.expr(v.X)
		pr.write(" IS")

		if v.Not {
			pr.write(" NOT")
		}

		pr.write(" NULL")
	case *ast.IsExpr:
		pr.expr(v.X)
		pr.write(" IS")

		if v.Not {
			pr.write(" NOT")
		}

		if v.Distinct {
			pr.write(" DISTINCT FROM")
		}

		pr.write(" ")
		pr.expr(v.Y)
	case *ast.CollateExpr:
		pr.expr(v.X)
		pr.write(" COLLATE ")
		pr.ident(v.Collation)
	case *ast.FuncCall:
		pr.funcCall(v)
	case *ast.CastExpr:
		pr.write("CAST(")
		pr.expr(v.X)
		pr.write(" AS ")
		pr.typeName(v.Type)
		pr.write(")")
	case *ast.CaseExpr:
		pr.caseExpr(v)
	case *ast.ParenExpr:
		pr.write("(")
		pr.expr(v.X)
		pr.write(")")
	case *ast.ExistsExpr:
		if v.Not {
			pr.write("NOT ")
		}

		pr.write("EXISTS (")
		pr.indent()
		pr.newline()
		pr.selectLike(v.Sub.Select)
		pr.dedent()
		pr.newline()
		pr.write(")")
	case *ast.Subquery:
		pr.write("(")
		pr.indent()
		pr.newline()
		pr.selectLike(v.Select)
		pr.dedent()
		pr.newline()
		pr.write(")")
	case *ast.ExprList:
		pr.write("(")

		for i, e := range v.Items {
			if i > 0 {
				pr.write(", ")
			}

			pr.expr(e)
		}

		pr.write(")")
	default:
		// Defensive fallback for a Node implemented outside this package
		// (ast.Node, unlike ast.Statement, is deliberately open to that —
		// see "Public and extensible" in ast/node.go) — falls back to the
		// node's own debug String() rather than producing no output at all.
		if n != nil {
			pr.write(n.String())
		}
	}
}

func (pr *printer) columnRef(c *ast.ColumnRef) {
	if c.Schema != "" {
		pr.ident(c.Schema)
		pr.write(".")
	}

	if c.Table != "" {
		pr.ident(c.Table)
		pr.write(".")
	}

	pr.ident(c.Column)
}

func (pr *printer) literal(l *ast.Literal) {
	switch l.LitKind {
	case ast.LitString:
		pr.write("'")
		pr.write(strings.ReplaceAll(l.Value, "'", "''"))
		pr.write("'")
	case ast.LitBlob:
		pr.write("X'")
		pr.write(l.Value)
		pr.write("'")
	case ast.LitNull:
		pr.write("NULL")
	case ast.LitBool:
		pr.write(strings.ToUpper(l.Value))
	default:
		// LitInteger and LitFloat: Value already holds a valid numeric
		// spelling straight from the lexer (see scanNumber in lexer.go),
		// so it's written verbatim rather than re-derived.
		pr.write(l.Value)
	}
}

// unaryExpr formats a prefix unary operator. Symbolic operators (-, +, ~)
// are written flush against their operand ("-1"); NOT, the one word-shaped
// unary operator this grammar has, gets a separating space ("NOT x") since
// "NOTx" would just be a different identifier, not the same expression.
func (pr *printer) unaryExpr(u *ast.UnaryExpr) {
	pr.write(u.Op)

	if u.Op == "NOT" {
		pr.write(" ")
	}

	pr.expr(u.X)
}

func (pr *printer) inExpr(v *ast.InExpr) {
	pr.expr(v.X)

	if v.Not {
		pr.write(" NOT")
	}

	pr.write(" IN (")

	switch {
	case v.Sub != nil:
		pr.indent()
		pr.newline()
		pr.selectLike(v.Sub.Select)
		pr.dedent()
		pr.newline()
	default:
		for i, e := range v.List {
			if i > 0 {
				pr.write(", ")
			}

			pr.expr(e)
		}
	}

	pr.write(")")
}

func (pr *printer) likeExpr(v *ast.LikeExpr) {
	pr.expr(v.X)

	if v.Not {
		pr.write(" NOT")
	}

	pr.write(" ")
	pr.write(v.Op)
	pr.write(" ")
	pr.expr(v.Pattern)

	if v.Escape != nil {
		pr.write(" ESCAPE ")
		pr.expr(v.Escape)
	}
}

// funcCall formats a function call. The function name is written as-is,
// never quoted even under PostgreSQL — see "Known limitations" in format.go's
// file comment.
func (pr *printer) funcCall(f *ast.FuncCall) {
	pr.write(f.Name)
	pr.write("(")

	switch {
	case f.Star:
		pr.write("*")
	default:
		if f.Distinct {
			pr.write("DISTINCT ")
		}

		for i, a := range f.Args {
			if i > 0 {
				pr.write(", ")
			}

			pr.expr(a)
		}
	}

	pr.write(")")

	if f.Filter != nil {
		pr.write(" FILTER (WHERE ")
		pr.expr(f.Filter)
		pr.write(")")
	}
}

func (pr *printer) caseExpr(c *ast.CaseExpr) {
	pr.write("CASE")

	if c.Operand != nil {
		pr.write(" ")
		pr.expr(c.Operand)
	}

	for _, w := range c.Whens {
		pr.write(" WHEN ")
		pr.expr(w.Cond)
		pr.write(" THEN ")
		pr.expr(w.Result)
	}

	if c.Else != nil {
		pr.write(" ELSE ")
		pr.expr(c.Else)
	}

	pr.write(" END")
}
