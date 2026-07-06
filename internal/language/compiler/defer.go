package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	bc "github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// findDeferCallEnd returns the token-stream position immediately after the
// closing ')' of the call expression that starts at startPos.  It skips the
// leading identifier chain (e.g. "wg.Done") and then counts parentheses to
// find the matching close of the argument list.
func (c *Compiler) findDeferCallEnd(startPos int) int {
	tokens := c.t.Tokens
	pos := startPos
	n := len(tokens)

	// Skip identifier tokens and dots (e.g. "wg.Done").
	for pos < n {
		t := tokens[pos]
		if t.IsIdentifier() || t.Is(tokenizer.DotToken) {
			pos++
		} else {
			break
		}
	}

	// Count parentheses to locate the matching close of the argument list.
	depth := 0

	for pos < n {
		t := tokens[pos]
		pos++

		if t.Is(tokenizer.StartOfListToken) {
			depth++
		} else if t.Is(tokenizer.EndOfListToken) {
			depth--
			if depth == 0 {
				return pos
			}
		}
	}

	return pos
}

// findDeferCallArgsStart returns the token-stream position of the '(' token
// that opens a call expression's argument list, given the position of the
// call's very first token (the callee name, or the first name in a dotted
// chain like "wg.Done"). It skips over the identifier/dot chain exactly the
// same way findDeferCallEnd does, but stops at the opening parenthesis
// instead of continuing on to find the matching close.
func (c *Compiler) findDeferCallArgsStart(startPos int) int {
	tokens := c.t.Tokens
	pos := startPos
	n := len(tokens)

	for pos < n {
		t := tokens[pos]
		if t.IsIdentifier() || t.Is(tokenizer.DotToken) {
			pos++
		} else {
			break
		}
	}

	return pos
}

