package bytecode

// This file implements the three bytecode instructions that back the Ego
// language's "@capture" directive. If you are new to Go, the short version
// is: these are three small functions, each one runs when the compiled
// program reaches a certain instruction, and together they let a block of
// Ego code have its printed output collected into a string instead of
// appearing on the screen.
//
// # Why does @capture exist?
//
// Ego test files often want to check that some code PRINTS the right thing
// (via fmt.Println, fmt.Printf, etc.), not just that it RETURNS the right
// value. Before "@capture" existed, there was no clean way to check printed
// output from inside an Ego test -- you could only compare things like the
// byte-count that fmt.Println() reports back, which proves the output was
// the right LENGTH but not the right CONTENT.
//
// The Ego source syntax looks like this:
//
//	@capture output := {
//	    fmt.Println("Hello")
//	    fmt.Printf("%d\n", 53)
//	}
//
// After this block runs, the variable "output" holds the exact text
// "Hello\n53\n" -- everything the block would otherwise have printed to the
// console. From there, ordinary Ego code (typically an "@assert" in a test)
// can check that string just like any other value.
//
// # How "capturing" actually works
//
// Every running Ego program has a single place all of its printed output
// funnels through: the Context struct (see context.go) has a field named
// "output" whose type is io.Writer -- a standard Go interface that just
// means "something you can write text to". By default this field holds
// os.Stdout, i.e. the real console. Both the "print" statement (print.go)
// and the Println/Printf/Print functions from Ego's "fmt" package
// (internal/runtime/fmt/print.go) end up writing to whatever this field
// currently points to.
//
// "@capture" works by temporarily swapping that field out for a
// strings.Builder -- a small in-memory buffer that satisfies the same
// io.Writer interface, but instead of showing text on screen, it just
// remembers it. Once the "@capture" block finishes, the builder's
// accumulated text is read out (as a plain Go string) and the "output"
// field is put back exactly the way it was.
//
// # Why a *stack*, and not just a single saved value?
//
// "@capture" blocks can nest (one @capture inside another), and -- more
// importantly -- they can appear INSIDE an "@test" block, which already has
// its own, separate, similar-sounding mechanism (see EnableConsoleOutput
// and consoleByteCode/sayByteCode in context.go/logging.go): every "@test"
// wraps its whole body in exactly this kind of swap, so it can gather up
// everything the test printed and show it as one tidy "TEST: ... (PASS)"
// line instead of letting it scroll by loosely.
//
// If "@capture" just remembered ONE previous writer, using it inside an
// "@test" would go like this: @capture saves "whatever @test was using" (a
// strings.Builder of its own), installs its OWN new builder, and everything
// inside the @capture block goes there correctly -- so far so good. But
// this only works because there happens to be exactly one "@capture" per
// "@test". If you nested a *second* @capture inside the first one, the
// second one would need to remember the FIRST @capture's builder as "what
// to go back to" -- and a single saved-value field can't remember two
// different "go back to this" destinations at once.
//
// A stack (Context.outputStack, a slice of io.Writer) solves this the same
// way a stack of bookmarks solves "which page was I on before this one":
// every time an "@capture" block starts, whatever writer is currently
// active gets pushed onto the stack, and a new builder takes over. Every
// time an "@capture" block ends, the stack is popped, and Context.output
// goes back to being exactly whatever it was one level up -- whether that's
// the real console, an enclosing "@capture"'s builder, or an enclosing
// "@test"'s builder. Nothing needs to know or care which of those three it
// actually is.
//
// # Why THREE opcodes and not two?
//
// You might expect just two instructions: one to start capturing
// (BeginCapture) and one to stop and collect the result (EndCapture). Ego
// actually needs a third, tiny instruction named SyncOutputWriter, and the
// reason is a subtlety in how "@capture" reports an error.
//
// Every "@capture" block is internally wrapped in the same try/catch
// machinery used by Ego's own "try { } catch(e) { }" statement (see
// compileTry in the compiler package, and try.go/catch.go in this package),
// so that if something inside the block goes wrong, @capture can still shut
// itself off cleanly and print whatever had been captured so far -- rather
// than silently discarding it -- before letting the error continue on its
// way. That catch-handling code runs in an inner "scope" (think of a scope
// as one level of a filing cabinet of variables) that is about to be thrown
// away. The captured TEXT and the io.Writer itself live on the Context
// struct, which doesn't care about scopes, so restoring Context.output is
// always safe no matter which scope is active. But part of "restoring the
// previous writer" also means telling Ego's "fmt" package (fmt.Println,
// etc.) which writer to use -- and that part IS recorded in the symbol
// table, in the CURRENT scope, using a method called SetAlways that never
// looks outside that one scope. If that bookkeeping step ran while the
// soon-to-be-discarded inner scope was still active, the correction would
// be thrown away along with the scope, and any fmt.Println calls *after*
// the @capture block (for the rest of that test, say) would keep writing
// into the abandoned buffer instead of going back to the console or the
// enclosing @test's buffer -- silently losing output.
//
// So the compiler splits the work into two steps: EndCapture handles only
// the parts that are always safe (restore Context.output, hand back the
// captured text), and SyncOutputWriter handles only the scope-sensitive
// bookkeeping. The compiler (see capture.go in the compiler package) is
// careful to emit the SyncOutputWriter instruction only once the correct,
// surviving scope is active again.

