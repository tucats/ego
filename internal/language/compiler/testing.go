package compiler

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

var testType *data.Type

// initTestType lazily creates the singleton "Testing" struct type that backs the
// implicit "T" variable available in Ego test files. The type has a single
// "description" string field and a set of built-in assertion methods (assert,
// Fail, Nil, NotNil, True, False, Equal, NotEqual). After the first call the
// type is cached in testType and subsequent calls are no-ops.
func initTestType() {
	if testType == nil {
		t := data.TypeDefinition("Testing",
			data.StructureType(data.Field{
				Name: "description",
				Type: data.StringType,
			}))

		// Define the type receiver functions
		t.DefineFunction("assert", nil, TestAssert)
		t.DefineFunction("Fail", nil, TestFail)
		t.DefineFunction("Nil", nil, TestNil)
		t.DefineFunction("NotNil", nil, TestNotNil)
		t.DefineFunction("True", nil, TestTrue)
		t.DefineFunction("False", nil, TestFalse)
		t.DefineFunction("Equal", nil, TestEqual)
		t.DefineFunction("NotEqual", nil, TestNotEqual)

		testType = t
	}
}

// testDirective compiles the "@test description" directive. It may only appear
// when the compiler is running in test mode (the "ego test" command). Each
// @test directive:
//
//  1. Creates a new Testing struct instance with the given description string
//     and stores it in the "T" variable so assertion methods are available.
//  2. Emits code to print "TEST: <description>" to the console and start a
//     timer for the test.
//  3. Compiles everything that makes up this test's own body -- up to the
//     next top-level @test directive, or end of file -- in an isolated
//     clone of this compiler (see compileTestBody), guarded by a try/catch
//     so that a compile error or an uncaught runtime error (e.g. a failed
//     @assert) inside just this one test cannot prevent the rest of the
//     file's tests from compiling and running. Either outcome closes the
//     test itself, immediately, with a PASS or FAIL report -- there is no
//     more lazy "the next @test closes the previous one" behavior, and no
//     more need for an explicit trailing @pass (removed; see docs/TESTING.md).
func (c *Compiler) testDirective() error {
	var (
		err             error
		testDescription string
	)

	// If we're not in test mode, this is an invalid use of the @test
	// directive.
	if !c.flags.testMode {
		return c.compileError(errors.ErrWrongMode)
	}

	testDescription = c.t.NextText()
	if testDescription[0] == '"' {
		testDescription, err = strconv.Unquote(testDescription)

		if err != nil {
			return c.compileError(err)
		}
	}

	// Sanity check; the can't be longer than 48 characters or the
	// formatting of the output gets weird. So truncate the string
	// to 48 chars max, including "..." if truncation happens.
	//
	// This -- and the pad calculation below, via PadString -- must count
	// runes, not bytes: a description containing any multi-byte UTF-8
	// character (an em-dash, say) has more bytes than visible characters,
	// so len() (byte count) both truncates it too short and pads it too
	// little relative to plain-ASCII descriptions, producing a ragged
	// column of "(PASS)"/"(FAIL)" markers instead of an aligned one. See
	// CLAUDE.md's "@test name constraints" for why descriptions are
	// supposed to be ASCII-only in the first place -- this just keeps the
	// alignment correct regardless.
	descRunes := []rune(testDescription)
	if len(descRunes) > 48 {
		testDescription = string(descRunes[:46]) + "..."
	}

	// Create an instance of the object, and assign the value to
	// the data field.
	initTestType()

	test := data.NewStruct(testType)
	test.SetAlways("description", testDescription)

	pad := PadString(testDescription, 50)

	c.b.Emit(bytecode.Push, test)

	c.b.Emit(bytecode.StoreAlways, "T")

	// Generate code to report that the test is starting.
	if err := c.ReferenceSymbol("T"); err != nil {
		return err
	}

	// Turn on output capture for the whole test (see docs/TESTING.md's
	// "Output capture" section) and start the elapsed-time clock, but do
	// NOT print the "TEST: <description>" header yet -- unlike the old
	// behavior, nothing is printed until the test's outcome (PASS or FAIL)
	// is known. Printing the header here, before the body ran, meant
	// anything the test itself printed landed in the middle of a
	// still-open "TEST: <description>" line instead of after it. Now the
	// header and the result are assembled and printed together, as a
	// single operation, by emitTestPass/emitTestFail once the test is
	// actually over -- so whatever the test printed (if anything) always
	// appears as its own, complete block, followed by one clean
	// "TEST: <description> ... (PASS|FAIL) ..." line, never interrupted
	// partway through.
	c.b.Emit(bytecode.Console, false)
	c.b.Emit(bytecode.Timer, 0)
	c.b.Emit(bytecode.PushTest)

	c.DefineGlobalSymbol("T")

	if err := c.ReferenceSymbol("T"); err != nil {
		return err
	}

	return c.compileTestBody(testDescription, pad)
}

