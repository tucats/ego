package compiler

// Tests for the BUG-26 fix: struct assignment (":=" and "="), function
// argument passing, and struct-field assignment must all give the
// destination its own independent copy of the struct, matching Go's value
// semantics for structs. Before the fix, none of these operations copied
// anything - the destination ended up pointing at the exact same
// *data.Struct as the source, so mutating one silently mutated the other.
//
// Arrays and maps are deliberately NOT copied by this fix (they continue to
// alias on assignment, matching Go's own slice/map reference semantics), so
// several tests below also pin down that array/map aliasing was not
// accidentally changed.

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

func TestBUG26StructValueSemantics(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			// The exact reproducer from docs/ISSUES.md BUG-26.
			name: "short declaration: p2 := p1 does not alias",
			text: `
				type Point struct { X int; Y int }
				p1 := Point{X: 1, Y: 2}
				p2 := p1
				p2.X = 99
				result := p1.X == 1 && p2.X == 99
			`,
			want: true,
		},
		{
			name: "plain assignment: p2 = p1 does not alias",
			text: `
				type Point struct { X int; Y int }
				p1 := Point{X: 1, Y: 2}
				var p2 Point
				p2 = p1
				p2.X = 99
				result := p1.X == 1 && p2.X == 99
			`,
			want: true,
		},
		{
			name: "function argument: struct parameter is a copy",
			text: `
				type Point struct { X int; Y int }
				func mutate(p Point) {
					p.X = 555
				}
				p1 := Point{X: 1, Y: 2}
				mutate(p1)
				result := p1.X == 1
			`,
			want: true,
		},
		{
			name: "anonymous struct assignment does not alias",
			text: `
				a1 := struct{ N int }{N: 5}
				a2 := a1
				a2.N = 42
				result := a1.N == 5 && a2.N == 42
			`,
			want: true,
		},
		{
			name: "struct field assignment of a struct value does not alias",
			text: `
				type Point struct { X int; Y int }
				type Box struct { Origin Point }
				src := Point{X: 10, Y: 20}
				var box Box
				box.Origin = src
				box.Origin.X = 555
				result := src.X == 10 && box.Origin.X == 555
			`,
			want: true,
		},
		{
			name: "nested struct-in-struct field is also copied, not aliased",
			text: `
				type Point struct { X int; Y int }
				type Box struct { Origin Point; Label string }
				b1 := Box{Origin: Point{X: 1, Y: 2}, Label: "a"}
				b2 := b1
				b2.Origin.X = 999
				result := b1.Origin.X == 1 && b2.Origin.X == 999
			`,
			want: true,
		},
		{
			// Regression guard: this fix must not change array aliasing, which is
			// intentional and matches Go's slice reference semantics.
			name: "arrays still alias on assignment (unaffected by this fix)",
			text: `
				arr1 := []int{1, 2, 3}
				arr2 := arr1
				arr2[0] = 99
				result := arr1[0] == 99 && arr2[0] == 99
			`,
			want: true,
		},
		{
			// Regression guard: this fix must not change map aliasing.
			name: "maps still alias on assignment (unaffected by this fix)",
			text: `
				m1 := map[string]int{"a": 1}
				m2 := m1
				m2["a"] = 77
				result := m1["a"] == 77 && m2["a"] == 77
			`,
			want: true,
		},
		{
			// Regression guard: a struct field that is itself an array/map must
			// keep sharing its backing storage after a struct-level copy - this
			// matches Go, where copying a struct copies a slice/map field's
			// lightweight header but not the data it refers to.
			name: "array/map fields inside a copied struct still share backing storage",
			text: `
				type Bag struct { Items []int; Tags map[string]int }
				b1 := Bag{Items: []int{1, 2, 3}, Tags: map[string]int{"a": 1}}
				b2 := b1
				b2.Items[0] = 999
				b2.Tags["a"] = 888
				result := b1.Items[0] == 999 && b1.Tags["a"] == 888
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

// TestBUG26ReceiverSemanticsUnaffected verifies that method receiver binding
// - which uses a completely separate code path (bytecode's GetThis opcode,
// not Store/CreateAndStore/Arg) - still has the correct, pre-existing
// behavior after the BUG-26 fix: pointer receivers mutate the original,
// value receivers do not.
func TestBUG26ReceiverSemanticsUnaffected(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		{
			name: "pointer receiver method still mutates the original",
			text: `
				type Counter struct { N int }
				func (c *Counter) Inc() { c.N++ }
				c := Counter{N: 1}
				c.Inc()
				result := c.N == 2
			`,
			want: true,
		},
		{
			name: "value receiver method does not mutate the original",
			text: `
				type Counter struct { N int }
				func (c Counter) Bump() { c.N++ }
				c := Counter{N: 1}
				c.Bump()
				result := c.N == 1
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

// TestBUG26PointerTypedParameterNotCopied is a regression test for a
// follow-up bug found (via REST server trace inspection, not a locally
// runnable repro) shortly after the BUG-26 fix shipped: the REST server
// hands its ResponseWriter to a service handler by loading it directly out
// of a reserved symbol ("_response_writer") and calling the handler - the
// value that arrives is a raw *data.Struct, not the *any that Ego's own
// "&x" address-of operator produces, even though the handler parameter is
// declared with a pointer type (e.g. "w *http.ResponseWriter").
//
// The BUG-26 fix's blanket "an argument that is a *data.Struct gets copied"
// rule broke this: the handler's local "w" became an independent copy, so a
// field mutation inside the handler (such as WriteHeader setting a status
// code) never reached the real ResponseWriter the REST server reads back
// after the handler returns - so, for example, a WriteHeader(400) call
// inside the handler was silently lost, and the client saw a 200 status
// with the correct body text.
//
// This test reproduces the same shape without needing the HTTP server: a
// *data.Struct is injected directly into the symbol table (as the REST
// server does for "_response_writer"), then passed to a function whose
// parameter is declared with a pointer type. A field write inside the
// function must be visible through the original struct afterward - i.e.
// argByteCode must skip the BUG-26 copy when the *declared* parameter type
// is a pointer, regardless of the argument's actual runtime Go type.
func TestBUG26PointerTypedParameterNotCopied(t *testing.T) {
	s := symbols.NewRootSymbolTable(t.Name())
	AddStandard(s)

	// A generic (anonymous-shaped) struct, deliberately built the same way
	// the REST server builds its ResponseWriter: a plain *data.Struct with
	// no *any pointer indirection wrapped around it.
	widget := data.NewStruct(data.StructureType())
	_ = widget.Set("n", 1)
	s.SetAlways("w", widget)

	program := `
		type Widget struct { n int }
		func mutate(w *Widget) {
			w.n = 99
		}
		mutate(w)
	`

	if err := RunString(t.Name(), s, program); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
		t.Fatalf("unexpected compile/run error: %v", err)
	}

	n, found := widget.Get("n")
	if !found {
		t.Fatal("field 'n' missing from the original struct after the call")
	}

	if n != 99 {
		t.Errorf("original struct was not mutated through the pointer-typed parameter: n = %v, want 99 (BUG-26 pointer-parameter regression)", n)
	}
}
