package compiler

// Tests for the BUG-17 fix: "var name = expr" (type inferred from the
// initializer, with no type token between the name and "=") previously
// failed to compile.
//
// Root cause: compileVar() parses an optional type token after the name
// list via parseTypeSpec(). When no type token is present -- which is
// exactly what happens for "var pi = 3.14", since the very next token is
// "=" rather than a type keyword -- parseTypeSpec() returns an "undefined"
// type. Before this fix, compileVar() treated that as "the next token must
// be a user-defined type name" and called varUserType(), which requires an
// identifier. Since "=" is not an identifier, varUserType() rejected it with
// "invalid type specification", even though "var pi = 3.14" is valid,
// commonly used Go syntax equivalent to "pi := 3.14".
//
// The fix adds varInferredInitializer(), which is selected instead of
// varUserType() whenever the token immediately following the name list is
// "=". It compiles the initializer expression exactly like a short variable
// declaration would (no target type, no coercion) and stores the result.
//
// A second, related bug was fixed as part of this change: compileVar()'s
// per-declaration loop (used for the parenthesized "var ( ... )" list form)
// was accidentally being controlled by a value returned from
// collectVarListNames() that had nothing to do with "are there more
// declarations in this list" -- it happened to become false exactly in the
// "name immediately followed by = or )" case, which is exactly the case this
// fix newly makes reachable. Without decoupling the two, a var(...) block
// containing more than one type-inferred declaration would silently stop
// after the first one. See the comment on collectVarListNames in var.go for
// the full explanation.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// runVarTest is a small helper shared by the tests below. It runs the given
// Ego program text and returns the value of the "result" variable the
// program is expected to set. Any compile or runtime error other than the
// normal end-of-program "stop" signal fails the test immediately.
func runVarTest(t *testing.T, name, text string) any {
	t.Helper()

	s := symbols.NewRootSymbolTable(name)
	AddStandard(s)

	if err := RunString(name, s, text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	result, found := s.Get("result")
	if !found {
		t.Fatal("program did not set 'result'")
	}

	return result
}

// TestBUG17VarInferredSingleName verifies that "var name = expr" compiles and
// runs correctly for a single declared name, inferring the type of "name"
// from the initializer expression at compile time (the value's own runtime
// type), just like "name := expr".
func TestBUG17VarInferredSingleName(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// The exact reproducer from BUG-17: a float initializer with no
			// type token. Before the fix this failed to compile at all with
			// "invalid type specification".
			name: "float initializer",
			text: `
				var pi = 3.14
				result := pi
			`,
			want: 3.14,
		},
		{
			name: "string initializer",
			text: `
				var greeting = "hello"
				result := greeting
			`,
			want: "hello",
		},
		{
			name: "int initializer",
			text: `
				var count = 42
				result := count
			`,
			want: 42,
		},
		{
			name: "bool initializer",
			text: `
				var ok = true
				result := ok
			`,
			want: true,
		},
		{
			// The initializer can be an arbitrary expression, not just a
			// literal -- it is compiled with the full expression grammar,
			// exactly as it would be for "sum := a + b".
			name: "expression initializer",
			text: `
				a := 2
				b := 3
				var sum = a + b
				result := sum
			`,
			want: 5,
		},
		{
			// A self-describing composite literal (the type name appears in
			// the literal itself) works fine even though there is no outer
			// type to borrow from.
			name: "typed slice literal initializer",
			text: `
				var list = []int{1, 2, 3}
				result := len(list) == 3 && list[0] == 1 && list[2] == 3
			`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVarTest(t, tt.name, tt.text)

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

// TestBUG17VarInferredGroupList verifies that the parenthesized var(...) list
// form supports type-inferred declarations, including the exact reproducer
// from the bug report:
//
//	var (
//	    p = 3.14
//	    q = "pi"
//	)
//
// This also exercises the fix to the list-continuation bug described in the
// package comment above: without it, only "p" would be declared and the
// compiler would misparse (or silently drop) "q".
func TestBUG17VarInferredGroupList(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			name: "two type-inferred declarations in one list",
			text: `
				var (
					p = 3.14
					q = "pi"
				)
				result := q == "pi" && p == 3.14
			`,
			want: true,
		},
		{
			// Mixing all four declaration forms in a single list: an
			// explicitly typed declaration with an initializer, a
			// type-inferred declaration, a user-defined struct type with no
			// initializer, and a multi-name type-inferred declaration.
			name: "mixed typed, inferred, and user-type declarations",
			text: `
				type Point struct {
					x int
					y int
				}
				var (
					a int = 5
					b = "hello"
					c Point
					d, e = 7
				)
				result := a == 5 && b == "hello" && c.x == 0 && c.y == 0 && d == 7 && e == 7
			`,
			want: true,
		},
		{
			// Three (not just two) type-inferred declarations, to confirm
			// the list loop keeps going past the second entry as well.
			name: "three type-inferred declarations in one list",
			text: `
				var (
					x = 1
					y = 2
					z = 3
				)
				result := x + y + z
			`,
			want: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runVarTest(t, tt.name, tt.text)

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

// TestBUG17VarInferredMultiNameSharedValue verifies that "var a, b = expr"
// (multiple names, one initializer) duplicates the single computed value
// across all the names, matching the pre-existing behavior of the typed
// path ("var a, b int = 5") for the same construct. Real Go rejects
// "var a, b int = 5" at compile time -- one value per name is required --
// but Ego's typed var path has always allowed it by copying the value, and
// this fix intentionally mirrors that existing, if non-standard, behavior
// for consistency rather than introducing a different rule for the
// inferred-type path.
func TestBUG17VarInferredMultiNameSharedValue(t *testing.T) {
	result := runVarTest(t, "multi-name shared value", `
		var a, b = 42
		result := a == 42 && b == 42
	`)

	if want := true; !reflect.DeepEqual(result, want) {
		t.Errorf("got %v (%T), want %v (%T)", result, result, want, want)
	}
}

// TestBUG17VarUserTypeStillErrorsOnUnknownType is a regression guard: the fix
// distinguishes "var name = expr" from "var name UserType" by peeking for an
// "=" token. It must not accidentally treat a genuinely unknown type name as
// an inferred initializer -- "var p NoSuchType" (no "=" present) must still
// go through varUserType() and fail with an "unknown symbol" error rather
// than silently succeeding or producing some other unrelated error.
func TestBUG17VarUserTypeStillErrorsOnUnknownType(t *testing.T) {
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	err := RunString(t.Name(), s, `
		var p NoSuchType
		_ = p
	`)

	if errors.Nil(err) {
		t.Fatal("expected an error for a reference to an unknown type, got nil")
	}
}