import (
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// beginCaptureByteCode implements the BeginCapture opcode. It has no
// operand (the "i any" parameter is unused) and never fails.
//
// What it does, in order:
//  1. Remember the writer currently in use (Context.output) by pushing it
//     onto Context.outputStack, so it can be restored later.
//  2. Create a brand-new, empty strings.Builder and make it the current
//     writer. From this point on, anything printed goes into this buffer
//     instead of wherever it was going before.
//  3. Tell the symbol table about the new writer too, because Ego's "fmt"
//     package (fmt.Println, fmt.Printf, ...) looks the writer up there
//     rather than asking the Context directly. Keeping both copies in sync
//     is what makes BOTH the "print" statement AND fmt.Println route to the
//     same captured buffer.
func beginCaptureByteCode(c *Context, i any) error {
	// Step 1: save a bookmark for where output was going before.
	c.outputStack = append(c.outputStack, c.output)

	// Step 2: install a fresh, empty buffer as the new destination.
	c.output = &strings.Builder{}

	// Step 3: let the "fmt" package's functions find the same buffer.
	c.symbols.SetAlways(defs.StdoutWriterSymbol, c.output)

	return nil
}

// endCaptureByteCode implements the EndCapture opcode. It has no operand.
//
// What it does, in order:
//  1. Read back everything that was written to the buffer installed by the
//     matching BeginCapture, as a plain Go string.
//  2. Push that string onto the value stack, so the compiler-generated code
//     that follows (a Store instruction) can save it into the user's
//     "@capture" variable.
//  3. Restore Context.output to whatever it was before the matching
//     BeginCapture ran, by popping Context.outputStack.
//
// Deliberately NOT done here: updating the symbol table's record of the
// current writer (see SyncOutputWriter, and the long comment at the top of
// this file for why that has to happen separately, and sometimes later,
// than this step).
func endCaptureByteCode(c *Context, i any) error {
	// There must be a matching BeginCapture for every EndCapture. If there
	// isn't, that's a bug in the compiler (not something an Ego program
	// author can trigger by accident), so report it as a stack-underflow
	// error, the same category of error used elsewhere in this package for
	// "something was popped that was never pushed" situations.
	if len(c.outputStack) == 0 {
		return c.runtimeError(errors.ErrStackUnderflow).Context("EndCapture")
	}

	// Step 1: read out whatever was captured. c.output is always a
	// *strings.Builder here because only BeginCapture ever installs one;
	// the two-value type assertion is just a defensive guard so that if
	// this invariant is ever accidentally violated by future code, the
	// worst that happens is an empty captured string -- not a crash.
	builder, _ := c.output.(*strings.Builder)

	captured := ""
	if builder != nil {
		captured = builder.String()
	}

	// Step 2: hand the captured text to whatever runs next (a Store
	// instruction that saves it into the user's variable).
	if err := c.push(captured); err != nil {
		return err
	}

	// Step 3: pop the bookmark and go back to writing wherever we were
	// writing before the matching BeginCapture.
	last := len(c.outputStack) - 1
	c.output = c.outputStack[last]
	c.outputStack = c.outputStack[:last]

	return nil
}

// syncOutputWriterByteCode implements the SyncOutputWriter opcode. It has
// no operand and never fails.
//
// All it does is copy the Context's current writer (Context.output) into
// the symbol table, under the same key EndCapture leaves stale on the error
// path. See the long comment at the top of this file for the full
// explanation of why this needs to be its own instruction rather than
// folded into EndCapture.
func syncOutputWriterByteCode(c *Context, i any) error {
	c.symbols.SetAlways(defs.StdoutWriterSymbol, c.output)

	return nil
}
