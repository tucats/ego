package compiler

// testing_body_test.go exercises the @test guard mechanism added to fix the
// original bug: a compile error, or an uncaught runtime error (e.g. a failed
// @assert), inside ONE @test block used to abort compilation/execution of
// the ENTIRE file, silently discarding every other test in it. See
// compileTestBody, collectTestBodyTokens, emitTestPass, and emitTestFail in
// testing.go for the implementation, and docs/TESTING.md for the
// user-facing behavior.
//
// These tests deliberately do NOT live under tests/*.ego: a test file that
// intentionally fails to compile or fails an @assert would make `ego test`
// (and any CI job driving it) report a failure on every run, which is not
// what a regression test for "failures are now survivable" should do. Go
// tests exercising the compiler API directly are the right tool here.
//
// runTestFile below mirrors the compilation/execution setup
// internal/commands/test.go's TestAction performs for `ego test`, trimmed to
// what these tests need. ui.QuietMode is set for the duration of each test so
// the "TEST: ... (PASS)/(FAIL)" lines these test files produce (via ui.Say,
// which writes directly to the real process stdout -- see
// docs/TESTING.md's "Output capture" section -- and is therefore not
// visible through Context.GetOutput()) don't clutter `go test` output.
// What's asserted instead is exactly what a caller like `ego test` itself
// depends on: whether Compile/Run returned an error at all, and the final
// _testcount/_testfailcount values in the root symbol table.