// collectTestBodyTokens collects every token that makes up the CURRENT
// test's body: everything from here up to (but not including) the next
// top-level "@test" directive, or the end of the token stream. This is the
// same implicit boundary the old lazy "close the preceding test" mechanism
// always assumed, made explicit so the body can be handed to an isolated
// sub-compiler.
//
// Nested "{"/"}" pairs (an if, for, func, or an explicit test-body block)
// are tracked via a brace-depth counter so a "@test" appearing inside one of
// those is never mistaken for the boundary -- only a "@test" seen at depth 0
// stops the scan. This deliberately does not require the test body to be
// wrapped in its own "{ }": a test may be a single braced block (the
// documented, common form) or a sequence of bare top-level statements (an
// older but still-supported style used by a few existing tests -- see
// docs/TESTING.md) or, in principle, a mix of both; either way, "everything
// up to the next @test" is the correct body.
func (c *Compiler) collectTestBodyTokens() *tokenizer.Tokenizer {
	tokens := tokenizer.New("", true)
	depth := 0

	for !c.t.AtEnd() {
		if depth == 0 && c.t.Peek(1).Is(tokenizer.DirectiveToken) && c.t.Peek(2).Is(tokenizer.TestToken) {
			break
		}

		// An "@compile ... eof=<marker>" directive intentionally contains
		// unbalanced braces -- that is the entire point of eof-marker mode
		// (see collectTokensUntilEOFMarker's doc comment in directives.go):
		// it lets a test exercise code with a missing or extra "{"/"}"
		// without that imbalance confusing the @compile directive's own
		// extent-finding. But this depth counter does not know that; a
		// stray unmatched "{" inside such a span throws off "depth" so it
		// never returns to 0 at the real end of the current test, and the
		// scan runs on past it -- silently swallowing one or more following
		// @test directives whole into this test's body. The sub-compiler
		// then compiles those swallowed @test directives as nested tests,
		// which corrupts the buffered/quiet-mode output machinery (each
		// nested test's own emitTestPass/emitTestFail flushes and resets
		// the output capture buffer, so this test's own PASS/FAIL line ends
		// up printed unbuffered, breaking straight through -q). Detect this
		// directive shape and skip its entire eof-marker-delimited span
		// atomically -- every token in it is still copied into the result,
		// just without adjusting depth for any of it.
		if marker := c.peekCompileEOFMarker(); marker != "" {
			c.collectVerbatimUntilMarker(tokens, marker)

			continue
		}

		t := c.t.Next()

		if t.Is(tokenizer.BlockBeginToken) {
			depth++
		} else if t.Is(tokenizer.BlockEndToken) {
			depth--
		}

		tokens.Append(t)
	}

	return tokens
}

// peekCompileEOFMarker looks ahead from the current tokenizer position,
// without consuming anything, to determine whether it is positioned at an
// "@compile ... eof=<marker>" directive statement. If so, it returns the
// marker's text (already unquoted, matching how compileBlockDirective itself
// reads the flag -- see its "eofFlag" case in directives.go); otherwise it
// returns "".
//
// This only needs to recognize the "eof" flag is present somewhere in the
// directive's flag list; it does not validate the rest of the directive's
// grammar; compileBlockDirective does that for real when the sub-compiler
// actually compiles this test's body.
func (c *Compiler) peekCompileEOFMarker() string {
	if !c.t.Peek(1).Is(tokenizer.DirectiveToken) || c.t.Peek(2).Spelling() != CompileDirective {
		return ""
	}

	for offset := 3; ; offset++ {
		tok := c.t.Peek(offset)

		if tok.IsClass(tokenizer.EndOfTokensClass) || tok.Is(tokenizer.BlockBeginToken) || tok.Is(tokenizer.SemicolonToken) {
			return ""
		}

		if tok.Spelling() == "eof" && c.t.Peek(offset+1).Spelling() == "=" {
			marker := c.t.Peek(offset + 2)
			if marker.IsString() {
				return marker.Spelling()
			}

			return ""
		}
	}
}

