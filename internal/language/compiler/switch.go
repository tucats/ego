package compiler

import (
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// compileSwitch compiles a switch statement. The "switch" keyword has already
// been consumed by the caller. Two forms are supported:
//
//  1. Value switch:  switch expr { case v1: ... case v2: ... default: ... }
//     The switch expression is evaluated once and stored in a temporary symbol.
//     Each case value is compared with Equal; the first matching case body runs.
//
//  2. Conditional switch:  switch { case cond1: ... case cond2: ... }
//     No value is computed upfront; each case expression is a full boolean
//     condition evaluated independently.
//
// Optional init-assignment (Ego form):  switch x := expr { ... }
//     The init variable becomes the switch value; a new scope is pushed so the
//     variable is only visible inside the switch block.
//
// Semicolon-separated init form (Go-compatible):  switch x := f(); expr { ... }
//     The init clause is evaluated first and stored under x; then a separate
//     switch expression (which may reference x) is evaluated as the switch value.
//     A new scope is pushed so both x and the switch value are local to the block.
//
// The default block, if present, is compiled separately and appended after all
// case blocks so it only runs when no case matches.
//
// Forward-reference patching is used throughout: each BranchFalse (case miss)
// and Branch (exit after match) is recorded and patched once the target address
// is known.
func (c *Compiler) compileSwitch() error {
	var (
		defaultBlock *bytecode.ByteCode
		// fallThrough holds the bytecode address of a "Branch" instruction
		// emitted for a pending "fallthrough" statement that has not yet been
		// patched to point at the next clause's body. It is passed by pointer
		// into compileSwitchCase(), which both reads it (to patch a fallthrough
		// left over from the *previous* case) and writes it (to record a new
		// one left by the *current* case). A value of 0 means "no fallthrough
		// is currently pending".
		fallThrough int
		// fallThroughToDefault remembers a fallThrough address that must be
		// patched to jump into the default clause once we know where the
		// default clause's bytecode will be appended. This is needed because
		// (see the comment on defaultBlock's use below) the default clause is
		// always compiled into its own separate buffer and spliced in only
		// after every case has been compiled - so its final address in the
		// bytecode stream is not known until the whole switch has been parsed.
		fallThroughToDefault int
		conditional          bool
		next                 int
		switchTestValueName  string
		err                  error
		fixups               = make([]int, 0)
	)

	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.compileError(errors.ErrMissingExpression)
	}

	// The switch value cannot contain a struct initializer
	// that doesn't include a dereference after it. This
	// prevents getting tangled up by the switch syntax which
	// can look like a struct initializer.
	c.flags.disallowStructInits = true
	defer func() {
		c.flags.disallowStructInits = false
	}()

	if c.t.Peek(1).Is(tokenizer.BlockBeginToken) {
		conditional = true
	} else {
		var err error

		// Do we have a symbol to store the value?
		switchTestValueName, err = c.compileSwitchAssignedValue()
		if err != nil {
			return err
		}
	}

	// Switch statement is followed by block syntax, so look for the
	// start of block.
	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.compileError(errors.ErrMissingBlock)
	}

	// Fix BUG-31: push a "switch" entry onto the same stack that "for" loops use
	// for tracking break/continue targets. Before this fix, a bare "break"
	// statement inside a switch's case body had nowhere of its own to attach
	// to, so compileBreak (in for.go) always resolved it against the nearest
	// enclosing *for* loop instead - meaning "break" inside a switch quit the
	// whole loop rather than just the switch (or, if there was no enclosing
	// loop at all, it was rejected as a compile error even though a bare
	// "break" inside a switch is perfectly legal Go). Pushing this entry
	// gives compileBreak somewhere correct to record the branch, and it is
	// popped again below once the switch is fully compiled. Because
	// switchLoopType is a distinct value from the real for-loop types,
	// compileContinue (in for.go) knows to skip straight past it and keep
	// looking for a genuine enclosing for loop, since Go has no notion of
	// "continue the switch".
	c.loopStackPush(switchLoopType)

	// Fix BUG-61 (docs/ISSUES.md): record the scope depth right now, which is
	// exactly the depth a bare "break" inside a case body must land at -
	// compileSwitchAssignedValue's own scope (always pushed now, for every
	// form - conditional switches have no test-value scope at all, so
	// c.scopeDepth here already correctly reflects "no extra scope" for
	// them) has already been pushed above, and nothing the switch's own
	// case/default bodies push is included yet.
	c.loops.scopeDepth = c.scopeDepth

	// Iterate over each case or default selector in the switch block.
	for !c.t.IsNext(tokenizer.BlockEndToken) {
		if next > 0 {
			_ = c.b.SetAddressHere(next)
		}

		// Could be a default statement:
		if c.t.IsNext(tokenizer.DefaultToken) {
			// If the case we just finished ended in "fallthrough", that
			// fallthrough's target is this default clause. We cannot patch
			// the branch address yet, though - compileSwitchDefaultBlock()
			// compiles the default body into a *separate* bytecode buffer
			// (defaultBlock) that only gets spliced into the main bytecode
			// stream after this loop ends, once every case has been seen.
			// So instead of patching now, we squirrel away the pending
			// address in fallThroughToDefault and clear fallThrough, so
			// the "did anything go unpatched?" check after the loop (below)
			// does not mistake this for an error.
			if fallThrough > 0 {
				fallThroughToDefault = fallThrough
				fallThrough = 0
			}

			defaultBlock, err = c.compileSwitchDefaultBlock()
			if err != nil {
				return err
			}
		} else {
			// Compile a case selector
			fixups, err = c.compileSwitchCase(conditional, switchTestValueName, &next, &fallThrough, fixups)
			if err != nil {
				return err
			}
		}
	}

	// If we reach the end of the switch block with a "fallthrough" still
	// pending, it means the last clause in the switch ended in "fallthrough"
	// with no case or default clause after it to fall into. Real Go rejects
	// this at compile time ("cannot fallthrough final case in switch"); Ego
	// used to leave the fallthrough's branch instruction targeting address 0,
	// which re-entered the switch from the top of the function and looped
	// forever. Report it as a compile error instead.
	if fallThrough > 0 {
		return c.compileError(errors.ErrInvalidFallthrough)
	}

	// If there was a last case with conditional, branch it here.
	if next > 0 {
		_ = c.b.SetAddressHere(next)
	}

	// If there was a default block, emit it here
	if defaultBlock != nil {
		// A pending fallthrough that targeted the default clause now knows
		// its destination: the address the default block's bytecode is
		// about to be appended at. Patch the branch instruction before the
		// default block is spliced in, so it lands on the default block's
		// first instruction instead of falling through to address 0.
		if fallThroughToDefault > 0 {
			if err := c.b.SetAddressHere(fallThroughToDefault); err != nil {
				return err
			}
		}

		c.b.Append(defaultBlock)
	}

	// Fixup all the jumps to the exit point
	for _, n := range fixups {
		_ = c.b.SetAddressHere(n)
	}

	// Fix BUG-31: patch every bare "break" statement found directly inside this
	// switch (collected on c.loops.breaks by compileBreak) so that it also
	// lands right here, at the same "just past the last case/default body"
	// address as a normal case miss. This is the point where the switch
	// statement is considered finished, so break behaves exactly like
	// falling out of the bottom of the switch normally would. Once patched,
	// pop the switchLoopType entry pushed earlier so outer code (an
	// enclosing for loop or switch, if any) sees its own loop stack entry on
	// top again.
	for _, fixAddr := range c.loops.breaks {
		_ = c.b.SetAddressHere(fixAddr)
	}

	c.loopStackPop()

	// If we weren't using conditional cases, pop the scope
	// compileSwitchAssignedValue pushed for the value used in case matching
	// (see its own doc comment: a real scope is now pushed uniformly for
	// every non-conditional form, named-init or anonymous, as part of
	// the BUG-61 fix - there is no longer a separate SymbolDelete path).
	// Conditional switches ("switch { case cond: ... }") never call
	// compileSwitchAssignedValue at all, so there is nothing to pop here.
	if !conditional {
		c.emitPopScope()
	}

	return nil
}