import (
	"testing"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// runTestFile compiles and executes text as an Ego test file (test mode,
// interactive/no-main, auto-imported standard packages -- the same
// combination internal/commands/test.go's TestAction uses), and returns the
// compile error (if compilation failed), the run error (if execution failed
// or was aborted), and the final passed/failed test counts.
func runTestFile(t *testing.T, text string) (compileErr, runErr error, passed, failed int) {
	t.Helper()

	previousQuiet := ui.QuietMode
	ui.QuietMode = true

	t.Cleanup(func() { ui.QuietMode = previousQuiet })

	symbols.RootSymbolTable.SetAlways("_testcount", 0)
	symbols.RootSymbolTable.SetAlways("_testfailcount", 0)

	symbolTable := symbols.NewSymbolTable("test").Shared(true)
	symbolTable.SetAlways(defs.ModeVariable, "test")

	comp := New(t.Name()).SetTestMode(true)

	AddStandard(symbolTable)

	if err := comp.AutoImport(true, symbolTable); err != nil {
		t.Fatalf("AutoImport failed: %v", err)
	}

	for _, packageName := range GetAutoImportedPackages() {
		comp.DefineGlobalSymbol(packageName)
	}

	comp.SetInteractive(true)

	tok := tokenizer.New(text, true)

	bc, err := comp.Compile(t.Name(), tok)
	if err != nil {
		compileErr = err
	} else {
		ctx := bytecode.NewContext(symbolTable, bc)
		ctx.EnableConsoleOutput(false)

		runErr = ctx.Run()
	}

	if v, found := symbols.RootSymbolTable.Get("_testcount"); found {
		passed, _ = data.Int(v)
	}

	if v, found := symbols.RootSymbolTable.Get("_testfailcount"); found {
		failed, _ = data.Int(v)
	}

	return compileErr, runErr, passed, failed
}

// TestCompileTestBody_CompileErrorDoesNotAbortFile is the primary regression
// test: a compile error inside the FIRST of three @test blocks must not
// prevent the file from compiling, or the other two tests from running.
func TestCompileTestBody_CompileErrorDoesNotAbortFile(t *testing.T) {
	const source = `
@test "first, has a compile error"
{
    pring "this is a typo, not a real statement"
}

@test "second, should still run"
{
    @assert 1 == 1
}

@test "third, also runs"
{
    @assert 2 == 2
}
`

	compileErr, runErr, passed, failed := runTestFile(t, source)

	if compileErr != nil {
		t.Fatalf("Compile() returned an error; a compile error inside one @test must not fail the whole file's compilation: %v", compileErr)
	}

	if runErr != nil {
		t.Fatalf("Run() returned an error: %v", runErr)
	}

	if passed != 2 {
		t.Errorf("passed count = %d, want 2 (second and third)", passed)
	}

	if failed != 1 {
		t.Errorf("failed count = %d, want 1 (first)", failed)
	}
}

// TestCompileTestBody_RuntimeAssertFailureDoesNotAbortFile verifies the
// runtime counterpart: a failed @assert (an uncaught but catchable runtime
// error) inside one @test must not prevent the rest of the file's tests
// from running, and must not make ctx.Run() itself return an error.
func TestCompileTestBody_RuntimeAssertFailureDoesNotAbortFile(t *testing.T) {
	const source = `
@test "first, fails an assert at runtime"
{
    @assert 1 == 2
}

@test "second, should still run"
{
    @assert 1 == 1
}

@test "third, also runs"
{
    x := 5
    @assert x == 5
}
`

	compileErr, runErr, passed, failed := runTestFile(t, source)

	if compileErr != nil {
		t.Fatalf("Compile() returned an unexpected error: %v", compileErr)
	}

	if runErr != nil {
		t.Fatalf("Run() returned an error; a failed @assert in one test must not abort the run: %v", runErr)
	}

	if passed != 2 {
		t.Errorf("passed count = %d, want 2 (second and third)", passed)
	}

	if failed != 1 {
		t.Errorf("failed count = %d, want 1 (first)", failed)
	}
}

// TestCompileTestBody_AllPass is the baseline happy-path case: every test
// passes, so the counts should reflect that and nothing should error.
func TestCompileTestBody_AllPass(t *testing.T) {
	const source = `
@test "alpha"
{
    @assert 1 == 1
}

@test "beta"
{
    @assert 2 == 2
}

@test "gamma"
{
    @assert 3 == 3
}
`

	compileErr, runErr, passed, failed := runTestFile(t, source)

	if compileErr != nil {
		t.Fatalf("Compile() returned an unexpected error: %v", compileErr)
	}

	if runErr != nil {
		t.Fatalf("Run() returned an unexpected error: %v", runErr)
	}

	if passed != 3 {
		t.Errorf("passed count = %d, want 3", passed)
	}

	if failed != 0 {
		t.Errorf("failed count = %d, want 0", failed)
	}
}

// TestCompileTestBody_MultipleCompileErrorsAllReported verifies that
// compile errors in MORE than one test are each independently caught and
// reported -- not just the first one -- and that tests in between and after
// still run normally.
func TestCompileTestBody_MultipleCompileErrorsAllReported(t *testing.T) {
	const source = `
@test "first, has a compile error"
{
    pring "typo one"
}

@test "second, runs fine"
{
    @assert 1 == 1
}

@test "third, also has a compile error"
{
    pring "typo two"
}

@test "fourth, runs fine"
{
    @assert 1 == 1
}
`

	compileErr, runErr, passed, failed := runTestFile(t, source)

	if compileErr != nil {
		t.Fatalf("Compile() returned an unexpected error: %v", compileErr)
	}

	if runErr != nil {
		t.Fatalf("Run() returned an unexpected error: %v", runErr)
	}

	if passed != 2 {
		t.Errorf("passed count = %d, want 2 (second and fourth)", passed)
	}

	if failed != 2 {
		t.Errorf("failed count = %d, want 2 (first and third)", failed)
	}
}

// TestCompileTestBody_FailDirectiveStillAborts documents and locks in a
// deliberate, unchanged behavior: unlike a failed @assert, @fail is designed
// to be an unconditional, unrecoverable stop (it flushes the try/catch stack
// before signaling -- see Fail() in testing.go -- specifically so it cannot
// be caught, including by compileTestBody's own guard around each test).
// This is intentional: @fail exists for flow-of-control checks where
// reaching a certain line is itself the bug, and test authors reach for it
// specifically because it stops everything immediately. A test after an
// @fail must NOT run.
func TestCompileTestBody_FailDirectiveStillAborts(t *testing.T) {
	const source = `
@test "first, uses @fail"
{
    @fail "deliberate hard stop"
}

@test "second, must not run"
{
    @assert 1 == 1
}
`

	compileErr, runErr, passed, failed := runTestFile(t, source)

	if compileErr != nil {
		t.Fatalf("Compile() returned an unexpected error: %v", compileErr)
	}

	if runErr == nil {
		t.Fatal("Run() returned nil; @fail must still abort the run")
	}

	if passed != 0 || failed != 0 {
		t.Errorf("passed=%d failed=%d, want 0/0 -- @fail must abort before either test closes out", passed, failed)
	}
}

// TestCompileTestBody_SharedTopLevelValueUsedByOneTest is a regression test
// for the stack-overflow bug found while implementing this feature: a value
// declared once at file scope (outside any @test block, and therefore
// shared across all of them -- see docs/TESTING.md's "Global scope"
// section) but referenced by only SOME of several @test blocks used to
// corrupt compiler state. Each @test body now compiles in its own clone of
// the compiler (see compileTestBody's doc comment), and those clones share
// the file-level scope's underlying usage-tracking map by reference; calling
// Errors() per test (which was the original bug) walked that shared map and
// re-chained the same *errors.Error values with Chain()'s in-place, mutating
// linked-list semantics on every single test, eventually forming a cycle and
// crashing with a stack overflow the next time anything walked the chain.
// This test reproduces the shape that triggered it: several shared
// top-level values, more tests than values, and no single test using all of
// them.
func TestCompileTestBody_SharedTopLevelValueUsedByOneTest(t *testing.T) {
	const source = `
shared1 := "a"
shared2 := "b"
shared3 := "c"

@test "uses shared1 only"
{
    @assert shared1 == "a"
}

@test "uses shared2 only"
{
    @assert shared2 == "b"
}

@test "uses shared3 only"
{
    @assert shared3 == "c"
}

@test "uses shared1 again"
{
    @assert shared1 == "a"
}

@test "uses none of them"
{
    @assert 1 == 1
}
`

	compileErr, runErr, passed, failed := runTestFile(t, source)

	if compileErr != nil {
		t.Fatalf("Compile() returned an unexpected error: %v", compileErr)
	}

	if runErr != nil {
		t.Fatalf("Run() returned an unexpected error: %v", runErr)
	}

	if passed != 5 {
		t.Errorf("passed count = %d, want 5", passed)
	}

	if failed != 0 {
		t.Errorf("failed count = %d, want 0", failed)
	}
}