// collectVerbatimUntilMarker consumes tokens from c.t, starting at the "@"
// of an eof-marker @compile directive already confirmed present by
// peekCompileEOFMarker, and appends every one of them to dest -- including
// the directive's own flag tokens, the raw code body, and the marker tokens
// themselves -- without ever adjusting a brace-depth counter. The directive
// preamble (up to and including its terminating ";") has no braces by
// grammar, so it is simply copied token-for-token. The code body that
// follows is then matched against marker using the exact same
// candidate/pending algorithm as collectTokensUntilEOFMarker in
// directives.go, so the boundary found here is guaranteed to agree with the
// one the sub-compiler will find later when it actually compiles this
// directive -- but here every token, including the marker's own, is kept
// (collectTokensUntilEOFMarker instead discards the marker, since its
// caller wants only the code to compile; this function's caller wants the
// complete, unmodified source text of the test body to hand to the
// sub-compiler).
func (c *Compiler) collectVerbatimUntilMarker(dest *tokenizer.Tokenizer, marker string) {
	// Copy the directive preamble verbatim through its terminating ";".
	for !c.t.AtEnd() {
		t := c.t.Next()
		dest.Append(t)

		if t.Is(tokenizer.SemicolonToken) {
			break
		}
	}

	// Now match the code body against marker, exactly as
	// collectTokensUntilEOFMarker does, except every token read -- pending
	// or not, and the final matching run too -- is appended to dest.
	var pending []tokenizer.Token

	candidate := ""

	for !c.t.AtEnd() {
		next := c.t.Next()
		attempt := candidate + next.Spelling()

		if attempt == marker {
			dest.Append(pending...)
			dest.Append(next)

			return
		}

		if strings.HasPrefix(marker, attempt) {
			pending = append(pending, next)
			candidate = attempt

			continue
		}

		dest.Append(pending...)
		pending = nil
		candidate = ""

		if next.Spelling() != "" && strings.HasPrefix(marker, next.Spelling()) {
			pending = append(pending, next)
			candidate = next.Spelling()
		} else {
			dest.Append(next)
		}
	}

	// Marker never found (malformed test source) -- flush anything still
	// pending so no tokens are silently dropped; the sub-compiler will
	// report a proper "missing EOF marker" error when it re-parses this
	// same directive for real.
	dest.Append(pending...)
}

