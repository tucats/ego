package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
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
// Optional init-assignment:  switch x := expr; { ... }
//     When the switch value is assigned to a named variable, a new scope is
//     pushed so the variable is only visible inside the switch block.
//
// The default block, if present, is compiled separately and appended after all
// case blocks so it only runs when no case matches.
//
// Forward-reference patching is used throughout: each BranchFalse (case miss)
// and Branch (exit after match) is recorded and patched once the target address
// is known.
func (c *Compiler) compileSwitch() error {
	var (
		defaultBlock        *bytecode.ByteCode
		fallThrough         int
		conditional         bool
		hasScope            bool
		next                int
		switchTestValueName string
		err                 error
		fixups              = make([]int, 0)
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
		switchTestValueName, hasScope, err = c.compileSwitchAssignedValue()
		if err != nil {
			return err
		}
	}

	// Switch statement is followed by block syntax, so look for the
	// start of block.
	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.compileError(errors.ErrMissingBlock)
	}

	// Iterate over each case or default selector in the switch block.
	for !c.t.IsNext(tokenizer.BlockEndToken) {
		if next > 0 {
			_ = c.b.SetAddressHere(next)
		}

		// Could be a default statement:
		if c.t.IsNext(tokenizer.DefaultToken) {
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

	// If there was a last case with conditional, branch it here.
	if next > 0 {
		_ = c.b.SetAddressHere(next)
	}

	// If there was a default block, emit it here
	if defaultBlock != nil {
		c.b.Append(defaultBlock)
	}

	// Fixup all the jumps to the exit point
	for _, n := range fixups {
		_ = c.b.SetAddressHere(n)
	}

	// If we weren't using conditional cases, clean up the symbol used for
	// the value used for case matching. If we were given one by the source
	// code, we can just delete the scope. Otherwise, it was a unique
	// generated symbol name and code is emitted to delete it.
	if !conditional {
		if hasScope {
			c.b.Emit(bytecode.PopScope)
		} else {
			c.b.Emit(bytecode.SymbolDelete, switchTestValueName)
		}
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

	for !tokenizer.InList(c.t.Peek(1),
		tokenizer.CaseToken,
		tokenizer.DefaultToken,
		tokenizer.FallthroughToken,
		tokenizer.BlockEndToken) {
		if err := c.compileStatement(); err != nil {
			return nil, err
		}
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

	for c.t.Peek(1).IsNot(tokenizer.CaseToken) && c.t.Peek(1).IsNot(tokenizer.BlockEndToken) {
		if err := c.compileStatement(); err != nil {
			return nil, err
		}
	}

	defaultBlock = c.b
	c.b = savedBC

	return defaultBlock, nil
}

// compileSwitchAssignedValue compiles the expression that provides the value
// compared against each case clause in a value switch. Two sub-forms exist:
//
//  1. Named assignment:  switch x := expr; { ... }
//     The identifier and ":=" have been peeked at; a new scope is pushed and
//     the name is used as the storage symbol (hasScope == true).
//
//  2. Anonymous expression:  switch expr { ... }
//     A synthetic unique name is generated to hold the value (hasScope == false).
//
// In both cases the expression is compiled and stored so that case clauses can
// load it by name for comparison. The returned name is what case bodies use
// for the Load instruction; hasScope signals whether a PopScope is needed at
// the end of the switch.
func (c *Compiler) compileSwitchAssignedValue() (string, bool, error) {
	var (
		hasScope            bool
		switchTestValueName string
	)

	if c.t.Peek(1).IsIdentifier() && c.t.Peek(2).Is(tokenizer.DefineToken) {
		switchTestValueName = c.t.Next().Spelling()
		hasScope = true

		c.b.Emit(bytecode.PushScope)
		c.t.Advance(1)
	} else {
		switchTestValueName = data.GenerateName()
	}

	if err := c.emitExpression(); err != nil {
		return "", false, err
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

	return switchTestValueName, hasScope, nil
}
