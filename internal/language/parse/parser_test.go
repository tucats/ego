package parse

import (
	"testing"

	"github.com/tucats/ego/internal/language/parse/ast"
)

// mustParseStmts parses a bare statement fragment and fails the test on error.
func mustParseStmts(t *testing.T, src string) *ast.File {
	t.Helper()

	file, err := ParseStatements(src)
	if err != nil {
		t.Fatalf("ParseStatements(%q) unexpected error: %v", src, err)
	}

	return file
}

// firstStmt returns the first top-level node of a parsed fragment.
func firstStmt(t *testing.T, src string) ast.Node {
	t.Helper()

	file := mustParseStmts(t, src)
	if len(file.Decls) == 0 {
		t.Fatalf("ParseStatements(%q) produced no statements", src)
	}

	return file.Decls[0]
}

func TestParseLiterals(t *testing.T) {
	cases := []struct {
		src  string
		kind ast.LitKind
	}{
		{"42", ast.LitInt},
		{"3.14", ast.LitFloat},
		{"2.5i", ast.LitImaginary},
		{"true", ast.LitBool},
		{`"hello"`, ast.LitString},
		{"nil", ast.LitNil},
	}

	for _, c := range cases {
		stmt := firstStmt(t, c.src)

		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			t.Errorf("%q: expected ExprStmt, got %T", c.src, stmt)

			continue
		}

		lit, ok := exprStmt.X.(*ast.BasicLit)
		if !ok {
			t.Errorf("%q: expected BasicLit, got %T", c.src, exprStmt.X)

			continue
		}

		if lit.LitKind != c.kind {
			t.Errorf("%q: expected lit kind %v, got %v", c.src, c.kind, lit.LitKind)
		}
	}
}

func TestParseBinaryPrecedence(t *testing.T) {
	// 1 + 2 * 3 should parse as 1 + (2 * 3).
	stmt := firstStmt(t, "x := 1 + 2 * 3")

	assign, ok := stmt.(*ast.AssignStmt)
	if !ok {
		t.Fatalf("expected AssignStmt, got %T", stmt)
	}

	bin, ok := assign.Rhs[0].(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", assign.Rhs[0])
	}

	if bin.Op != "+" {
		t.Fatalf("expected top operator '+', got %q", bin.Op)
	}

	right, ok := bin.Y.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected right operand BinaryExpr, got %T", bin.Y)
	}

	if right.Op != "*" {
		t.Errorf("expected inner operator '*', got %q", right.Op)
	}
}

func TestParseAssignments(t *testing.T) {
	cases := []struct {
		src    string
		op     string
		lhsLen int
		rhsLen int
	}{
		{"x := 5", ":=", 1, 1},
		{"x = 5", "=", 1, 1},
		{"x += 1", "+=", 1, 1},
		{"a, b := 1, 2", ":=", 2, 2},
		{"a, b = f()", "=", 2, 1},
	}

	for _, c := range cases {
		stmt := firstStmt(t, c.src)

		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			t.Errorf("%q: expected AssignStmt, got %T", c.src, stmt)

			continue
		}

		if assign.Op != c.op {
			t.Errorf("%q: op = %q, want %q", c.src, assign.Op, c.op)
		}

		if len(assign.Lhs) != c.lhsLen {
			t.Errorf("%q: len(Lhs) = %d, want %d", c.src, len(assign.Lhs), c.lhsLen)
		}

		if len(assign.Rhs) != c.rhsLen {
			t.Errorf("%q: len(Rhs) = %d, want %d", c.src, len(assign.Rhs), c.rhsLen)
		}
	}
}

func TestParseCallAndSelector(t *testing.T) {
	stmt := firstStmt(t, `fmt.Println("hi", x)`)

	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", stmt)
	}

	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", exprStmt.X)
	}

	if len(call.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(call.Args))
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expected SelectorExpr fun, got %T", call.Fun)
	}

	if sel.Sel.Name != "Println" {
		t.Errorf("selector = %q, want Println", sel.Sel.Name)
	}
}

func TestParseIfElse(t *testing.T) {
	stmt := firstStmt(t, "if x := f(); x > 0 { return x } else { return 0 }")

	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt, got %T", stmt)
	}

	if ifStmt.Init == nil {
		t.Error("expected init statement")
	}

	if ifStmt.Cond == nil {
		t.Error("expected condition")
	}

	if _, ok := ifStmt.Else.(*ast.Block); !ok {
		t.Errorf("expected else Block, got %T", ifStmt.Else)
	}
}

func TestParseForForms(t *testing.T) {
	cases := []struct {
		src   string
		check func(*ast.ForStmt) bool
	}{
		{"for { break }", func(f *ast.ForStmt) bool {
			return f.Cond == nil && f.Range == nil && f.Init == nil
		}},
		{"for x < 10 { x++ }", func(f *ast.ForStmt) bool {
			return f.Cond != nil && f.Init == nil && f.Range == nil
		}},
		{"for i := 0; i < 10; i++ { }", func(f *ast.ForStmt) bool {
			return f.Init != nil && f.Cond != nil && f.Post != nil
		}},
		{"for k, v := range items { }", func(f *ast.ForStmt) bool {
			return f.Range != nil && f.Key != nil && f.Value != nil && f.Define
		}},
	}

	for _, c := range cases {
		stmt := firstStmt(t, c.src)

		forStmt, ok := stmt.(*ast.ForStmt)
		if !ok {
			t.Errorf("%q: expected ForStmt, got %T", c.src, stmt)

			continue
		}

		if !c.check(forStmt) {
			t.Errorf("%q: for-loop fields did not match expected form", c.src)
		}
	}
}