// compileSwitchCase compiles a single "case expr:" block inside a switch statement.
//
// For a value switch (conditional == false) the case value is compared against
// the stored switch expression using an Equal instruction. For a conditional
// switch (conditional == true) the case expression is used directly as a boolean.
//
// The next pointer holds the address of the preceding BranchFalse instruction that
// jumps here on a miss; it is patched at the top of this function so that the
// previous case's miss-branch arrives at the current case test. The fallThrough
// pointer, when non-zero, causes the previous case's fall-through branch to land at
// the start of this case body. After the body, a Branch is emitted to skip the rest
// of the switch; its address is appended to the fixups slice and patched later.
func (c *Compiler) compileSwitchCase(conditional bool, switchTestValueName string, next *int, fallThrough *int, fixups []int) ([]int, error) {
	var err error

	if !c.t.IsNext(tokenizer.CaseToken) {
		return nil, errors.ErrMissingCase
	}

	if c.t.IsNext(tokenizer.ColonToken) {
		return nil, errors.ErrMissingColon
	}

	if err := c.emitExpression(); err != nil {
		return nil, err
	}

	if !conditional {
		if err := c.ReferenceSymbol(switchTestValueName); err != nil {
			return nil, err
		}

		c.b.Emit(bytecode.Load, switchTestValueName)
		c.b.Emit(bytecode.Equal)
	}

	for c.t.IsNext(tokenizer.CommaToken) {
		if err := c.emitExpression(); err != nil {
			return nil, err
		}

		if !conditional {
			if err := c.ReferenceSymbol(switchTestValueName); err != nil {
				return nil, err
			}

			c.b.Emit(bytecode.Load, switchTestValueName)
			c.b.Emit(bytecode.Equal)
		}

		c.b.Emit(bytecode.Or)
	}

	*next = c.b.Mark()

	c.b.Emit(bytecode.BranchFalse, 0)

	if !c.t.IsNext(tokenizer.ColonToken) {
		return nil, err
	}

	if *fallThrough > 0 {
		if err := c.b.SetAddressHere(*fallThrough); err != nil {
			return nil, err
		}

		*fallThrough = 0
	}

	// Fix BUG-44: a "case" clause body is its own implicit block, nested
	// inside the switch statement's own scope (the scope that, for a
	// "switch init; expr" or "switch v := expr" form, holds the init
	// variable). Without this, a case body's ":=" declarations were created
	// directly in that outer switch scope, so a case body could not declare
	// a variable with the same name as the switch's init variable (runtime
	// "symbol already exists" error), and - worse, for a plain "switch expr"
	// with no init clause at all - a variable declared in a case body leaked
	// into whatever scope was enclosing the switch statement, colliding with
	// any later, unrelated declaration of the same name after the switch
	// ended. Real Go scopes each case body independently of both the switch
	// and every other case, including across "fallthrough" (a variable
	// declared in a case that falls through is not visible in the case it
	// falls into). Pushing a fresh child scope here, popped again right
	// after the body, matches that: see compileBlock's own PushScope/
	// PushSymbolScope pair for the same pattern used by brace-delimited
	// blocks elsewhere in the compiler.
	// Eligible for PERFORMANCE.md Finding 8 scope elision now that BUG-61
	// (docs/ISSUES.md) is fixed: compileBreak/compileContinue correctly
	// unwind however many scopes they jump over, so a "continue" inside a
	// declaration-free case body no longer needs this scope to exist purely
	// to accidentally mask the switch's own cleanup being skipped - see
	// switchCaseBodyNeedsOwnScope in block.go for the detailed history.
	c.blockDepth++
	c.PushSymbolScope()

	needsScope := c.switchCaseBodyNeedsOwnScope()
	if needsScope {
		c.emitPushScope()
	}

	for !tokenizer.InList(c.t.Peek(1),
		tokenizer.CaseToken,
		tokenizer.DefaultToken,
		tokenizer.FallthroughToken,
		tokenizer.BlockEndToken) {
		if err := c.compileStatement(); err != nil {
			return nil, err
		}
	}

	if needsScope {
		c.emitPopScope()
	}

	c.blockDepth--

	if err := c.PopSymbolScope(); err != nil {
		return nil, err
	}

	if c.t.IsNext(tokenizer.FallthroughToken) {
		c.t.IsNext(tokenizer.SemicolonToken)

		*fallThrough = c.b.Mark()

		c.b.Emit(bytecode.Branch, 0)
	} else {
		fixups = append(fixups, c.b.Mark())

		c.b.Emit(bytecode.Branch, 0)
	}

	return fixups, nil
}