// hoistDeferCallArguments is the fix for BUG-16 / FLOW-M4. It makes
// "defer namedFunc(arg1, arg2)" evaluate arg1 and arg2 *immediately*, at the
// point where the "defer" statement appears in the program, instead of
// leaving their source text to be re-evaluated later when the deferred call
// finally runs (at function-return time). That "later" re-evaluation is what
// caused the bug: if a variable used as an argument changed value between the
// "defer" statement and the function returning, the deferred call would see
// the *new* value instead of the value that was current when "defer" ran --
// the opposite of what Go does.
//
// How it works: compileDefer() (below) always compiles a call like
// "namedFunc(arg1, arg2)" by rewriting it into an equivalent closure literal
// that gets called immediately -- "func(){ namedFunc(arg1, arg2) }()" -- and
// then turning that immediate call into a deferred one. The closure's BODY
// (everything between "{" and "}") is only compiled into bytecode once, but
// that bytecode only actually *runs* later, when the deferred call executes.
// So any expression left inside the closure body -- including "arg1" and
// "arg2" exactly as the user wrote them -- is naturally evaluated late, which
// is correct for the function call itself (Go defers the CALL) but wrong for
// the ARGUMENTS (Go does not defer evaluating them).
//
// The fix is to compile each argument expression *before* that wrapping
// happens, right here in the compiler's normal (non-deferred) output stream,
// and immediately store each computed value into its own uniquely-named
// temporary variable (using the same "generate a private $N name and
// StoreAlways into it" idiom already used elsewhere in the compiler for
// "materialize this expression's value under a synthetic name", e.g.
// compileAddressOf/compilePointerDereference in expr_atom.go). Because this
// all happens using the compiler's normal bytecode stream, it runs at
// "defer"-statement time, not at deferred-call time -- exactly what we want.
//
// Once each value is safely frozen in its own temp variable, the original
// argument expressions in the token stream are replaced with references to
// those temp variables (e.g. "namedFunc(arg1, arg2)" becomes
// "namedFunc($1, $2)"). The existing closure-wrapping logic then proceeds
// completely unchanged: it wraps "namedFunc($1, $2)" instead of
// "namedFunc(arg1, arg2)". When the deferred closure eventually runs, "$1"
// and "$2" are simply variable *reads* of values that were already computed
// and frozen long ago -- there is nothing left to re-evaluate.
//
// Temp variable names starting with "$" are a compiler-internal convention:
// DefineSymbol/ReferenceSymbol (symbols.go) both special-case names with that
// prefix and never flag them as unused or undeclared, so no additional
// bookkeeping is required to make "$1"/"$2" visible to the nested closure --
// closures already resolve free (outer-scope) variable names by reading the
// live symbol table at runtime, which is exactly how a normal closure like
// "defer func(){ log = log + x }()" already reads the outer "x" today.
//
// If the call being deferred takes no arguments (e.g. "defer wg.Done()") or
// isn't actually a call at all (a malformed defer statement), this function
// leaves the token stream untouched and lets the existing, unrelated
// validation further down in compileDefer() report the appropriate error.
func (c *Compiler) hoistDeferCallArguments() error {
	// startPos is the position of the callee's first token (e.g. "wg" in
	// "wg.Done()", or "setLog" in "setLog(x)"). We must leave the tokenizer
	// positioned here when we're done, because that's what the rest of
	// compileDefer() expects to find next.
	startPos := c.t.Mark()

	argsStart := c.findDeferCallArgsStart(startPos)
	if argsStart >= len(c.t.Tokens) || c.t.Tokens[argsStart].IsNot(tokenizer.StartOfListToken) {
		// There's no "(...)" at all (e.g. a malformed "defer obj.field" with
		// no call). Nothing to hoist -- leave this for the existing
		// "must end in a Call instruction" check to reject.
		return nil
	}

	c.t.Set(argsStart)
	c.t.Advance(1) // Move past the '(' so we're looking at the first argument (or ')').

	var tempNames []string

	// variadicLast becomes true if the final argument was followed by "...",
	// meaning it names a slice whose elements should be spread into
	// individual arguments when the call actually executes. Exactly as in an
	// ordinary function call (see functionCall() in expr_function.go), "..."
	// may only appear after the last argument.
	variadicLast := false

	for !c.t.Peek(1).Is(tokenizer.EndOfListToken) {
		// Compile one argument expression right now, into the compiler's
		// normal (non-deferred) bytecode stream. This is the key step: it
		// makes the argument's value get computed at "defer"-statement time.
		if err := c.conditional(); err != nil {
			return err
		}

		// fix BUG-43 (broadened): a struct-valued argument must be an
		// independent copy, exactly as an ordinary "tempName := arg" short
		// variable declaration would produce, not an alias of the caller's
		// variable -- otherwise mutating the original struct after the
		// "defer" statement would still be visible when the deferred call
		// finally runs. See ValueCopy's own comment (bytecode/store.go) for
		// why this is a separate instruction rather than switching to
		// CreateAndStore.
		c.b.Emit(bytecode.ValueCopy)

		tempName := data.GenerateName()
		c.b.Emit(bytecode.StoreAlways, tempName)
		tempNames = append(tempNames, tempName)

		if c.t.IsNext(tokenizer.VariadicToken) {
			variadicLast = true

			break
		}

		if c.t.Peek(1).Is(tokenizer.EndOfListToken) {
			break
		}

		if c.t.Peek(1).IsNot(tokenizer.CommaToken) {
			return c.compileError(errors.ErrInvalidList)
		}

		c.t.Advance(1)
	}

	if c.t.Peek(1).IsNot(tokenizer.EndOfListToken) {
		return c.compileError(errors.ErrMissingParenthesis)
	}

	if len(tempNames) == 0 {
		// No arguments to hoist (e.g. "defer wg.Done()"). Nothing in the
		// token stream needs to change; just put the read position back
		// where the caller expects it.
		c.t.Set(startPos)

		return nil
	}

	// argsEnd is one token past the closing ')', so that Delete(argsStart,
	// argsEnd) removes the entire "(...)" argument list, ')' included.
	argsEnd := c.t.Mark() + 1

	// Build the replacement argument list: "($1, $2, ...)" using the
	// temporary names we just generated, in the same order as the original
	// arguments, with the "..." flatten marker reattached to the last one if
	// the original call used it.
	replacement := make([]tokenizer.Token, 0, len(tempNames)*2+1)
	replacement = append(replacement, tokenizer.StartOfListToken)

	for i, name := range tempNames {
		if i > 0 {
			replacement = append(replacement, tokenizer.CommaToken)
		}

		replacement = append(replacement, tokenizer.NewIdentifierToken(name))

		if variadicLast && i == len(tempNames)-1 {
			replacement = append(replacement, tokenizer.VariadicToken)
		}
	}

	replacement = append(replacement, tokenizer.EndOfListToken)

	if err := c.t.Delete(argsStart, argsEnd); err != nil {
		return c.compileError(err)
	}

	if err := c.t.Insert(argsStart, replacement...); err != nil {
		return c.compileError(err)
	}

	// Put the read position back at the callee's first token so the rest of
	// compileDefer() sees "namedFunc($1, $2)" exactly where it originally saw
	// "namedFunc(arg1, arg2)".
	c.t.Set(startPos)

	return nil
}

