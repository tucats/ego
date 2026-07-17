package format

import (
	"strings"

	"github.com/tucats/ego/internal/language/parse/ast"
)

// printStmt renders a statement or declaration inline at the current cursor
// (the caller is responsible for the leading indentation of the line). Block-
// bearing statements manage their own internal line breaks and indentation.
func (p *printer) printStmt(node ast.Node) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	// Statements.
	case *ast.Block:
		p.printBlock(n)

	case *ast.EmptyStmt:
		// Nothing to emit.

	case *ast.ExprStmt:
		p.printExpr(n.X)

	case *ast.AssignStmt:
		p.printExprList(n.Lhs)
		p.write(" " + n.Op + " ")
		p.printExprList(n.Rhs)

	case *ast.IncDecStmt:
		p.printExpr(n.X)
		p.write(n.Op)

	case *ast.SendStmt:
		p.printExpr(n.Chan)
		p.write(" <- ")
		p.printExpr(n.Value)

	case *ast.ReturnStmt:
		p.write("return")

		if len(n.Results) > 0 {
			p.write(" ")
			p.printExprList(n.Results)
		}

	case *ast.BreakStmt:
		p.write("break")
		p.printOptionalLabel(n.Label)

	case *ast.ContinueStmt:
		p.write("continue")
		p.printOptionalLabel(n.Label)

	case *ast.LabeledStmt:
		p.write(n.Label + ":")
		p.newline()
		p.printStmt(n.Stmt)

	case *ast.DeferStmt:
		p.write("defer ")
		p.printExpr(n.Call)

	case *ast.GoStmt:
		p.write("go ")
		p.printExpr(n.Call)

	case *ast.IfStmt:
		p.printIf(n)

	case *ast.ForStmt:
		p.printFor(n)

	case *ast.SwitchStmt:
		p.printSwitch(n)

	case *ast.TryStmt:
		p.printTry(n)

	case *ast.PanicStmt:
		p.write("panic(")
		p.printExpr(n.Arg)
		p.write(")")

	case *ast.PrintStmt:
		p.printPrint(n)

	case *ast.CallStmt:
		p.write("call ")
		p.printExpr(n.Call)

	case *ast.ThrowStmt:
		p.write("throw ")
		p.printExpr(n.X)

	case *ast.ExitStmt:
		p.write("exit")

		if n.Code != nil {
			p.write(" ")
			p.printExpr(n.Code)
		}

	case *ast.DirectiveStmt:
		p.printDirective(n)

	// Declarations.
	case *ast.PackageDecl:
		p.write("package " + n.Name)

	case *ast.ImportDecl:
		p.printImport(n)

	case *ast.ConstDecl:
		p.printConst(n)

	case *ast.TypeDecl:
		p.write("type " + n.Name.Name + " ")
		p.printExpr(n.Type)

	case *ast.VarDecl:
		p.printVar(n)

	case *ast.FuncDecl:
		p.printFuncDecl(n)

	default:
		// Fall back to expression printing (covers a bare type or expression
		// used where a statement is expected).
		p.printExpr(node)
	}
}

// printBlock renders "{ body }" with the body indented one level. The opening
// brace is written at the cursor (on the current line); an empty block is "{}".
func (p *printer) printBlock(block *ast.Block) {
	if block == nil {
		p.write("{}")

		return
	}

	// An empty block may still carry comments between its braces; keep it "{}"
	// only when there is genuinely nothing (statements or pending inner
	// comments) to place inside.
	innerComments := p.commentsBefore(block.End().Line)

	if len(block.Stmts) == 0 && !innerComments {
		p.write("{}")

		return
	}

	p.write("{")

	p.indent++

	for _, stmt := range block.Stmts {
		p.emitLeadingComments(stmt.Pos().Line)
		p.newline()
		p.printStmt(stmt)
		p.emitTrailingComment(stmt.Pos().Line)
	}

	// Flush comments that fall inside the block after the last statement.
	p.emitLeadingComments(block.End().Line)

	p.indent--
	p.newline()
	p.write("}")
}

// commentsBefore reports whether a pending comment falls on a line before the
// given source line.
func (p *printer) commentsBefore(line int) bool {
	return p.ci < len(p.comments) && p.comments[p.ci].Line < line
}