// compileSwitchDefaultBlock compiles the body of the "default:" clause in a
// switch statement. Because the default block must execute only when no case
// matches — regardless of where it appears in source — it is compiled into a
// separate, temporary bytecode buffer and returned to the caller. The caller
// appends it to the main bytecode after all case blocks have been emitted,
// ensuring correct execution order.
func (c *Compiler) compileSwitchDefaultBlock() (*bytecode.ByteCode, error) {
	var defaultBlock *bytecode.ByteCode

	if !c.t.IsNext(tokenizer.ColonToken) {
		return nil, c.compileError(errors.ErrMissingColon)
	}

	savedBC := c.b
	c.b = bytecode.New("default switch")

	// Fix BUG-44: the default clause's body is its own implicit block too,
	// exactly like an ordinary case body - see the matching comment in
	// compileSwitchCase for the full explanation. Also eligible for
	// PERFORMANCE.md Finding 8 scope elision now that BUG-61 is fixed - see
	// switchCaseBodyNeedsOwnScope in block.go.
	c.blockDepth++
	c.PushSymbolScope()

	needsScope := c.switchCaseBodyNeedsOwnScope()
	if needsScope {
		c.emitPushScope()
	}

	for c.t.Peek(1).IsNot(tokenizer.CaseToken) && c.t.Peek(1).IsNot(tokenizer.BlockEndToken) {
		if err := c.compileStatement(); err != nil {
			return nil, err
		}
	}

	if needsScope {
		c.emitPopScope()
	}

	c.blockDepth--

	if err := c.PopSymbolScope(); err != nil {
		return nil, err
	}

	defaultBlock = c.b
	c.b = savedBC

	return defaultBlock, nil
}