// compileTestBody consumes and compiles the remainder of the current test
// (see collectTestBodyTokens for exactly what that spans), in an isolated
// clone of this compiler, and wraps it in a try/catch so that neither a
// compile-time error nor an uncaught runtime error can escape and abort the
// rest of the file's tests.
//
// Cloning (rather than compiling directly into c, or building a wholly
// independent Compiler) is what makes this safe: Clone copies the state a
// nested block needs to see and validate against correctly -- functionDepth,
// blockDepth, flags, scopes (a shallow copy that shares the underlying
// per-name usage maps, so a reference to an outer-scope symbol, such as the
// "T" variable just defined above, or a variable shared across multiple
// @test blocks in this file, is validated AND marks that symbol used in the
// very same map the parent compiler holds), types, and packages. On success,
// Compile's own call to Close() merges the clone's final state back into c
// (Close's "if c.parent != nil" branch); on failure, Compile returns before
// ever calling Close(), so a partially-declared scope, a half-registered
// type, or any other partial state from the failed attempt is simply
// discarded along with the abandoned clone, leaving c completely untouched.
// This is the same technique compileBlockDirective (@compile) already uses,
// and is required here for the same reason: compileBlock's own scope/type
// bookkeeping is only unwound on its NORMAL return path, not via a defer, so
// compiling directly into the live c and letting an error propagate up
// would leave c's blockDepth, scopes, and types permanently desynchronized.
func (c *Compiler) compileTestBody(testDescription, pad string) error {
	tokens := c.collectTestBodyTokens()

	subCompiler := c.Clone("@test")

	bc, compileErr := subCompiler.Compile("@test", tokens)
	// Deliberately NOT also calling subCompiler.Errors() here: it walks
	// every entry in c.scopes, including the scopes this clone inherited
	// (shares, by reference, via the shallow copy in Clone) from the parent
	// -- most importantly the file's top-level scope, shared by every test
	// in the file. Errors() chains any still-unused entries it finds
	// together with *errors.Error's mutable, in-place .Chain() (it sets
	// e.next directly and returns the same receiver, not a copy). Calling
	// it here, once per test, would repeatedly re-chain the SAME shared
	// error objects across up to dozens of independent clones -- each call
	// mutating .next pointers a previous call already set -- which can and
	// does corrupt the chain into a cycle (observed as a stack overflow in
	// Error.Chain while compiling tests/json/parse.ego, which declares
	// several top-level values shared across ~30 @test blocks). The file's
	// top-level scope is correctly checked exactly once, for the whole
	// file, by the outer (non-clone) compiler's own Close() call at the
	// very end of compilation -- unaffected by anything in this function --
	// so nothing is lost by not duplicating that check per test. Unused
	// variables declared and used entirely within this test's own body are
	// still caught normally: compileBlock's PopSymbolScope call already
	// reports those as part of subCompiler.Compile()'s own returned error,
	// with no help from Errors() needed.

	tryAddr := c.b.Mark()

	// The marker's label must be exactly "try" -- handleCatch's stack unwind
	// (bytecode/catch.go) specifically looks for a StackMarker labeled "try"
	// to know where to stop popping when it redirects control to a catch
	// handler; any other label is invisible to it and the unwind overshoots
	// into real stack data below the marker.
	tryMarker := bytecode.NewStackMarker("try")

	c.b.Emit(bytecode.Try, 0)
	c.b.Emit(bytecode.Push, tryMarker)

	if compileErr != nil {
		c.b.Emit(bytecode.Push, compileErr)
		c.b.Emit(bytecode.Signal, nil)
	} else {
		// Bracket the test code with a capture operation
		c.b.Emit(bytecode.BeginCapture)

		c.b.Append(bc)

		c.b.Emit(bytecode.EndCapture)

		// EndCapture only restores Context.output -- it deliberately does
		// not touch the "fmt" package's own notion of where to write (see
		// the long comment at the top of bytecode/capture.go). Without
		// this, defs.StdoutWriterSymbol in the symbol table would still
		// point at the buffer BeginCapture installed above, and any
		// fmt.Println reachable later in this scope would silently write
		// into a buffer nobody ever reads again.
		c.b.Emit(bytecode.SyncOutputWriter)

		// The EndCapture will leave a string on the stack containing any output
		// generated during the test run. The following code is essentially:
		//   if len(text) == 0 {
		//		drop text
		//	 } else {
		//		print "TEST: .... (OUTPUT)"
		//		print text
		//	 }

		c.b.Emit(bytecode.Dup)
		c.b.Emit(bytecode.Load, "len")
		c.b.Emit(bytecode.Swap)
		c.b.Emit(bytecode.Call, 1)
		c.b.Emit(bytecode.Push, 0)
		c.b.Emit(bytecode.Equal)

		elsePatch := c.b.Mark()
		c.b.Emit(bytecode.BranchTrue, 0)

		// Both Print instructions below pop exactly one item, so they must
		// NOT be combined into a single "Print, 2" -- printByteCode inserts
		// a separator space before every item after the first when a
		// single Print instruction pops more than one, which would put a
		// stray leading space in front of the captured text.
		//
		// Also deliberately NOT a Say here: Say is the one-time, end-of-test
		// operation (see emitTestPass/emitTestFail below) that drains the
		// per-test capture buffer installed by "Console false" and hands it
		// to ui.Say, gated by quiet mode. Calling it here would drain and
		// reset that buffer early, so the test's own end-of-test Say call
		// would find it already empty and print a spurious blank line
		// instead of quietly no-op'ing. Print instead, into the still-open
		// buffer, and let that one end-of-test Say flush everything --
		// this OUTPUT block and the PASS/FAIL line -- together.
		text := "TEST: " + testDescription + pad + "(OUTPUT)\n"
		c.b.Emit(bytecode.Push, text)
		c.b.Emit(bytecode.Print)
		c.b.Emit(bytecode.Print)

		exitPatch := c.b.Mark()
		c.b.Emit(bytecode.Branch, 0)
		c.b.SetAddressHere(elsePatch)

		c.b.Emit(bytecode.Drop) // it was empty output, toss it
		c.b.SetAddressHere(exitPatch)
	}

	c.b.Emit(bytecode.DropToMarker, tryMarker)

	// Success path: close this test out with a normal PASS report, then
	// skip over the catch/FAIL handler below entirely.
	if err := c.emitTestPass(testDescription, pad); err != nil {
		return err
	}

	branchAddr := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	// Catch handler: the Try instruction's own operand branches here on any
	// caught error, whether it is the Signal above (a compile error) or a
	// genuine uncaught runtime error from the compiled body (e.g. a failed
	// @assert).
	_ = c.b.SetAddressHere(tryAddr)

	c.emitTestFail(testDescription, pad)

	_ = c.b.SetAddressHere(branchAddr)

	c.b.Emit(bytecode.TryPop)

	return nil
}