// printExprList renders a comma-separated list of expressions.
func (p *printer) printExprList(list []ast.Node) {
	for i, expr := range list {
		if i > 0 {
			p.write(", ")
		}

		p.printExpr(expr)
	}
}

// printOptionalLabel appends " label" when label is non-empty.
func (p *printer) printOptionalLabel(label string) {
	if label != "" {
		p.write(" " + label)
	}
}

// printIf renders an if statement, including any else / else-if chain.
func (p *printer) printIf(n *ast.IfStmt) {
	p.write("if ")

	if n.Init != nil {
		p.printStmt(n.Init)
		p.write("; ")
	}

	p.printExpr(n.Cond)
	p.write(" ")
	p.printBlock(n.Body)

	if n.Else == nil {
		return
	}

	p.write(" else ")

	switch e := n.Else.(type) {
	case *ast.IfStmt:
		p.printIf(e)

	case *ast.Block:
		p.printBlock(e)

	default:
		p.printStmt(n.Else)
	}
}

// printFor renders all four for-loop forms.
func (p *printer) printFor(n *ast.ForStmt) {
	p.write("for")

	switch {
	case n.Range != nil:
		p.write(" ")
		p.printExpr(n.Key)

		if n.Value != nil {
			p.write(", ")
			p.printExpr(n.Value)
		}

		if n.Define {
			p.write(" := range ")
		} else {
			p.write(" = range ")
		}

		p.printExpr(n.Range)
	case n.Init != nil || n.Post != nil:
		// Three-clause form.
		p.write(" ")
		p.printStmt(n.Init)
		p.write("; ")
		p.printExpr(n.Cond)
		p.write("; ")
		p.printStmt(n.Post)

	case n.Cond != nil:
		// Conditional form.
		p.write(" ")
		p.printExpr(n.Cond)
	}

	p.write(" ")
	p.printBlock(n.Body)
}

// printSwitch renders a switch statement and its case clauses.
func (p *printer) printSwitch(n *ast.SwitchStmt) {
	p.write("switch")

	if n.Init != nil {
		p.write(" ")
		p.printStmt(n.Init)

		if n.Tag != nil {
			p.write(";")
		}
	}

	if n.Tag != nil {
		p.write(" ")
		p.printExpr(n.Tag)
	}

	p.write(" {")

	// Case clauses sit at the switch's own indentation (gofmt does not indent
	// them past the "switch"); their bodies are indented one level.
	for _, clause := range n.Body {
		p.newline()
		p.printCaseClause(clause)
	}

	p.newline()
	p.write("}")
}

// printCaseClause renders one "case exprs:" or "default:" clause with its body.
func (p *printer) printCaseClause(clause *ast.CaseClause) {
	if clause.Default {
		p.write("default:")
	} else {
		p.write("case ")
		p.printExprList(clause.Exprs)
		p.write(":")
	}

	p.indent++

	for _, stmt := range clause.Body {
		p.emitLeadingComments(stmt.Pos().Line)
		p.newline()
		p.printStmt(stmt)
		p.emitTrailingComment(stmt.Pos().Line)
	}

	if clause.Fallthrough {
		p.newline()
		p.write("fallthrough")
	}

	p.indent--
}

// printTry renders "try { } catch [(var)] { }".
func (p *printer) printTry(n *ast.TryStmt) {
	p.write("try ")
	p.printBlock(n.Body)

	if n.Catch != nil {
		p.write(" catch")

		if n.CatchVar != "" {
			p.write("(" + n.CatchVar + ")")
		}

		p.write(" ")
		p.printBlock(n.Catch)
	}
}

// printPrint renders the "print" extension statement.
func (p *printer) printPrint(n *ast.PrintStmt) {
	p.write("print")

	if len(n.Args) > 0 {
		p.write(" ")
		p.printExprList(n.Args)
	}

	if n.NoNewline {
		p.write(",")
	}
}