func TestParseSwitch(t *testing.T) {
	src := `switch x {
case 1, 2:
	return "low"
case 3:
	fallthrough
default:
	return "other"
}`

	stmt := firstStmt(t, src)

	sw, ok := stmt.(*ast.SwitchStmt)
	if !ok {
		t.Fatalf("expected SwitchStmt, got %T", stmt)
	}

	if sw.Tag == nil {
		t.Error("expected switch tag")
	}

	if len(sw.Body) != 3 {
		t.Fatalf("expected 3 clauses, got %d", len(sw.Body))
	}

	if len(sw.Body[0].Exprs) != 2 {
		t.Errorf("first case: expected 2 exprs, got %d", len(sw.Body[0].Exprs))
	}

	if !sw.Body[1].Fallthrough {
		t.Error("second case: expected fallthrough")
	}

	if !sw.Body[2].Default {
		t.Error("third clause: expected default")
	}
}

func TestParseFuncDecl(t *testing.T) {
	src := `func add(a, b int) int { return a + b }`

	file, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("ParseProgram error: %v", err)
	}

	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", file.Decls[0])
	}

	if fn.Name.Name != "add" {
		t.Errorf("name = %q, want add", fn.Name.Name)
	}

	if len(fn.Type.Params) != 1 {
		t.Fatalf("expected 1 param group, got %d", len(fn.Type.Params))
	}

	if len(fn.Type.Params[0].Names) != 2 {
		t.Errorf("expected 2 shared names, got %d", len(fn.Type.Params[0].Names))
	}

	if len(fn.Type.Returns) != 1 {
		t.Errorf("expected 1 return, got %d", len(fn.Type.Returns))
	}
}

func TestParseMethodDecl(t *testing.T) {
	src := `func (r *Rect) Area() float64 { return r.w * r.h }`

	file, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("ParseProgram error: %v", err)
	}

	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", file.Decls[0])
	}

	if fn.Recv == nil {
		t.Fatal("expected receiver")
	}

	if !fn.Recv.Pointer {
		t.Error("expected pointer receiver")
	}

	if fn.Recv.Type.Name != "Rect" {
		t.Errorf("receiver type = %q, want Rect", fn.Recv.Type.Name)
	}
}

func TestParseDeclarations(t *testing.T) {
	src := `package main

import (
	"fmt"
	str "strings"
)

const Pi = 3.14

type Point struct {
	X, Y int
}

var count int = 0
`

	file, err := ParseProgram(src)
	if err != nil {
		t.Fatalf("ParseProgram error: %v", err)
	}

	var (
		pkg, imp, con, typ, vr bool
	)

	for _, d := range file.Decls {
		switch n := d.(type) {
		case *ast.PackageDecl:
			pkg = n.Name == "main"
		case *ast.ImportDecl:
			imp = len(n.Specs) == 2 && n.Specs[1].Alias == "str"
		case *ast.ConstDecl:
			con = len(n.Specs) == 1
		case *ast.TypeDecl:
			_, isStruct := n.Type.(*ast.StructType)
			typ = isStruct
		case *ast.VarDecl:
			vr = len(n.Specs) == 1
		}
	}

	if !pkg || !imp || !con || !typ || !vr {
		t.Errorf("missing decls: pkg=%v imp=%v const=%v type=%v var=%v", pkg, imp, con, typ, vr)
	}
}

func TestParseTypeConstructs(t *testing.T) {
	// Exercise composite literals, indexing, maps, and closures together.
	src := `
m := map[string]int{"a": 1, "b": 2}
s := []int{1, 2, 3}
p := Point{X: 1, Y: 2}
f := func(x int) int { return x * 2 }
v := s[0]
`

	if _, err := ParseStatements(src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseChannels(t *testing.T) {
	src := `
ch := make(chan, 10)
ch <- 5
x := <-ch
`

	file := mustParseStmts(t, src)

	if len(file.Decls) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(file.Decls))
	}

	if _, ok := file.Decls[1].(*ast.SendStmt); !ok {
		t.Errorf("expected SendStmt, got %T", file.Decls[1])
	}
}

func TestWalkVisitsChildren(t *testing.T) {
	file := mustParseStmts(t, "x := 1 + 2 * 3")

	count := 0

	ast.Walk(file, func(ast.Node) bool {
		count++

		return true
	})

	// File, AssignStmt, Ident(x), BinaryExpr(+), BasicLit(1),
	// BinaryExpr(*), BasicLit(2), BasicLit(3) = 8 nodes.
	if count < 8 {
		t.Errorf("Walk visited %d nodes, expected at least 8", count)
	}
}
