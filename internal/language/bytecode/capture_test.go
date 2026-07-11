package bytecode

// capture_test.go tests the three bytecode instructions that back the
// "@capture" directive: beginCaptureByteCode, endCaptureByteCode, and
// syncOutputWriterByteCode. See capture.go for a full explanation of what
// each instruction does and why there are three of them instead of two.
//
// These tests exercise the instructions directly (the same way the rest of
// this package's *_test.go files do), without going through the compiler.
// Compiler-level tests that actually parse and run "@capture ... { ... }"
// source text live in internal/language/compiler/capture_test.go instead.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// Test_beginCaptureByteCode_InstallsBuilder verifies that BeginCapture
// replaces the context's output writer with an empty *strings.Builder, and
// remembers the previous writer (os.Stdout, for a freshly created test
// context) on the outputStack so it can be restored later.
func Test_beginCaptureByteCode_InstallsBuilder(t *testing.T) {
	tc := newTestContext(t)

	// A fresh context always starts out writing to the real console.
	if tc.ctx.output != os.Stdout {
		t.Fatalf("precondition failed: expected fresh context to write to os.Stdout, got %T", tc.ctx.output)
	}

	err := beginCaptureByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	if _, ok := tc.ctx.output.(*strings.Builder); !ok {
		t.Fatalf("beginCaptureByteCode: output = %T, want *strings.Builder", tc.ctx.output)
	}

	if len(tc.ctx.outputStack) != 1 {
		t.Fatalf("beginCaptureByteCode: outputStack has %d entries, want 1", len(tc.ctx.outputStack))
	}

	if tc.ctx.outputStack[0] != os.Stdout {
		t.Errorf("beginCaptureByteCode: saved writer = %v, want os.Stdout", tc.ctx.outputStack[0])
	}
}

// Test_beginCaptureByteCode_SyncsSymbolTable verifies that BeginCapture also
// updates the defs.StdoutWriterSymbol entry, which is what the native "fmt"
// package (fmt.Println, fmt.Printf, ...) actually checks to decide where to
// write -- see internal/runtime/fmt/print.go. Without this, capturing would
// only affect the "print" language-extension statement, not fmt.Println.
func Test_beginCaptureByteCode_SyncsSymbolTable(t *testing.T) {
	tc := newTestContext(t)

	err := beginCaptureByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	writer, found := tc.ctx.symbols.Get(defs.StdoutWriterSymbol)
	if !found {
		t.Fatal("beginCaptureByteCode: defs.StdoutWriterSymbol not set in symbol table")
	}

	if writer != tc.ctx.output {
		t.Errorf("beginCaptureByteCode: symbol table writer = %v, want %v (same as ctx.output)", writer, tc.ctx.output)
	}
}

// Test_endCaptureByteCode_ReturnsCapturedTextAndRestoresWriter is the core
// round-trip test: begin a capture, write some text directly to the
// installed writer (simulating what Print/fmt.Println would do), then end
// the capture and confirm the exact text comes back and the original
// writer (os.Stdout) is restored.
func Test_endCaptureByteCode_ReturnsCapturedTextAndRestoresWriter(t *testing.T) {
	tc := newTestContext(t)

	if err := beginCaptureByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("beginCaptureByteCode failed: %v", err)
	}

	// Simulate program output happening while capture is active.
	fmt.Fprint(tc.ctx.output, "Hello\n")
	fmt.Fprint(tc.ctx.output, "53\n")

	err := endCaptureByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	tc.assertTopStack("Hello\n53\n")

	if tc.ctx.output != os.Stdout {
		t.Errorf("endCaptureByteCode: output = %v, want os.Stdout restored", tc.ctx.output)
	}

	if len(tc.ctx.outputStack) != 0 {
		t.Errorf("endCaptureByteCode: outputStack has %d entries left, want 0", len(tc.ctx.outputStack))
	}
}