// printDirective renders a compile-time directive or macro invocation. Most
// directives (e.g. @assert, @error, @status) take an ordinary expression as
// their argument, so the captured argument tokens are re-parsed and pretty-
// printed to get canonical spacing ("math.Sin(0.0) == 0.0" rather than the raw
// token join "math . Sin ( 0.0 ) == 0.0"). Directives whose arguments are not a
// single expression — anything containing a block, or that fails to re-parse —
// fall back to the raw space-joined token spellings.
func (p *printer) printDirective(n *ast.DirectiveStmt) {
	p.write("@" + n.Name)

	if len(n.RawArgs) == 0 {
		return
	}

	if pretty, ok := prettyDirectiveArgs(n.RawArgs); ok {
		p.write(" " + pretty)

		return
	}

	p.write(" " + strings.Join(n.RawArgs, " "))
}

// prettyDirectiveArgs attempts to render directive argument tokens as a
// canonically-formatted single-line expression. It returns false (so the caller
// uses the raw join) when the arguments contain a block, do not re-parse as a
// single statement, or would format across multiple lines.
func prettyDirectiveArgs(rawArgs []string) (string, bool) {
	for _, tok := range rawArgs {
		if tok == "{" || tok == "}" || tok == "{}" {
			return "", false
		}
	}

	// Re-parse the joined tokens. Formatting is delegated to Node, which shares
	// this package's printer, so the result is canonical.
	out, err := Source(strings.Join(rawArgs, " "), true)
	if err != nil {
		return "", false
	}

	out = strings.TrimRight(out, "\n")
	if out == "" || strings.Contains(out, "\n") {
		return "", false
	}

	return out, true
}

// printImport renders an import declaration, grouped or single.
func (p *printer) printImport(n *ast.ImportDecl) {
	if n.Parenthesized || len(n.Specs) > 1 {
		p.write("import (")

		p.indent++

		for _, spec := range n.Specs {
			p.newline()
			p.printImportSpec(spec)
		}

		p.indent--
		p.newline()
		p.write(")")

		return
	}

	p.write("import ")

	if len(n.Specs) == 1 {
		p.printImportSpec(n.Specs[0])
	}
}

// printImportSpec renders "[alias] \"path\"".
func (p *printer) printImportSpec(spec *ast.ImportSpec) {
	if spec.Alias != "" {
		p.write(spec.Alias + " ")
	}

	p.write(quoteString(spec.Path))
}

// printConst renders a const declaration, grouped or single.
func (p *printer) printConst(n *ast.ConstDecl) {
	if n.Parenthesized || len(n.Specs) > 1 {
		p.write("const (")

		p.indent++

		for _, spec := range n.Specs {
			p.newline()
			p.printConstSpec(spec)
		}

		p.indent--
		p.newline()
		p.write(")")

		return
	}

	p.write("const ")

	if len(n.Specs) == 1 {
		p.printConstSpec(n.Specs[0])
	}
}

// printConstSpec renders "Name = value" (the "= value" clause is optional).
func (p *printer) printConstSpec(spec *ast.ConstSpec) {
	p.write(spec.Name.Name)

	if spec.Value != nil {
		p.write(" = ")
		p.printExpr(spec.Value)
	}
}

// printVar renders a var declaration, grouped or single.
func (p *printer) printVar(n *ast.VarDecl) {
	if n.Parenthesized || len(n.Specs) > 1 {
		p.write("var (")

		p.indent++

		for _, spec := range n.Specs {
			p.newline()
			p.printVarSpec(spec)
		}

		p.indent--
		p.newline()
		p.write(")")

		return
	}

	p.write("var ")

	if len(n.Specs) == 1 {
		p.printVarSpec(n.Specs[0])
	}
}

// printVarSpec renders "names [type] [= values]".
func (p *printer) printVarSpec(spec *ast.VarSpec) {
	for i, name := range spec.Names {
		if i > 0 {
			p.write(", ")
		}

		p.write(name.Name)
	}

	if spec.Type != nil {
		p.write(" ")
		p.printExpr(spec.Type)
	}

	if len(spec.Values) > 0 {
		p.write(" = ")
		p.printExprList(spec.Values)
	}
}

// printFuncDecl renders a named function or method definition.
func (p *printer) printFuncDecl(n *ast.FuncDecl) {
	p.write("func ")

	if n.Recv != nil {
		p.write("(")
		p.write(n.Recv.Name.Name + " ")

		if n.Recv.Pointer {
			p.write("*")
		}

		p.write(n.Recv.Type.Name + ") ")
	}

	p.write(n.Name.Name)
	p.printSignature(n.Type)
	p.write(" ")
	p.printBlock(n.Body)
}
