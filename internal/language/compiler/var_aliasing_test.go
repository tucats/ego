package compiler

// Tests for the BUG-23 fix: var declarations of struct/array types must produce
// a fresh instance on every function call.
//
// Before the fix, varInitializer emitted "Push model" where model was a single
// *Struct or *Array pointer computed once at compile time. Every invocation of
// the function pushed and mutated the same shared pointer, so state accumulated
// across calls.
//
// After the fix, varInitializer emits "$new(kind)" for *Struct and *Array
// zero-values, creating a fresh allocation at each function invocation.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// TestBUG23VarStructNoAliasing verifies that a function containing
// "var c StructType" allocates a distinct struct on each call rather than
// reusing the compile-time model pointer.
func TestBUG23VarStructNoAliasing(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// The classic BUG-23 reproducer: increment() declares "var c Counter",
			// adds 1 to c.n, and returns c. Without the fix, the third call would
			// start with c.n==2 instead of c.n==0 because all three calls share the
			// same *Struct. The bool result captures whether all three returned n==1.
			name: "named struct: three calls each return n==1 (not 1,2,3)",
			text: `
				type Counter struct { n int }
				func increment() Counter {
					var c Counter
					c.n = c.n + 1
					return c
				}
				r1 := increment()
				r2 := increment()
				r3 := increment()
				result := r1.n == 1 && r2.n == 1 && r3.n == 1
			`,
			want: true,
		},
		{
			// Returned pointer independence: if the underlying struct is shared, then
			// a mutation via one returned pointer would corrupt the others. After the
			// fix each returned *Struct is a distinct heap object.
			name: "pointer return: mutating b.x does not affect a.x",
			text: `
				type Point struct { x int }
				func makePoint() *Point { var p Point; p.x = 7; return &p }
				a := makePoint()
				b := makePoint()
				b.x = 99
				result := a.x == 7
			`,
			want: true,
		},
		{
			// When two names are declared in a single "var a, b MyStruct" the
			// compiler emits two separate $new(kind) calls. a and b must be
			// independent objects.
			name: "multi-name var: a and b are independent structs",
			text: `
				type Box struct { v int }
				var a, b Box
				a.v = 1
				b.v = 2
				result := a.v == 1 && b.v == 2
			`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AddStandard populates the runtime symbol table with built-in
			// functions, including "$new" which the BUG-23 fix emits for
			// struct/array var declarations.  Without this call, runtime
			// lookup of "$new" fails with "unknown identifier".
			s := symbols.NewRootSymbolTable(tt.name)
			AddStandard(s)

			if err := RunString(tt.name, s, tt.text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Fatalf("unexpected compile/run error: %v", err)
			}

			result, found := s.Get("result")
			if !found {
				t.Fatal("program did not set 'result'")
			}

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

// TestBUG23VarSliceNoAliasing verifies that a function containing
// "var a []T" allocates a distinct slice on each call, so that appends
// in one call do not carry over to the next.
func TestBUG23VarSliceNoAliasing(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// Without the fix, every call to collect() appended to the same
			// *Array. The third call would return a slice of length 3. After the
			// fix, each call starts with a fresh empty slice.
			name: "slice var: three calls each append one item (lengths all 1)",
			text: `
				func collect(item int) []int {
					var list []int
					list = append(list, item)
					return list
				}
				l1 := collect(10)
				l2 := collect(20)
				l3 := collect(30)
				result := len(l1) == 1 && len(l2) == 1 && len(l3) == 1
			`,
			want: true,
		},
		{
			// Verify that the correct elements are present, not just the lengths.
			name: "slice var: element values are correct across calls",
			text: `
				func collect(item int) []int {
					var list []int
					list = append(list, item)
					return list
				}
				l1 := collect(10)
				l2 := collect(20)
				result := l1[0] == 10 && l2[0] == 20
			`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			AddStandard(s)

			if err := RunString(tt.name, s, tt.text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Fatalf("unexpected compile/run error: %v", err)
			}

			result, found := s.Get("result")
			if !found {
				t.Fatal("program did not set 'result'")
			}

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

// TestBUG23NilMapVarSafe verifies that nil-state maps (var m map[K]V) still
// correctly report as nil and produce an error on write even though their
// compile-time model pointer is shared (aliasing is safe for nil-state maps
// because Set() refuses writes — BUG-12 fix).
func TestBUG23NilMapVarSafe(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// Both calls should return a nil map. The shared compile-time pointer
			// is harmless because nil-state maps cannot be mutated.
			name: "nil map var: two calls both return nil maps",
			text: `
				func nilMap() map[string]int { var m map[string]int; return m }
				m1 := nilMap()
				m2 := nilMap()
				result := m1 == nil && m2 == nil
			`,
			want: true,
		},
		{
			// Assigning an initialized literal to m1 must not affect m2.
			// After m1 = map[string]int{}, m2 must still be nil.
			name: "nil map var: assigning to one does not affect the other",
			text: `
				var m1 map[string]int
				var m2 map[string]int
				m1 = map[string]int{}
				result := m1 != nil && m2 == nil
			`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			AddStandard(s)

			if err := RunString(tt.name, s, tt.text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Fatalf("unexpected compile/run error: %v", err)
			}

			result, found := s.Get("result")
			if !found {
				t.Fatal("program did not set 'result'")
			}

			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}