// Test_endCaptureByteCode_EmptyCapture verifies that a capture block that
// never wrote anything correctly produces an empty string, not an error.
func Test_endCaptureByteCode_EmptyCapture(t *testing.T) {
	tc := newTestContext(t)

	if err := beginCaptureByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("beginCaptureByteCode failed: %v", err)
	}

	err := endCaptureByteCode(tc.ctx, nil)
	tc.assertNoError(err)
	tc.assertTopStack("")
}

// Test_endCaptureByteCode_Nested verifies that two nested capture blocks
// each get exactly their own text back, and that ending the inner one
// restores the OUTER capture's builder (not the real console) as the
// current writer -- this is the specific nesting behavior the outputStack
// field exists to support (see the long comment in capture.go).
func Test_endCaptureByteCode_Nested(t *testing.T) {
	tc := newTestContext(t)

	// Outer capture begins.
	if err := beginCaptureByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("outer beginCaptureByteCode failed: %v", err)
	}

	fmt.Fprint(tc.ctx.output, "outer-before\n")

	outerWriter := tc.ctx.output

	// Inner capture begins; must save "outerWriter" as its bookmark.
	if err := beginCaptureByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("inner beginCaptureByteCode failed: %v", err)
	}

	if len(tc.ctx.outputStack) != 2 {
		t.Fatalf("after nested begin: outputStack has %d entries, want 2", len(tc.ctx.outputStack))
	}

	fmt.Fprint(tc.ctx.output, "inner-text\n")

	// Inner capture ends: must return only "inner-text\n", and restore
	// Context.output to the OUTER capture's builder, not os.Stdout.
	if err := endCaptureByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("inner endCaptureByteCode failed: %v", err)
	}

	tc.assertTopStack("inner-text\n")

	if tc.ctx.output != outerWriter {
		t.Errorf("after inner end: output = %v, want outer capture's builder restored", tc.ctx.output)
	}

	// Finish writing to the (now current again) outer builder, then end it.
	fmt.Fprint(tc.ctx.output, "outer-after\n")

	if err := endCaptureByteCode(tc.ctx, nil); err != nil {
		t.Fatalf("outer endCaptureByteCode failed: %v", err)
	}

	tc.assertTopStack("outer-before\nouter-after\n")

	if tc.ctx.output != os.Stdout {
		t.Errorf("after outer end: output = %v, want os.Stdout restored", tc.ctx.output)
	}
}

// Test_endCaptureByteCode_WithoutMatchingBegin verifies that calling
// EndCapture with no matching BeginCapture (an empty outputStack) reports a
// clean internal error instead of panicking. This should never happen from
// ordinary Ego source code -- it would mean the compiler emitted a broken
// instruction sequence -- so this is a defensive/internal-consistency test.
func Test_endCaptureByteCode_WithoutMatchingBegin(t *testing.T) {
	tc := newTestContext(t)

	err := endCaptureByteCode(tc.ctx, nil)
	tc.assertError(err, errors.ErrStackUnderflow)
}

// Test_syncOutputWriterByteCode_CopiesCurrentWriter verifies that
// SyncOutputWriter always copies whatever Context.output currently is into
// the symbol table, regardless of what it was set to before.
func Test_syncOutputWriterByteCode_CopiesCurrentWriter(t *testing.T) {
	tc := newTestContext(t)

	// Install some arbitrary writer directly (bypassing BeginCapture) to
	// prove SyncOutputWriter only ever looks at Context.output, nothing else.
	builder := &strings.Builder{}
	tc.ctx.output = builder

	err := syncOutputWriterByteCode(tc.ctx, nil)
	tc.assertNoError(err)

	writer, found := tc.ctx.symbols.Get(defs.StdoutWriterSymbol)
	if !found {
		t.Fatal("syncOutputWriterByteCode: defs.StdoutWriterSymbol not set")
	}

	if writer != builder {
		t.Errorf("syncOutputWriterByteCode: symbol table writer = %v, want %v", writer, builder)
	}
}
