package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// ---------------------------------------------------------------------------
// PERFORMANCE.md Finding 8: skip the scope for a block that declares nothing
//
// PERFORMANCE.md Finding 4 already stopped the compiler from re-allocating a
// fresh runtime scope on every iteration of a for-loop whose body never
// declares a local and never captures a variable by reference. This section
// applies the exact same idea one level lower: to any brace-enclosed block,
// wherever it appears (an "if"/"else" body, a function body, a switch
// "case"/"default" body, a "try"/"catch" body, or a bare "{ ... }" statement).
//
// A PushScope/PopScope pair exists for exactly one reason: to give a block's
// own ":="/"var" declarations a home that is discarded when the block exits,
// so a second execution of the same block (e.g. because it sits inside a
// loop, or the enclosing function is called again) does not collide with the
// first. When a block declares nothing of its own, there is nothing that
// needs that isolation: any variable a nested closure captures belongs to
// SOME enclosing construct (a function call or a for-loop body), and that
// construct is already responsible for giving ITS OWN scope the correct
// lifetime. Removing an empty pass-through scope layer in between changes
// nothing about which names are visible or how long they live.
//
// scopeElisionDisqualified (below) is a deliberately conservative token-level
// scan - not a full semantic analysis - matching the spirit of Finding 4's own
// predicate. It disqualifies a block from scope elision, forcing the always
// -scoped behavior, whenever the block contains, anywhere:
//
//   - a function literal, a "go" statement, or a "defer" (kept as a
//     precaution even though a block with no local declarations of its own
//     has nothing scope-identity-sensitive for a closure to capture; erring
//     conservative here costs nothing but a missed optimization), or
//   - a ":=", "var", "const", "type", "import", or "package" declaration
//     directly at the block's own top level (brace-depth 0 relative to the
//     block itself) - statement.go's compileStatement() dispatches all six
//     of these as legal both at the top level AND inside a function body,
//     and every one of them binds a new name into whatever scope is
//     current at the time (a runtime StoreAlways/Store, not merely
//     compile-time bookkeeping). This is the one case that would actually
//     be unsafe to elide: without the block's own scope, that name is
//     created directly in the enclosing scope instead, where it can
//     collide with (or shadow, undetected) an unrelated, later declaration
//     of the same name once the block that "should" have owned it is gone.
//     A "const" declaration with no matching scope is what originally
//     surfaced this: internal/language/compiler's own test harness runs
//     many independent Ego programs against one shared root symbol table,
//     and a leaked immutable constant from one program collided with an
//     unrelated later variable of the same name in another.
//
// Being overly cautious only costs a missed optimization opportunity, never
// correctness: a nested block's own "if x := f(); ... {}" init-clause ":="
// sitting at depth 0 (before ITS OWN opening brace) is treated as
// disqualifying even though it is actually scoped to the nested construct,
// exactly like the equivalent gap already documented for Finding 4.
// ---------------------------------------------------------------------------

// blockBodyNeedsOwnScope decides whether a brace-delimited block (an
// "if"/"else" body, a function body, a "try"/"catch" body, or a bare
// "{ ... }" statement) needs its own runtime scope. It must be called with
// the tokenizer positioned exactly as compileBlock expects to be called -
// i.e. the block's opening "{" has already been consumed, so the current
// position is the first token of the block's body.
//
// The scan tracks nested-brace depth so that declarations properly scoped to
// a NESTED block (if/for/switch/etc. inside this one) do not disqualify this
// block itself; it stops as soon as it reaches, at its own top level (depth
// 0), the block's own matching "}".
func (c *Compiler) blockBodyNeedsOwnScope() bool {
	return c.scopeElisionScan(nil)
}

// switchCaseBodyNeedsOwnScope is the switch "case"/"default" body
// equivalent of blockBodyNeedsOwnScope. Unlike an ordinary block, a case
// body is not brace-delimited - it runs until the next "case", "default",
// "fallthrough", or the switch's own closing "}" (see compileSwitchCase and
// compileSwitchDefaultBlock). It must be called with the tokenizer
// positioned at the first token of the case body (immediately after the
// clause's ":").
//
// Until BUG-61 (docs/ISSUES.md) was fixed, this call site was deliberately
// excluded from scope elision: eliding a case body's scope removed an
// accidental masking of a separate bug (a "continue" inside a case body
// skipped the switch's own synthetic-symbol/scope cleanup), making that bug
// immediately reproducible instead of merely latent. With BUG-61's general
// fix - compileBreak/compileContinue now correctly unwind however many
// scopes they jump over, and the switch's own test-value scope is always
// real, never a bare SymbolDelete - that masking is no longer needed, and
// case/default bodies are safe to elide like any other block.
func (c *Compiler) switchCaseBodyNeedsOwnScope() bool {
	return c.scopeElisionScan(func(tok tokenizer.Token) bool {
		return tok.Is(tokenizer.CaseToken) ||
			tok.Is(tokenizer.DefaultToken) ||
			tok.Is(tokenizer.FallthroughToken)
	})
}