// hoistDeferReceiver is the fix for BUG-43. It complements
// hoistDeferCallArguments above: that function freezes a deferred call's
// ARGUMENTS at "defer"-statement time, but left the RECEIVER of a dotted call
// (the "l" in "defer l.Log(msg)", or the "wg" in "defer wg.Done()") to be
// re-resolved from the live symbol table when the deferred call finally runs.
// That is wrong for the same reason BUG-16/FLOW-M4 was wrong for arguments:
// real Go evaluates and saves the entire function/method value -- including
// any receiver or package selector -- at the point "defer" executes, not at
// the point the deferred call actually happens. Mutating the receiver
// variable between the "defer" statement and the function returning must not
// change which receiver the deferred call observes.
//
// Must be called AFTER hoistDeferCallArguments, so the call's argument list
// has already been reduced to a simple "($1, $2, ...)" form of hoisted temp
// variables; this function only needs to locate the boundary between the
// receiver chain and the final ".MethodName(...)" suffix, which is
// unaffected by having already hoisted the arguments.
//
// How it works: the receiver of a deferred call is constrained (by the
// existing validation earlier in compileDefer, which requires the callee to
// start with a bare identifier) to be a simple dotted identifier chain --
// "l", "wg.mu", "pkg.Type", etc. -- never an arbitrary expression such as an
// array index or a parenthesized dereference. That means the receiver chain
// can always be found by looking for the LAST "." token before the call's
// opening "(": everything before that dot is the receiver expression;
// everything from that dot onward ("MethodName(...)") is the call itself and
// must be left untouched, since re-compiling it inside the later-deferred
// closure is exactly what causes the runtime's SetThis mechanism (see
// compileDotReference in expr_reference.go) to correctly bind whatever
// receiver value it finds at that time -- which will now be our frozen one.
//
// The receiver chain's tokens are removed from the stream, compiled right
// here (in the compiler's normal, non-deferred bytecode stream, exactly like
// hoistDeferCallArguments does for arguments) so its value is computed NOW,
// and stored into its own temp variable. The token stream is then rewritten
// from "receiverChain.MethodName(args)" to "$tempName.MethodName(args)", so
// the closure-wrapping logic that follows in compileDefer wraps a call whose
// receiver is a frozen temp variable rather than the original, possibly
// later-mutated, expression.
//
// If there is no "." at all in the callee (a bare "defer namedFunc(args)"
// call has no receiver to freeze), this function leaves the token stream
// untouched. That bare-call case is FLOW-M4's own, distinct, still-open gap
// (lazy re-read of the function reference itself), not this one.
func (c *Compiler) hoistDeferReceiver() error {
	startPos := c.t.Mark()

	argsStart := c.findDeferCallArgsStart(startPos)
	if argsStart >= len(c.t.Tokens) || c.t.Tokens[argsStart].IsNot(tokenizer.StartOfListToken) {
		// No "(...)" at all -- same guard as hoistDeferCallArguments; leave
		// this for the existing "must end in a Call instruction" check.
		c.t.Set(startPos)

		return nil
	}

	// Find the last "." in the identifier chain before the "(" -- the dot
	// that separates the receiver (or package qualifier) from the final
	// method/function name, e.g. the "." right before "Lock" in "wg.mu.Lock".
	lastDot := -1

	for i := startPos; i < argsStart; i++ {
		if c.t.Tokens[i].Is(tokenizer.DotToken) {
			lastDot = i
		}
	}

	if lastDot < 0 {
		// A bare "defer namedFunc(...)" call -- no receiver to freeze.
		c.t.Set(startPos)

		return nil
	}

	// methodChainEnd is one token past the matching ")" of the call: the full
	// extent of "receiverChain.MethodName(args)" that is about to be split
	// into a frozen receiver plus an untouched call suffix.
	methodChainEnd := c.findDeferCallEnd(startPos)

	// Save the call suffix -- the final ".MethodName(args)" -- so it can be
	// reattached, unchanged, after the receiver's frozen replacement.
	suffix := make([]tokenizer.Token, methodChainEnd-lastDot)
	copy(suffix, c.t.Tokens[lastDot:methodChainEnd])

	if err := c.t.Delete(lastDot, methodChainEnd); err != nil {
		return c.compileError(err)
	}

	// Only the receiver chain itself (e.g. "l" or "wg.mu") remains at
	// startPos now. Compile it right here, in the compiler's normal
	// (non-deferred) bytecode stream -- the same "compile now, store to a
	// temp variable" idiom hoistDeferCallArguments uses for arguments -- so
	// its value is computed at "defer"-statement time.
	c.t.Set(startPos)

	if err := c.conditional(); err != nil {
		return err
	}

	// This is the crux of BUG-43: the receiver must be an independent copy
	// when it is a struct value, exactly as "tempName := l" would produce,
	// not an alias of the live "l" -- otherwise mutating l's fields after
	// the "defer" statement (the whole point of the bug report) would still
	// be visible through the frozen receiver when the deferred call finally
	// runs. See ValueCopy's own comment (bytecode/store.go) for why this is
	// a separate instruction rather than switching to CreateAndStore.
	c.b.Emit(bytecode.ValueCopy)

	tempName := data.GenerateName()
	c.b.Emit(bytecode.StoreAlways, tempName)

	// Replace the (now fully consumed) receiver-chain tokens with a single
	// reference to the frozen temp variable, then reattach the untouched
	// ".MethodName(args)" suffix immediately after it.
	receiverEnd := c.t.Mark()

	if err := c.t.Delete(startPos, receiverEnd); err != nil {
		return c.compileError(err)
	}

	replacement := append([]tokenizer.Token{tokenizer.NewIdentifierToken(tempName)}, suffix...)

	if err := c.t.Insert(startPos, replacement...); err != nil {
		return c.compileError(err)
	}

	// Put the read position back at the (rewritten) callee's first token, as
	// hoistDeferCallArguments does, so the rest of compileDefer() sees
	// "$tempName.MethodName(args)" exactly where it originally saw
	// "receiverChain.MethodName(args)".
	c.t.Set(startPos)

	return nil
}

