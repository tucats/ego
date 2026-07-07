package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// TestBUG07TwoValueChannelReceive exercises the two-value channel receive
// form "v, ok := <-ch" end-to-end.  Because the TestArbitraryCodeFragments
// harness uses a bare symbol table without built-in functions (so "make" is
// unavailable), these tests inject a pre-built *data.Channel directly into
// the symbol table before running the Ego program snippet.
//
// Three scenarios are covered:
//   1. Open channel with a value already buffered: ok=true, v=datum.
//   2. Open channel — verify the received value itself (not just ok).
//   3. Closed, empty channel: ok=false, v=nil (the zero-value convention).
//
// Note on settings: "ch" is injected straight into the runtime symbol table
// rather than being declared with "make(chan, 1)" inside the Ego snippet.
// Run()/RunString() compile the program against a fresh, empty compiler
// symbol table — the "s" table passed in here is only wired up for
// execution afterward — so the compiler's static unknown-symbol check can
// never see "ch" as declared. With the default (in-memory) settings this is
// harmless, because compiler.defs.UnknownVarSetting defaults to false and
// unknown-symbol diagnostics are suppressed. But some other test in this
// package (TestCompiler_ReadDirectory) loads the real on-disk settings
// profile into the global settings singleton, and a developer's real
// profile may have unknown.var.error=true persisted, which would make this
// test fail depending on what ran before it in the same test binary. Pin
// the setting to "false" for the duration of this test so its outcome only
// depends on the channel-receive bytecode being exercised, not on ambient,
// test-order-dependent global state.
func TestBUG07TwoValueChannelReceive(t *testing.T) {
	previousSetting := settings.Get(defs.UnknownVarSetting)
	settings.SetDefault(defs.UnknownVarSetting, defs.False)

	defer settings.SetDefault(defs.UnknownVarSetting, previousSetting)

	tests := []struct {
		name    string
		setup   func(*symbols.SymbolTable) // called once to prepare the channel
		program string                     // Ego code fragment; must set "result"
		want    any
	}{
		{
			// Happy path: the channel has one item buffered.  After
			// "v, ok := <-ch", ok should be true.
			name: "ok is true when channel has a value",
			setup: func(s *symbols.SymbolTable) {
				ch := data.NewChannel(1)
				_ = ch.Send(99)
				s.SetAlways("ch", ch)
			},
			program: `v, ok := <-ch; result := ok`,
			want:    true,
		},
		{
			// Verify the received datum is correct, not just the ok flag.
			name: "received value is correct",
			setup: func(s *symbols.SymbolTable) {
				ch := data.NewChannel(1)
				_ = ch.Send(42)
				s.SetAlways("ch", ch)
			},
			program: `v, ok := <-ch; result := v`,
			want:    42,
		},
		{
			// Closed-channel path: ok should be false and datum nil.
			name: "ok is false on closed channel",
			setup: func(s *symbols.SymbolTable) {
				ch := data.NewChannel(1)
				ch.Close() // close before any receive
				s.SetAlways("ch", ch)
			},
			program: `v, ok := <-ch; result := ok`,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			tt.setup(s)

			if err := RunString(tt.name, s, tt.program); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Errorf("unexpected error: %v", err)
			}

			result, found := s.Get("result")
			if !found || !reflect.DeepEqual(result, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", result, result, tt.want, tt.want)
			}
		})
	}
}