// scopeElisionScan is the shared token-level scan behind
// blockBodyNeedsOwnScope and switchCaseBodyNeedsOwnScope: a deliberately
// conservative scan - not a full semantic analysis - matching the spirit of
// PERFORMANCE.md Finding 4's own predicate. It disqualifies a block from
// scope elision, forcing the always-scoped behavior, whenever the body
// contains, anywhere:
//
//   - a function literal, a "go" statement, or a "defer" (kept as a
//     precaution even though a block with no local declarations of its own
//     has nothing scope-identity-sensitive for a closure to capture; erring
//     conservative here costs nothing but a missed optimization), or
//   - a ":=", "var", "const", "type", "import", or "package" declaration
//     directly at the body's own top level (brace-depth 0 relative to the
//     body itself) - statement.go's compileStatement() dispatches all six
//     of these as legal both at the top level AND inside a function body,
//     and every one of them binds a new name into whatever scope is
//     current at the time (a runtime StoreAlways/Store, not merely
//     compile-time bookkeeping). This is the one case that would actually
//     be unsafe to elide: without the body's own scope, that name is
//     created directly in the enclosing scope instead, where it can
//     collide with (or shadow, undetected) an unrelated, later declaration
//     of the same name once the body that "should" have owned it is gone.
//     A "const" declaration with no matching scope is what originally
//     surfaced this: internal/language/compiler's own test harness runs
//     many independent Ego programs against one shared root symbol table,
//     and a leaked immutable constant from one program collided with an
//     unrelated later variable of the same name in another.
//
// Being overly cautious only costs a missed optimization opportunity, never
// correctness: a nested block's own "if x := f(); ... {}" init-clause ":="
// sitting at depth 0 (before ITS OWN opening brace) is treated as
// disqualifying even though it is actually scoped to the nested construct,
// exactly like the equivalent gap already documented for Finding 4.
//
// The scan stops as soon as it reaches, at its own top level (depth 0),
// either the body's own matching "}" or - for a non-brace-delimited body,
// such as a switch case - a token for which isTerminator returns true. Pass
// a nil isTerminator when the body is always brace-delimited (the "}" check
// alone is sufficient).
func (c *Compiler) scopeElisionScan(isTerminator func(tokenizer.Token) bool) bool {
	pos := c.t.Mark()

	if pos >= len(c.t.Tokens) {
		return true
	}

	depth := 0

	for i := pos; i < len(c.t.Tokens); i++ {
		tok := c.t.Tokens[i]

		switch {
		case tok.Is(tokenizer.BlockBeginToken):
			depth++

		case tok.Is(tokenizer.BlockEndToken):
			// depth == 0 means this is OUR OWN closing brace (for a
			// brace-delimited body) - the whole body has been scanned with
			// nothing disqualifying found.
			if depth == 0 {
				return false
			}

			depth--

		case tok.Is(tokenizer.FuncToken), tok.Is(tokenizer.GoToken), tok.Is(tokenizer.DeferToken):
			return true

		case depth == 0 && (tok.Is(tokenizer.DefineToken) ||
			tok.Is(tokenizer.VarToken) ||
			tok.Is(tokenizer.ConstToken) ||
			tok.Is(tokenizer.TypeToken) ||
			tok.Is(tokenizer.ImportToken) ||
			tok.Is(tokenizer.PackageToken)):
			return true

		case depth == 0 && isTerminator != nil && isTerminator(tok):
			// A non-brace-delimited body (a switch case/default) ends here,
			// with nothing disqualifying found.
			return false
		}
	}

	// Unbalanced braces / ran off the end of the token stream: malformed
	// input the parser would already reject elsewhere. Take the safe path.
	return true
}

// ---------------------------------------------------------------------------
// Fix BUG-61: keep c.scopeDepth in lockstep with the actual bytecode
//
// emitPushScope and emitPopScope are the ONLY places that should ever emit a
// bytecode.PushScope/PopScope instruction for a block/switch/try scope (loop
// scopes are the one exception - see for.go's own bookkeeping, which mirrors
// this same pattern for the ForScope and per-iteration/shared body scopes).
// Every caller that used to write "c.b.Emit(bytecode.PushScope, ...)"/
// "c.b.Emit(bytecode.PopScope)" directly has been converted to call these
// instead, so that c.scopeDepth - a running count of how many scopes are
// open at any given point in the bytecode being generated - never drifts out
// of sync with reality. compileBreak/compileContinue (for.go) depend on that
// accuracy to compute how many scopes to explicitly pop before branching out
// of however many nested constructs separate them from their target loop.
// ---------------------------------------------------------------------------

