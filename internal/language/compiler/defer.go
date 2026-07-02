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
