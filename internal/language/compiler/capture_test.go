package compiler

// capture_test.go tests the "@capture" directive (captureDirective, in
// capture.go) end to end: real Ego source text is compiled and run via
// RunString, and the resulting symbol table is inspected to see what value
// ended up in the captured variable. This mirrors the style already used
// by directives_test.go for the "@compile" directive.
//
// Tests that exercise the individual BeginCapture/EndCapture/
// SyncOutputWriter bytecode instructions directly (without going through
// the compiler) live in internal/language/bytecode/capture_test.go instead.

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// TestCaptureDirective_Define exercises the basic "@capture x := { ... }"
// form: x should end up holding exactly what the block printed, character
// for character.
func TestCaptureDirective_Define(t *testing.T) {
	program := `
		import "fmt"

		@capture output := {
			fmt.Println("Hello")
			fmt.Printf("%d\n", 53)
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("output"); !ok || v != "Hello\n53\n" {
		t.Errorf("output = %#v (found=%v), want \"Hello\\n53\\n\"", v, ok)
	}
}

// TestCaptureDirective_Assign exercises "@capture x = { ... }": x must
// already exist (declared here with "var"), and @capture assigns into it
// rather than declaring a new one.
func TestCaptureDirective_Assign(t *testing.T) {
	program := `
		import "fmt"

		var output string

		@capture output = {
			fmt.Println("reused")
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("output"); !ok || v != "reused\n" {
		t.Errorf("output = %#v (found=%v), want \"reused\\n\"", v, ok)
	}
}

// TestCaptureDirective_Nested verifies that an "@capture" block inside
// another "@capture" block only captures its own output, and that the
// outer block correctly resumes capturing its own text once the inner one
// ends (rather than losing track of where its own buffer left off).
//
// "inner" is declared with ":=" INSIDE the outer block, so (like any other
// ordinary variable declared inside a block) it only exists for the
// lifetime of that block and is not visible from the root symbol table
// once the program finishes -- that is ordinary Ego scoping, nothing
// specific to "@capture". So this test checks the inner capture's value
// indirectly: it prints whether "inner" held the expected text, and that
// message becomes part of the OUTER capture's own text, which IS still
// visible afterward.
func TestCaptureDirective_Nested(t *testing.T) {
	program := `
		import "fmt"

		@capture outer := {
			fmt.Println("outer-before")

			@capture inner := {
				fmt.Println("inner-text")
			}

			fmt.Println("inner was:", inner == "inner-text\n")
			fmt.Println("outer-after")
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	want := "outer-before\ninner was:true\nouter-after\n"
	if v, ok := s.Get("outer"); !ok || v != want {
		t.Errorf("outer = %#v (found=%v), want %#v", v, ok, want)
	}
}

// TestCaptureDirective_ErrorIsRethrownWithPartialOutput is the key
// error-safety test: when an error occurs partway through the block, (1)
// the variable still receives whatever was printed BEFORE the error, and
// (2) the error keeps propagating outward and is observable by a real,
// enclosing try/catch (rather than being silently swallowed by @capture).
func TestCaptureDirective_ErrorIsRethrownWithPartialOutput(t *testing.T) {
	program := `
		import "fmt"

		var output string
		caught := false

		try {
			@capture output = {
				fmt.Println("before the error")
				x := 5 / 0
				fmt.Println(x)
			}
		} catch(e) {
			caught = true
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("caught"); !ok || v != true {
		t.Errorf("caught = %v (found=%v), want true -- the enclosing try/catch must observe the rethrown error", v, ok)
	}

	if v, ok := s.Get("output"); !ok || v != "before the error\n" {
		t.Errorf("output = %#v (found=%v), want \"before the error\\n\" (the partial output up to the error)", v, ok)
	}
}

// TestCaptureDirective_MissingAssignOperator verifies that "@capture x"
// with neither ":=" nor "=" following is a compile-time error, not a
// runtime surprise.
func TestCaptureDirective_MissingAssignOperator(t *testing.T) {
	program := `
		@capture output {
			fmt.Println("nope")
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); errors.Nil(err) {
		t.Fatal("expected a compile error for a missing ':=' or '=', got nil")
	}
}

// TestCaptureDirective_AssignToUndeclaredVariable verifies that
// "@capture x = { ... }" is rejected at compile time when x was never
// declared -- a stricter (and more helpful) check than ordinary "x = value"
// assignment, which only fails at runtime for an undeclared variable.
func TestCaptureDirective_AssignToUndeclaredVariable(t *testing.T) {
	program := `
		@capture neverDeclared = {
			fmt.Println("nope")
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); errors.Nil(err) {
		t.Fatal("expected a compile error for '=' assignment to an undeclared variable, got nil")
	}
}

// TestCaptureDirective_DiscardTwiceSameScope is a regression test for a
// real bug found during development: using "@capture _ := { ... }" twice
// in the same scope used to fail at RUNTIME with "stack underflow: EndCapture".
//
// The root cause: captureDirective originally always emitted a SymbolCreate
// instruction for the ":=" form, even when the variable name was "_". Every
// other place in the language that assigns to "_" skips SymbolCreate/Store
// entirely (see lvalue.go) precisely because re-declaring "_" this way
// fails at runtime the second time it happens in the same scope. Since that
// SymbolCreate ran INSIDE the try-protected region but BEFORE BeginCapture,
// its failure landed in @capture's own catch handler before there was
// anything for EndCapture to match, corrupting the capture bookkeeping
// instead of cleanly reporting an error. The fix skips SymbolCreate/Store
// entirely for "_", matching how the rest of the language already treats it.
func TestCaptureDirective_DiscardTwiceSameScope(t *testing.T) {
	program := `
		import "fmt"

		@capture _ := {
			fmt.Println("first")
		}

		@capture _ := {
			fmt.Println("second")
		}

		done := true
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("done"); !ok || v != true {
		t.Errorf("done = %v (found=%v), want true -- program must run to completion", v, ok)
	}
}

// TestCaptureDirective_DiscardWithAssignOperator verifies that "_" works
// with "=" too, not just ":=" -- matching how ordinary Ego code can assign
// to "_" with either operator interchangeably.
func TestCaptureDirective_DiscardWithAssignOperator(t *testing.T) {
	program := `
		import "fmt"

		@capture _ = {
			fmt.Println("first")
		}

		@capture _ = {
			fmt.Println("second")
		}

		done := true
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("done"); !ok || v != true {
		t.Errorf("done = %v (found=%v), want true -- program must run to completion", v, ok)
	}
}

// TestCaptureDirective_DiscardErrorStillRethrows verifies that discarding
// the captured text on success doesn't affect error handling: an error
// inside a "@capture _ := { ... }" block must still be cleanly cleaned up
// and rethrown, observable by a real enclosing try/catch, exactly as it
// would be for a named capture variable.
func TestCaptureDirective_DiscardErrorStillRethrows(t *testing.T) {
	program := `
		import "fmt"

		caught := false

		try {
			@capture _ := {
				fmt.Println("before the error")
				x := 5 / 0
				fmt.Println(x)
			}
		} catch(e) {
			caught = true
		}
	`

	s := symbols.NewRootSymbolTable(t.Name())
	if err := RunString(t.Name(), s, program); !errors.Nil(err) {
		t.Fatalf("unexpected error running program: %v", err)
	}

	if v, ok := s.Get("caught"); !ok || v != true {
		t.Errorf("caught = %v (found=%v), want true -- the enclosing try/catch must observe the rethrown error", v, ok)
	}
}