func TestArbitraryCodeFragments(t *testing.T) {
	tests := []struct {
		name string
		text string
		want any
	}{
		// The text of each test contains the entire program snipped to
		// run, which _must_ create a variable named "result" which is compared to
		// the wanted value. The program can be of arbitrary complexity, and need
		// not be a complete program or function.
		{
			name: "decimal integer assignment",
			text: "result := 1",
			want: 1,
		},

		// --- BUG-06 regression tests: ++ and -- on qualified lvalues ----------
		//
		// Each test below creates an array or struct, applies ++ or -- to one
		// of its elements/fields, and stores the modified value in "result" for
		// comparison.  Before the BUG-06 fix these programs produced a compile
		// error "invalid use of auto increment/decrement operation".
		{
			// Array element increment: a[0] starts at 1, after a[0]++ it is 2.
			name: "array element increment (BUG-06)",
			text: `a := []int{1, 2, 3}; a[0]++; result := a[0]`,
			want: 2,
		},
		{
			// Array element decrement: a[1] starts at 2, after a[1]-- it is 1.
			name: "array element decrement (BUG-06)",
			text: `a := []int{1, 2, 3}; a[1]--; result := a[1]`,
			want: 1,
		},
		{
			// Struct field increment using a dynamic (anonymous) struct.
			// s.x starts at 10, after s.x++ it is 11.
			name: "struct field increment (BUG-06)",
			text: `s := {x: 10}; s.x++; result := s.x`,
			want: 11,
		},
		{
			// Struct field decrement: s.y starts at 5, after s.y-- it is 4.
			name: "struct field decrement (BUG-06)",
			text: `s := {y: 5}; s.y--; result := s.y`,
			want: 4,
		},
		{
			// Multiple increments: each ++ must be independent and cumulative.
			// a[0] starts at 0, after three a[0]++ calls it is 3.
			name: "multiple array element increments (BUG-06)",
			text: `a := []int{0, 10, 20}; a[0]++; a[0]++; a[0]++; result := a[0]`, //nolint:dupword
			want: 3,
		},
		{
			// Other array elements are unaffected by ++ on a[0].
			// a[2] should remain 20 after a[0]++.
			name: "array increment does not affect other elements (BUG-06)",
			text: `a := []int{1, 2, 3}; a[0]++; result := a[2]`,
			want: 3,
		},
		{
			// Array element ++ at a computed (variable) index.
			// a[i] where i=1 starts at 5; after a[i]++ it is 6.
			name: "array element increment with variable index (BUG-06)",
			text: `a := []int{0, 5, 10}; i := 1; a[i]++; result := a[i]`,
			want: 6,
		},
		// --- end BUG-06 regression tests -------------------------------------

		// --- BUG-63 regression tests: standalone x++/x-- leaked a "let" -------
		// stack marker, corrupting later function calls in the same scope.
		//
		// Background for the novice reader: the compiler uses temporary
		// "marker" values pushed onto the runtime value stack as bookmarks,
		// so that cleanup code later can pop everything back down to a known
		// point. Before the BUG-63 fix, compiling a bare "x++" or "x--"
		// statement (used on its own, not as a for-loop's increment clause)
		// pushed one of these bookmarks but never popped it back off. The
		// leftover bookmark stayed on the stack forever, so any function
		// call compiled afterward in the same scope would see one extra,
		// unexpected item mixed in with its own arguments/return values --
		// most visibly reported as "function did not return the expected
		// number of values". See docs/ISSUES.md, BUG-63, for the full
		// root-cause writeup.
		{
			// This is the exact reproducer from the bug report: a function
			// whose body contains a bare "n++" followed by "return n". Before
			// the fix, the leaked marker made the function's own "return"
			// appear to hand back too many values, and the call below failed
			// with "did not return the expected number of values" instead of
			// simply returning 1.
			name: "standalone increment inside function body (BUG-63)",
			text: `
				func run() int {
					n := 0
					n++

					return n
				}

				result := run()
			`,
			want: 1,
		},
		{
			// This mirrors the reproducer above but uses "--" and starts the
			// counter above zero, so a sign error in the arithmetic (Add vs
			// Sub) would also be caught, not just the stack-marker leak.
			name: "standalone decrement inside function body (BUG-63)",
			text: `
				func run() int {
					n := 5
					n--

					return n
				}

				result := run()
			`,
			want: 4,
		},
		{
			// The clearest demonstration of "corrupts a LATER function call":
			// the leaked marker from a bare "x++" statement is left sitting
			// on the stack, and then an unrelated function call happens
			// immediately afterward, in the same scope. Before the fix, the
			// stray marker was mistaken for one of double()'s own return
			// values, and the call itself would fail even though double()
			// has nothing to do with the earlier increment.
			name: "increment followed by unrelated function call (BUG-63)",
			text: `
				func double(v int) int {
					return v * 2
				}

				x := 1
				x++

				result := double(x)
			`,
			want: 4,
		},
		{
			// Multiple bare increments in a row must each clean up their own
			// marker; if even one leaked, this would compound the stack
			// corruption and the final call would fail.
			name: "multiple standalone increments before a function call (BUG-63)",
			text: `
				func triple(v int) int {
					return v * 3
				}

				x := 0
				x++
				x++
				x++

				result := triple(x)
			`,
			want: 9,
		},
		// --- end BUG-63 regression tests ---------------------------------------

		// --- end BUG-07 regression tests -------------------------------------
		{
			name: "octal integer assignment",
			text: "result := 0o10",
			want: 8,
		},
		{
			name: "hexadecimal integer assignment",
			text: "result := 0x10",
			want: 16,
		},
		{
			name: "optional error catch",
			text: "result := ?(5/0):-1",
			want: -1,
		},
		{
			name: "Conditional expression",
			text: `result := true?"yes":"no"`,
			want: "yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewRootSymbolTable(tt.name)
			if err := RunString(tt.name, s, tt.text); !errors.Nil(err) && err.Error() != errors.ErrStop.Error() {
				t.Errorf("Unexpected error %v", err)
			}

			result, found := s.Get("result")
			if !reflect.DeepEqual(result, tt.want) || !found {
				t.Errorf("Unexpected result; got %v, want %v", result, tt.want)
			}
		})
	}
}