// Helper function to get the test name.
func getTestName(s *symbols.SymbolTable) string {
	// Figure out the test name. If not found, use "test"
	name := "test"

	if m, ok := s.Get("T"); ok {
		if testStruct, ok := m.(*data.Struct); ok {
			if nameString, ok := testStruct.Get("description"); ok {
				name = data.String(nameString)
			}
		}
	}

	return name
}

// TestAssert implements the T.assert() function.
//
// This used to call a bare fmt.Println() before returning a failure, writing
// a newline directly to the real process stdout to cleanly terminate
// whatever partial "TEST: ..." line was on the terminal before the
// (previously always fatal) top-level "Error: ..." message printed. That
// bypassed Ego's own output-capture buffering (see docs/TESTING.md's
// "Output capture" section) entirely, so the newline could land at any point
// relative to a test's buffered output once assert failures became
// catchable instead of always fatal -- see compileTestBody/emitTestFail in
// this file -- producing a stray blank line in the wrong place instead of
// the clean, single "TEST: ... (FAIL) ..." line callers now get. Removed;
// nothing is lost, since emitTestFail's own Console/Say handling already
// produces a properly terminated line without it.
func TestAssert(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In("assert")
	}

	// The argument could be an array with the boolean value and the
	// messaging string, or it might just be the boolean.
	if array, ok := args.Get(0).([]any); ok && len(array) == 2 {
		b, err := data.Bool(array[0])
		if err != nil {
			return nil, err
		}

		if !b {
			msg := data.String(array[1])

			return nil, errors.ErrAssert.In(getTestName(s)).Context(msg)
		}

		return true, nil
	}

	// Just the boolean; the string is optionally in the second
	// argument.
	b, err := data.Bool(args.Get(0))
	if err != nil {
		return nil, err
	}

	if !b {
		msg := errors.ErrTestingAssert

		if args.Len() > 1 {
			msg = msg.Context(args.Get(1))
		} else {
			msg = msg.Context(getTestName(s))
		}

		return nil, msg
	}

	return true, nil
}

// TestFail implements the T.fail() function which generates a fatal
// error.
func TestFail(s *symbols.SymbolTable, args data.List) (any, error) {
	msg := "T.fail()"

	if args.Len() == 1 {
		msg = data.String(args.Get(0))
	}

	return nil, errors.Message(msg).In(getTestName(s))
}

// TestNil implements the T.Nil() function.
func TestNil(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	isNil := args.Get(0) == nil
	if e, ok := args.Get(0).(error); ok {
		isNil = errors.Nil(e)
	}

	if args.Len() == 2 {
		return []any{isNil, data.String(args.Get(1))}, nil
	}

	return isNil, nil
}

// TestNotNil implements the T.NotNil() function.
func TestNotNil(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	isNil := args.Get(0) == nil
	if e, ok := args.Get(0).(error); ok {
		isNil = errors.Nil(e)
	}

	if args.Len() == 2 {
		return []any{!isNil, data.String(args.Get(1))}, nil
	}

	return !isNil, nil
}

// TestTrue implements the T.True() function.
func TestTrue(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	if args.Len() == 2 {
		b, err := data.Bool(args.Get(0))
		if err != nil {
			return nil, err
		}

		return []any{b, data.String(args.Get(1))}, nil
	}

	v, err := data.Bool(args.Get(0))

	return v, err
}

// TestFalse implements the T.False() function.
func TestFalse(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 1 || args.Len() > 2 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	if args.Len() == 2 {
		b, err := data.Bool(args.Get(0))
		if err != nil {
			return nil, err
		}

		return []any{!b, data.String(args.Get(1))}, nil
	}

	b, err := data.Bool(args.Get(0))

	return !b, err
}