// emitPushScope emits a bytecode.PushScope instruction (with optional
// operands, e.g. bytecode.ForScope) and records that one more scope is now
// open. Always use this instead of emitting PushScope directly.
func (c *Compiler) emitPushScope(operands ...any) {
	c.b.Emit(bytecode.PushScope, operands...)

	c.scopeDepth++
}

// emitPopScope emits a bytecode.PopScope instruction and records that the
// matching scope has closed. Always use this instead of emitting PopScope
// directly.
func (c *Compiler) emitPopScope() {
	c.b.Emit(bytecode.PopScope)
	
	c.scopeDepth--
}

// compileBlock compiles a brace-enclosed statement block. The caller must have
// already consumed the opening "{" token before calling this function.
//
// A block introduces a new symbol scope. Any variables declared with ":=" or
// "var" inside the block are local to that block and are discarded when the
// block exits. This is implemented by emitting a PushScope bytecode at the
// start and a PopScope at the end; the runtime interpreter creates and destroys
// a child symbol table to match.
//
// mayElideScope opts this call site in to PERFORMANCE.md Finding 8: when
// true, and blockBodyNeedsOwnScope() finds nothing that requires a runtime
// scope, the PushScope/PopScope pair is skipped entirely. Compile-time
// bookkeeping (blockDepth, PushSymbolScope/PopSymbolScope, and therefore
// unused-variable detection) always happens regardless, exactly as it does
// for every other block - only the runtime bytecode is conditional. Passing
// false preserves the exact previous behavior (always scoped).
//
// Semicolons between statements are silently consumed. If the token stream ends
// before a closing "}" is found, a compile error is returned.
func (c *Compiler) compileBlock(runDefers bool, mayElideScope bool) error {
	parsing := true
	c.blockDepth++

	// Tell the symbol-usage tracker that we are entering a new scope so it
	// can detect variables that are declared but never read.
	c.PushSymbolScope()

	needsScope := true
	if mayElideScope {
		needsScope = c.blockBodyNeedsOwnScope()
	}

	// At runtime, PushScope creates a child symbol table so that variables
	// declared inside this block shadow but do not overwrite outer variables.
	if needsScope {
		c.emitPushScope()
	}

	for parsing {
		// A closing "}" ends the block.
		if c.t.IsNext(tokenizer.BlockEndToken) {
			break
		}

		if err := c.compileStatement(); err != nil {
			return err
		}

		// The tokenizer may insert semicolons between statements; skip them.
		_ = c.t.IsNext(tokenizer.SemicolonToken)

		// If we run out of tokens without a "}", the source is malformed.
		if c.t.AtEnd() {
			return c.compileError(errors.ErrMissingEndOfBlock)
		}
	}

	// If this block supports `defer` statements, run them now as we exit
	// the block. This is unconditional on runDefers, independent of
	// needsScope: a block that qualifies for scope elision can never
	// contain a "defer" in the first place (blockBodyNeedsOwnScope treats
	// any "defer" as disqualifying), so RunDefers here is a correctly-timed
	// no-op for elided blocks and unchanged behavior for scoped ones.
	if runDefers {
		c.b.Emit(bytecode.RunDefers)
	}

	// Emit the matching PopScope so the runtime destroys the child symbol
	// table when execution leaves the block.
	if needsScope {
		c.emitPopScope()
	}

	c.blockDepth--

	// PopSymbolScope checks for any variables that were declared but never
	// referenced, reporting them as errors when unused-variable checking is on.
	return c.PopSymbolScope()
}

// compileRequiredBlock compiles a block that must be present. Unlike
// compileBlock, this function consumes the opening "{" itself (or accepts
// an empty-block token "{}"). If neither is found, a compile error is
// returned.
//
// IF this is a function block, runDefers will be true and causes the exit
// from the block to emit RunDefers which runs any pending defers for this
// call frame.
//
// mayElideScope is passed straight through to compileBlock - see its doc
// comment for what it controls.
//
// This helper is used by if, for, func, switch, try, and other statements
// that are always followed by a block body.
func (c *Compiler) compileRequiredBlock(runDefers bool, mayElideScope bool) error {
	// An empty block ({}) is legal and generates no code.
	if c.t.IsNext(tokenizer.EmptyBlockToken) {
		return nil
	}

	// A non-empty block must start with "{".
	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.compileError(errors.ErrMissingBlock)
	}

	return c.compileBlock(runDefers, mayElideScope)
}
