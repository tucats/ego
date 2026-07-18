package resolve

import (
	"testing"

	"github.com/tucats/ego/internal/language/parse"
	"github.com/tucats/ego/internal/language/parse/ast"
)

// mustResolveProgram parses src as a full program and resolves it.
func mustResolveProgram(t *testing.T, src string) (*Info, []Diagnostic) {
	t.Helper()

	file, err := parse.ParseProgram(src)
	if err != nil {
		t.Fatalf("ParseProgram(%q) unexpected error: %v", src, err)
	}

	return Resolve(file)
}

// mustResolveBare parses src as a bare statement fragment and resolves it.
func mustResolveBare(t *testing.T, src string) (*Info, []Diagnostic) {
	t.Helper()

	file, err := parse.ParseStatements(src)
	if err != nil {
		t.Fatalf("ParseStatements(%q) unexpected error: %v", src, err)
	}

	return Resolve(file)
}

// symbolsNamed returns every Symbol in info.Symbols with the given name, in
// declaration order -- used by tests that need to distinguish two distinct
// declarations sharing a name (e.g. a shadowing test).
func symbolsNamed(info *Info, name string) []*Symbol {
	var out []*Symbol

	for _, s := range info.Symbols {
		if s.Name == name {
			out = append(out, s)
		}
	}

	return out
}

// oneSymbolNamed returns the single Symbol named name, failing the test if
// there is not exactly one.
func oneSymbolNamed(t *testing.T, info *Info, name string) *Symbol {
	t.Helper()

	syms := symbolsNamed(info, name)
	if len(syms) != 1 {
		t.Fatalf("expected exactly one symbol named %q, found %d", name, len(syms))
	}

	return syms[0]
}

func diagKinds(diags []Diagnostic) []DiagnosticKind {
	kinds := make([]DiagnosticKind, len(diags))
	for i, d := range diags {
		kinds[i] = d.Kind
	}

	return kinds
}