// TestEqual implements the T.Equal() function.
func TestEqual(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 2 || args.Len() > 3 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	b := reflect.DeepEqual(args.Get(0), args.Get(1))

	if a1, ok := args.Get(0).([]any); ok {
		if a2, ok := args.Get(1).(*data.Array); ok {
			b = reflect.DeepEqual(a1, a2.BaseArray())
		}
	} else if a1, ok := args.Get(1).([]any); ok {
		if a2, ok := args.Get(0).(*data.Array); ok {
			b = reflect.DeepEqual(a1, a2.BaseArray())
		}
	} else if a1, ok := args.Get(0).(*data.Array); ok {
		if a2, ok := args.Get(1).(*data.Array); ok {
			b = a1.DeepEqual(a2)
		}
	}

	if args.Len() == 3 {
		return []any{b, data.String(args.Get(2))}, nil
	}

	return b, nil
}

// TestNotEqual implements the T.NotEqual() function.
func TestNotEqual(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() < 2 || args.Len() > 3 {
		return nil, errors.ErrArgumentCount.In(getTestName(s))
	}

	b, err := TestEqual(s, args)
	if err == nil {
		b, err := data.Bool(b)

		return !b, err
	}

	return nil, err
}

// Assert implements the @assert directive.
func (c *Compiler) Assert() error {
	_ = c.modeCheck("test")

	if c.t.IsNext(tokenizer.SemicolonToken) {
		return c.compileError(errors.ErrMissingExpression)
	}

	if err := c.ReferenceSymbol("T"); err != nil {
		return err
	}

	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("assert"))
	c.b.Emit(bytecode.Load, "T")
	c.b.Emit(bytecode.Member, "assert")

	argCount := 1

	if err := c.emitExpression(); err != nil {
		return err
	}

	c.b.Emit(bytecode.Call, argCount)
	c.b.Emit(bytecode.DropToMarker)

	return nil
}

// Fail implements the @fail directive. Note that unlike other test-related
// directives, @fail can be used even when not in "test" mode.
func (c *Compiler) Fail() error {
	if !c.t.EndOfStatement() {
		if err := c.emitExpression(); err != nil {
			return err
		}
	} else {
		c.b.Emit(bytecode.Push, "@fail error signal")
	}

	c.b.Emit(bytecode.TryFlush)
	c.b.Emit(bytecode.Signal, nil)

	return nil
}

// emitTestPass emits the "this test passed" close-out sequence: it prints
// the complete "TEST: <description> ... (PASS)  <elapsed>" line -- as a
// single operation, only now that the test is known to have passed -- and
// increments the passed-test counter.
//
// Printing the header here (rather than eagerly, before the body ran) means
// anything the test itself printed lands, as its own complete block, BEFORE
// this line, never interrupted partway through by it -- see the comment in
// testDirective where the header used to be printed.
//
// This used to be reachable directly from Ego source as the @pass directive
// and was also invoked implicitly at the start of the NEXT @test to lazily
// close the previous one. Both of those triggers are gone: @pass has been
// removed entirely (it's never required -- see docs/TESTING.md), and
// compileTestBody (testing.go) now calls this itself, immediately, right
// after each test's own body completes without error -- see that function's
// doc comment for the full replacement design.
//
// PushTest/PopTest's active-test counting (this function calls PopTest)
// still makes it safe to call this more than once for the same test, should
// that ever happen for some other reason in the future: a second call finds
// the active-test count already at zero and skips all of its own work (see
// PopTest's "count<1" branch in bytecode/test.go).
func (c *Compiler) emitTestPass(testDescription, pad string) error {
	_ = c.modeCheck("test")
	c.b.Emit(bytecode.RunDefers)

	here := c.b.Mark()
	c.b.Emit(bytecode.PopTest, 0)

	// Increment the test counter variable
	c.b.Emit(bytecode.Load, "_testcount")
	c.b.Emit(bytecode.Push, 1)
	c.b.Emit(bytecode.Add)
	c.b.Emit(bytecode.StoreGlobal, "_testcount")

	// Print the header and status result as one unbroken sequence.
	c.b.Emit(bytecode.Push, "TEST: ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, testDescription)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, pad)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, "(PASS)  ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 1)
	c.b.Emit(bytecode.Dup)
	c.b.Emit(bytecode.Push, "<none>")
	c.b.Emit(bytecode.Equal)

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchTrue, 0)

	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Say, true)
	done := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	c.b.SetAddressHere(branch)
	c.b.Emit(bytecode.Drop) // timer value string

	c.b.SetAddressHere(done)
	c.b.SetAddressHere(here)

	return nil
}