// compileDefer compiles the "defer" statement. This compiles a statement,
// and attaches the resulting bytecode to the compilation unit's defer queue.
// Later, when a return is processed, this queue will be used to generate the
// appropriate deferred operations. The order of the "defer" statements determines
// the order in the queue, and therefore the order in which they are run when a
// return is executed.
func (c *Compiler) compileDefer() error {
	minDepth := 1
	if c.flags.exitEnabled {
		minDepth = 2
	}

	if c.functionDepth < minDepth {
		return c.compileError(errors.ErrDeferOutsideFunction)
	}

	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrMissingFunction)
	}

	// Is it a function constant?
	if c.t.IsNext(tokenizer.FuncToken) {
		c.b.Emit(bc.DeferStart, true)

		// Compile a function literal onto the stack.
		if err := c.compileFunctionDefinition(c.isLiteralFunction()); err != nil {
			return err
		}
	} else {
		c.b.Emit(bc.DeferStart, true)

		// Peek ahead to see if this is a legit function call. If the next token is not an
		// identifier, and it's not followed by a parenthesis or dot-notation identifier,
		// then this is not a function call and we're done.
		if !c.t.Peek(1).IsIdentifier() || (c.t.Peek(2).IsNot(tokenizer.StartOfListToken) && c.t.Peek(2).IsNot(tokenizer.DotToken)) {
			return c.compileError(errors.ErrInvalidFunctionCall)
		}

		// fixed BUG-16 / FLOW-M4 fix: compile and freeze the call's arguments right
		// now, before the call gets wrapped in the anonymous closure below.
		// See hoistDeferCallArguments()'s own comment for the full
		// explanation of why this is necessary and how it works.
		if err := c.hoistDeferCallArguments(); err != nil {
			return err
		}

		// fixed BUG-43 fix: compile and freeze the call's RECEIVER (e.g. the "l" in
		// "defer l.Log(msg)"), if any, right now as well -- for the same reason as
		// above, just applied to the receiver instead of the arguments. Must run
		// after hoistDeferCallArguments(); see hoistDeferReceiver()'s own comment
		// for the full explanation.
		if err := c.hoistDeferReceiver(); err != nil {
			return err
		}

		// Wrap the call in an anonymous function literal so the full call — including
		// receiver setup (LoadThis / "__this") — is deferred rather than just the
		// resolved function pointer.  Logically:  defer f()  →  defer func(){ f() }()
		startPos := c.t.Mark()
		endPos := c.findDeferCallEnd(startPos)

		// Insert '}()' after the call expression.  TokenP < endPos so it is not adjusted.
		c.t.Insert(endPos, tokenizer.BlockEndToken, tokenizer.StartOfListToken, tokenizer.EndOfListToken)

		// Insert 'func(){' before the call.  Insert advances TokenP past the new tokens,
		// so reset it back to startPos so the compiler reads from 'func'.
		c.t.Insert(startPos, tokenizer.FuncToken, tokenizer.StartOfListToken, tokenizer.EndOfListToken, tokenizer.BlockBeginToken)
		c.t.Set(startPos)

		// Consume the injected 'func' token and compile the anonymous literal.
		c.t.IsNext(tokenizer.FuncToken)

		if err := c.compileFunctionDefinition(c.isLiteralFunction()); err != nil {
			return err
		}
	}

	// Let's stop now and see if the stack looks right.
	lastBytecode := c.b.Mark()

	i := c.b.Instruction(lastBytecode - 1)
	if i.Operation != bytecode.Call {
		return c.compileError(errors.ErrInvalidFunctionCall)
	}

	argc, err := data.Int(i.Operand)
	if err != nil {
		return c.compileError(err)
	}

	// Drop the Call operation from the end of the bytecode
	// and replace with the "defer function" operation.
	c.b.Delete(lastBytecode - 1)
	c.b.Emit(bc.Defer, argc)

	return nil
}