// compileSwitchAssignedValue compiles the expression that provides the value
// compared against each case clause in a value switch. Three sub-forms exist:
//
//  1. Named init (Ego form):  switch x := expr { ... }
//     The identifier and ":=" have been peeked at, and the init variable is
//     also used as the switch test value (hasScope == true).
//
//  2. Semicolon-separated init (Go-compatible):  switch x := f(); expr { ... }
//     The init clause is evaluated and stored under x; then a separate switch
//     expression (which may reference x) is evaluated and stored under a new
//     synthetic name that becomes the switch test value (hasScope == true).
//     Anonymous init without a named variable (switch f(); expr { ... }) is
//     also handled: the side-effect-only init result is discarded with Drop.
//
//  3. Anonymous expression:  switch expr { ... }
//     A synthetic unique name is generated to hold the value (hasScope ==
//     false - there is no NAMED init variable in this form, but see below).
//
// In all cases the expression is compiled and stored so that case clauses can
// load it by name for comparison. The returned name is what case bodies use
// for the Load instruction. hasScope (local only - no longer returned, see
// below) reports whether there is a user-visible named init variable; this
// affects only how the semicolon-separated form above is compiled, further
// down.
//
// A scope is ALWAYS pushed here, for all three forms, regardless of
// hasScope - compileSwitch always pops it at the end, so hasScope no longer
// needs to be part of this function's return value. Before BUG-61
// (docs/ISSUES.md) was fixed, only the named-init forms got a real scope;
// the anonymous form's synthetic test-value name was instead created
// directly in the enclosing scope and cleaned up with an explicit
// SymbolDelete at the switch's own normal exit point - a "continue" inside
// a case body could skip that SymbolDelete, leaking the placeholder. Now
// that compileBreak/compileContinue correctly unwind any scope they jump
// over, using a real, uniform scope for every form is both simpler and
// removes that leak entirely: the scope's own PopScope handles it exactly
// like any other case.
func (c *Compiler) compileSwitchAssignedValue() (string, error) {
	var (
		hasScope            bool
		initVarName         string
		switchTestValueName string
	)

	c.emitPushScope()

	if c.t.Peek(1).IsIdentifier() && c.t.Peek(2).Is(tokenizer.DefineToken) {
		initVarName = c.t.Next().Spelling()
		switchTestValueName = initVarName
		hasScope = true

		c.t.Advance(1)
	} else {
		switchTestValueName = data.GenerateName()
	}

	if err := c.emitExpression(); err != nil {
		return "", err
	}

	// Handle the Go semicolon-separated init form: switch x := f(); expr { ... }
	// The first expression has been emitted; if a semicolon follows, treat it as
	// the init clause and parse a second expression as the actual switch value.
	if c.t.IsNext(tokenizer.SemicolonToken) {
		if hasScope {
			// Reject shadowing a built-in type name when
			// ego.compiler.type.shadowing is turned off (BUG-75).
			if err := c.checkTypeShadowing(initVarName); err != nil {
				return "", err
			}

			// Store the init expression under the named variable so that the
			// second expression and the case bodies can reference it.
			c.DefineSymbol(initVarName)
			c.b.Emit(bytecode.CreateAndStore, initVarName)
		} else {
			// Anonymous init (side-effect only): discard the result.
			c.b.Emit(bytecode.Drop)
		}

		// Generate a fresh name for the actual switch test value.
		switchTestValueName = data.GenerateName()
		c.flags.disallowStructInits = true

		if err := c.emitExpression(); err != nil {
			return "", err
		}

		c.DefineSymbol(switchTestValueName)
		c.b.Emit(bytecode.CreateAndStore, switchTestValueName)
		c.flags.disallowStructInits = false
		c.flags.hasUnwrap = false

		return switchTestValueName, nil
	}

	c.DefineSymbol(switchTestValueName)

	if c.flags.hasUnwrap {
		c.b.Emit(bytecode.CreateAndStore, switchTestValueName)

		switchTestValueName = data.GenerateName()
		c.b.Emit(bytecode.CreateAndStore, switchTestValueName)
	} else {
		c.b.Emit(bytecode.CreateAndStore, switchTestValueName)
	}

	c.flags.disallowStructInits = false
	c.flags.hasUnwrap = false

	return switchTestValueName, nil
}