// emitTestFail emits the "this test failed" close-out sequence, used as the
// catch-block body compileTestBody installs around each test's own body
// (testing.go). It runs when that body either failed to compile (the
// generated code signals the compile error directly -- see compileTestBody)
// or raised an uncaught, catchable runtime error while executing, such as a
// failed @assert.
//
// Like emitTestPass, the "TEST: <description> ... (FAIL)  <elapsed>" line is
// assembled and printed as one operation, only now that the test is known to
// have failed -- so whatever the test itself printed before failing (if
// anything) still comes out as its own complete block first, never with this
// status line stuck in the middle of it. The line is deliberately structured
// identically to emitTestPass's "(PASS)" line -- same prefix width, same
// elapsed-timer handling, same Print/Say sequence -- just with "(FAIL)" in
// place of "(PASS)  " and a different counter variable, so a failed test's
// summary line is visually indistinguishable in shape from a passing one and
// can be grepped for the same way. This is what actually fixes the original
// problem this whole mechanism exists for: before this, a compile error deep
// inside one @test block aborted the entire file's compilation, so nothing
// after it -- not even a "TEST: ..." header for the failing test itself --
// ever printed, leaving no way to tell which test broke, or that any of the
// OTHER tests in the file even existed.
//
// The underlying error (caught in defs.ErrorVariable by the runtime's
// catch-dispatch machinery -- see bytecode/catch.go) is printed as a
// follow-up line immediately after the FAIL status line has flushed, once
// console output is back to normal (unbuffered) -- see docs/TESTING.md's
// "Output capture" section for why buffering/flushing works the way it does.
func (c *Compiler) emitTestFail(testDescription, pad string) {
	c.b.Emit(bytecode.RunDefers)

	here := c.b.Mark()
	c.b.Emit(bytecode.PopTest, 0)

	// Increment the failed-test counter variable.
	c.b.Emit(bytecode.Load, "_testfailcount")
	c.b.Emit(bytecode.Push, 1)
	c.b.Emit(bytecode.Add)
	c.b.Emit(bytecode.StoreGlobal, "_testfailcount")

	// Print the header and status result as one unbroken sequence, in the
	// same shape as emitTestPass.
	c.b.Emit(bytecode.Push, "TEST: ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, testDescription)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, pad)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Push, "(FAIL)  ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Timer, 1)
	c.b.Emit(bytecode.Dup)
	c.b.Emit(bytecode.Push, "<none>")
	c.b.Emit(bytecode.Equal)

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchTrue, 0)

	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Say, true)
	done := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	c.b.SetAddressHere(branch)
	c.b.Emit(bytecode.Drop) // timer value string

	c.b.SetAddressHere(done)

	// Print the underlying error as its own, clearly-labeled follow-up
	// line. By this point Say has already flushed the FAIL status line and
	// reset output back to the real console, so this prints immediately
	// rather than joining the buffered line above.
	c.b.Emit(bytecode.Push, "       Error: ")
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Load, defs.ErrorVariable)
	c.b.Emit(bytecode.Print)
	c.b.Emit(bytecode.Newline)

	c.b.SetAddressHere(here)
}

// Fail implements the @file directive.
func (c *Compiler) File() error {
	fileName := c.t.Next().Spelling()
	c.sourceFile = fileName

	c.b.Emit(bytecode.InFile, fileName)

	return nil
}

// PadString returns the whitespace needed to right-pad s so that
// "s + PadString(s, width)" is exactly width runes wide -- not bytes, so a
// string containing multi-byte UTF-8 characters (an em-dash, say) is
// measured by its visible character count, not its encoded size. If s is
// already width runes or longer, it returns "" (no padding, never a
// negative amount).
//
// This exists specifically so the @test PASS/FAIL/OUTPUT columns in "ego
// test" output line up regardless of what a test's description string
// contains; see testDirective's use of it for the full rationale.
func PadString(s string, width int) string {
	n := width - len([]rune(s))
	if n <= 0 {
		return ""
	}

	return strings.Repeat(" ", n)
}