func TestResolveParamAndVarTypes(t *testing.T) {
	info, diags := mustResolveProgram(t, `
func f(a int, b string) {
	var x float64
	const y = 1
	_ = a
	_ = b
	_ = x
	_ = y
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	a := oneSymbolNamed(t, info, "a")
	if a.Kind != KindParam {
		t.Errorf("a.Kind = %v, want KindParam", a.Kind)
	}

	primType, ok := a.Type.(*ast.PrimitiveType)
	if !ok || primType.Name != "int" {
		t.Errorf("a.Type = %#v, want PrimitiveType(int)", a.Type)
	}

	x := oneSymbolNamed(t, info, "x")
	if primType, ok := x.Type.(*ast.PrimitiveType); !ok || primType.Name != "float64" {
		t.Errorf("x.Type = %#v, want PrimitiveType(float64)", x.Type)
	}

	y := oneSymbolNamed(t, info, "y")
	if y.Kind != KindConst {
		t.Errorf("y.Kind = %v, want KindConst", y.Kind)
	}

	if primType, ok := y.Type.(*ast.PrimitiveType); !ok || primType.Name != "int" {
		t.Errorf("y.Type = %#v, want PrimitiveType(int) inferred from literal 1", y.Type)
	}
}

func TestResolveShadowingAndPendingDeclaration(t *testing.T) {
	// The inner "x := x + 1" must resolve its RHS "x" to the OUTER binding
	// (matching docs/SLOTS.md's documented pending-declaration semantics: a
	// name being declared is not yet in scope while its own RHS is
	// evaluated), and the inner "x" must shadow the outer one for the
	// reference that follows it.
	info, diags := mustResolveBare(t, `
x := 1
{
	x := x + 1
	_ = x
}
_ = x
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	syms := symbolsNamed(info, "x")
	if len(syms) != 2 {
		t.Fatalf("expected 2 symbols named x (outer + inner), got %d", len(syms))
	}

	outer, inner := syms[0], syms[1]
	if outer.Scope == inner.Scope {
		t.Fatalf("outer and inner x unexpectedly share a scope")
	}

	// Outer x: used by the inner "x + 1" RHS and the final "_ = x".
	if len(outer.Uses) != 2 {
		t.Errorf("outer x Uses = %d, want 2", len(outer.Uses))
	}

	// Inner x: used only by the "_ = x" inside the block.
	if len(inner.Uses) != 1 {
		t.Errorf("inner x Uses = %d, want 1", len(inner.Uses))
	}
}

func TestResolveUnusedVariable(t *testing.T) {
	_, diags := mustResolveProgram(t, `
func f() {
	y := 1
}
`)
	if len(diags) != 1 || diags[0].Kind != UnusedSymbol || diags[0].Name != "y" {
		t.Fatalf("diags = %+v, want single UnusedSymbol(y)", diags)
	}
}

func TestResolveUnusedVariableExemptsParamsAndReturns(t *testing.T) {
	// Go's "declared and not used" rule only applies to local var/const
	// declarations -- an unused parameter or named return is never flagged.
	_, diags := mustResolveProgram(t, `
func f(a int) (result int) {
	return 0
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}

func TestResolvePlainAssignDoesNotCountAsUse(t *testing.T) {
	// A bare "x = 6" write, with no subsequent read, does not satisfy Go's
	// "declared and not used" rule -- only a read does.
	_, diags := mustResolveProgram(t, `
func f() {
	x := 5
	x = 6
}
`)
	if len(diags) != 1 || diags[0].Kind != UnusedSymbol || diags[0].Name != "x" {
		t.Fatalf("diags = %+v, want single UnusedSymbol(x)", diags)
	}
}

func TestResolveCompoundAssignCountsAsUse(t *testing.T) {
	// Unlike a plain "=", a compound assignment reads the old value first,
	// so it does satisfy "declared and not used".
	_, diags := mustResolveProgram(t, `
func f() {
	x := 5
	x += 1
	_ = x
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}

func TestResolveUsedBeforeDefined(t *testing.T) {
	_, diags := mustResolveProgram(t, `
func f() {
	fmt.Println(x)
	x := 1
	_ = x
}
`)
	if len(diags) != 1 {
		t.Fatalf("diags = %+v, want exactly 1", diags)
	}

	if diags[0].Kind != UsedBeforeDefined || diags[0].Name != "x" {
		t.Errorf("diags[0] = %+v, want UsedBeforeDefined(x)", diags[0])
	}
}

func TestResolveGenuinelyUndefined(t *testing.T) {
	_, diags := mustResolveProgram(t, `
func f() {
	_ = neverDeclared
}
`)
	if len(diags) != 1 || diags[0].Kind != UndefinedSymbol || diags[0].Name != "neverDeclared" {
		t.Fatalf("diags = %+v, want single UndefinedSymbol(neverDeclared)", diags)
	}
}

func TestResolveClosureCaptureMarksHeap(t *testing.T) {
	info, diags := mustResolveProgram(t, `
func outer() {
	x := 1
	f := func() int {
		return x
	}
	_ = f
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	x := oneSymbolNamed(t, info, "x")
	if !x.Captured {
		t.Errorf("x.Captured = false, want true")
	}

	if x.Storage != StorageHeap {
		t.Errorf("x.Storage = %v, want StorageHeap", x.Storage)
	}

	if len(x.CapturedBy) != 1 {
		t.Errorf("x.CapturedBy = %v, want exactly one entry", x.CapturedBy)
	}

	f := oneSymbolNamed(t, info, "f")
	if f.Captured {
		t.Errorf("f.Captured = true, want false")
	}

	if f.Storage != StorageRegister {
		t.Errorf("f.Storage = %v, want StorageRegister", f.Storage)
	}
}

func TestResolveShadowedParamIsNotCapture(t *testing.T) {
	// g's own parameter is also named x, so "return x" inside g resolves to
	// g's own parameter, not outer's local -- outer's x is never referenced
	// from within the closure and must stay Register.
	info, diags := mustResolveProgram(t, `
func outer() {
	x := 1
	g := func(x int) int {
		return x
	}
	_ = g
	_ = x
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	outerXs := symbolsNamed(info, "x")
	if len(outerXs) != 2 {
		t.Fatalf("expected 2 symbols named x (outer local + g's param), got %d", len(outerXs))
	}

	outerX := outerXs[0]
	if outerX.Captured {
		t.Errorf("outer x.Captured = true, want false")
	}

	if outerX.Storage != StorageRegister {
		t.Errorf("outer x.Storage = %v, want StorageRegister", outerX.Storage)
	}

	if len(outerX.Uses) != 1 {
		t.Errorf("outer x.Uses = %d, want 1 (only the trailing \"_ = x\")", len(outerX.Uses))
	}
}

func TestResolveMutualRecursionAtFileScope(t *testing.T) {
	// isEven references isOdd, which is declared later in the source --
	// this must resolve regardless of order, the same way the current
	// token-based compiler defers "unknown symbol" reporting for exactly
	// this case.
	_, diags := mustResolveProgram(t, `
func isEven(n int) bool {
	if n == 0 {
		return true
	}
	return isOdd(n - 1)
}
func isOdd(n int) bool {
	if n == 0 {
		return false
	}
	return isEven(n - 1)
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}

func TestResolvePackageSelectorNotMisflagged(t *testing.T) {
	info, diags := mustResolveProgram(t, `
import "fmt"

func f() {
	fmt.Println("hi")
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	fmtSym := oneSymbolNamed(t, info, "fmt")
	if fmtSym.Kind != KindImport {
		t.Errorf("fmt.Kind = %v, want KindImport", fmtSym.Kind)
	}

	if len(fmtSym.Uses) != 1 {
		t.Errorf("fmt.Uses = %d, want 1", len(fmtSym.Uses))
	}
}

func TestResolveAutoImportedPackageWithoutExplicitImport(t *testing.T) {
	// No "import" statement at all -- fmt must still resolve via the
	// builtin allow-list (see builtins.go), matching the auto-import
	// behavior CLAUDE.md documents for `ego test` compilations.
	_, diags := mustResolveProgram(t, `
func f() {
	fmt.Println("hi")
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}

func TestResolveStructFieldNameNotMisflaggedAsVariable(t *testing.T) {
	// "X" and "Y" are struct field names in this composite literal, not
	// variable references -- they must never be resolved/flagged.
	_, diags := mustResolveProgram(t, `
type Point struct {
	X int
	Y int
}
func f() {
	p := Point{X: 1, Y: 2}
	_ = p
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}

func TestResolveForLoopVariableScoping(t *testing.T) {
	info, diags := mustResolveProgram(t, `
func f() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	i := oneSymbolNamed(t, info, "i")
	if i.Kind != KindVar {
		t.Errorf("i.Kind = %v, want KindVar", i.Kind)
	}
}

func TestResolveIfInitScopedToIfElse(t *testing.T) {
	// The if-init variable must be visible in both the "then" and "else"
	// arms but must not leak into the surrounding function scope.
	info, diags := mustResolveProgram(t, `
func f() bool {
	if v := compute(); v > 0 {
		return v > 1
	} else {
		return v == 0
	}
}
func compute() int {
	return 1
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	v := oneSymbolNamed(t, info, "v")
	if len(v.Uses) != 3 {
		t.Errorf("v.Uses = %d, want 3 (the if condition, then-arm, else-arm)", len(v.Uses))
	}
}

func TestResolveMethodNotRegisteredAtFileScope(t *testing.T) {
	// A method (with a receiver) is not directly callable by bare name, so
	// it must not appear as a file-scope symbol -- but its body is still
	// walked (an unused local inside it is still flagged).
	info, diags := mustResolveProgram(t, `
type T struct{}

func (t T) Method() {
	unused := 1
}
`)
	if syms := symbolsNamed(info, "Method"); len(syms) != 0 {
		t.Errorf("Method registered as a file-scope symbol: %+v", syms)
	}

	if len(diags) != 1 || diags[0].Kind != UnusedSymbol || diags[0].Name != "unused" {
		t.Fatalf("diags = %+v, want single UnusedSymbol(unused)", diags)
	}
}

func TestResolveBareFragmentLocalsAreRegisterEligible(t *testing.T) {
	// A bare fragment (REPL/@test style) is treated as a single implicit
	// function activation: its top-level locals are classified like any
	// other function's, not as PackageGlobal.
	info, diags := mustResolveBare(t, `
x := 1
_ = x
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	x := oneSymbolNamed(t, info, "x")
	if x.Storage != StorageRegister {
		t.Errorf("x.Storage = %v, want StorageRegister", x.Storage)
	}
}

func TestResolveFileScopeVarIsPackageGlobal(t *testing.T) {
	info, diags := mustResolveProgram(t, `
var count int

func f() {
	count = 1
}
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	count := oneSymbolNamed(t, info, "count")
	if count.Storage != StoragePackageGlobal {
		t.Errorf("count.Storage = %v, want StoragePackageGlobal", count.Storage)
	}
}

func TestResolveFileScopeVarNeverFlaggedUnused(t *testing.T) {
	// Single-file limitation: another file in the same package could use
	// this, so a file-scope declaration is never reported UnusedSymbol.
	_, diags := mustResolveProgram(t, `
var neverReferenced int
`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}
