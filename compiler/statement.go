package compiler

import (
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// compileStatement compiles a single statement.
func (c *Compiler) compileStatement() *errors.EgoError {
	// We just eat statement separators and empty blocks, and also
	// terminate processing when we hit the end of the token stream
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return nil
	}

	if c.t.IsNext(tokenizer.EmptyBlockToken) {
		// Empty body at end of token array means no more at-lines...
		if c.t.TokenP < len(c.t.Line) {
			c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])
		}

		return nil
	}

	// Is it a directive token? These really just store data in the compiler
	// symbol table that is used to extend features. These symbols end up in
	// the runtime context of the running code
	if c.t.IsNext("@") {
		return c.compileDirective()
	}

	c.statementCount = c.statementCount + 1

	// Is it a function definition? These aren't compiled inline,
	// so we call a special compile unit that will compile the
	// function and store it in the bytecode symbol table.
	if c.t.IsNext(tokenizer.FuncToken) {
		return c.compileFunctionDefinition(false)
	}

	if c.t.IsNext("panic") && settings.GetBool(defs.RuntimePanicsSetting) {
		return c.compilePanic()
	}
	// At this point, we know we're trying to compile a statement,
	// so store the current line number in the stream to help us
	// form runtime error messages as needed.
	if c.t.TokenP < len(c.t.Line) {
		c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])
	}

	// Is it a function call? We only do this if we are already in
	// the body of a function.

	if c.functionDepth > 0 && c.isFunctionCall() {
		return c.compileFunctionCall()
	}

	// If the next item(s) constitute a value LValue, then this is
	// an assignment statement.
	if c.functionDepth > 0 && c.isAssignmentTarget() {
		return c.compileAssignment()
	}

	// Remaining statement types all have a starting term that defines
	// which compiler unit to call. For each term, call the appropriate
	// handler (which assumes the leading verb has already been consumed)
	verb := c.t.Next()

	// First, check the statements that can appear anywhere.
	switch verb {
	case tokenizer.ConstToken:
		return c.compileConst()

	case tokenizer.ImportToken:
		return c.compileImport()

	case tokenizer.PackageToken:
		return c.compilePackage()

	case tokenizer.TypeToken:
		return c.compileTypeDefinition()

	case tokenizer.VarToken:
		return c.compileVar()
	}

	// If we are in the body of a function, the rest of these are
	// also valid.
	if c.functionDepth > 0 {
		switch verb {
		case tokenizer.BlockBeginToken:
			return c.compileBlock()

		case tokenizer.AssertToken:
			if c.extensionsEnabled {
				return c.Assert()
			}

			return c.newError(errors.ErrUnrecognizedStatement, c.t.Peek(0))

		case tokenizer.BreakToken:
			return c.compileBreak()

		case tokenizer.CallToken:
			if c.extensionsEnabled {
				return c.compileFunctionCall()
			}

		case tokenizer.ContinueToken:
			return c.compileContinue()

		case tokenizer.DeferToken:
			return c.compileDefer()

		case tokenizer.ExitToken:
			if c.exitEnabled {
				return c.compileExit()
			}

		case tokenizer.ForToken:
			return c.compileFor()

		case tokenizer.GoToken:
			return c.compileGo()

		case tokenizer.IfToken:
			return c.compileIf()

		case tokenizer.PrintToken:
			if c.extensionsEnabled {
				return c.compilePrint()
			}

		case tokenizer.ReturnToken:
			return c.compileReturn()

		case tokenizer.SwitchToken:
			return c.compileSwitch()

		case tokenizer.TryToken:
			if c.extensionsEnabled {
				return c.compileTry()
			}
		}
	}

	// Unknown statement, return an error
	return c.newError(errors.ErrUnrecognizedStatement, c.t.Peek(0))
}

// isFunctionCall indicates if the token stream points to a function call.
func (c *Compiler) isFunctionCall() bool {
	// Skip through any referencing tokens to see if we find a function
	// invocation.
	pos := 1
	subexpr := 0
	lastWasSymbol := false

	for pos < len(c.t.Tokens) {
		// Are we at the end?
		t := c.t.Peek(pos)
		if t == tokenizer.EndOfTokens {
			return false
		}

		// Part of an object-oriented call?
		if t == "->" {
			return true
		}
		// If this is a paren and there are no
		// pending subexpression tokens, then this
		// is a function calls
		if t == "(" && subexpr == 0 {
			return true
		}

		// Is this a reserved word or delimiter punctuation? IF so we've shot past the statement
		if subexpr == 0 && util.InList(t,
			tokenizer.SemicolonToken,
			"@",
			tokenizer.DataBeginToken,
			tokenizer.AssignToken,
			"+",
			"/",
			"*",
			"-",
			"^",
			"&",
			"|",
			tokenizer.DataBeginToken,
			"==",
			">=",
			"<=",
			"!=",
			"==",
			"=",
			tokenizer.AssertToken,
			tokenizer.BreakToken,
			tokenizer.CallToken,
			tokenizer.ConstToken,
			tokenizer.ContinueToken,
			tokenizer.ExitToken,
			tokenizer.ForToken,
			tokenizer.FuncToken,
			tokenizer.GoToken,
			tokenizer.IfToken,
			tokenizer.ImportToken,
			tokenizer.PackageToken,
			tokenizer.ReturnToken,
			tokenizer.SwitchToken,
			tokenizer.TryToken,
			tokenizer.TypeToken,
			tokenizer.VarToken,
		) {
			return false
		}

		// If it's a symbol, just consume it unless the last token was also a symbol
		if tokenizer.IsSymbol(t) {
			if lastWasSymbol {
				return false
			}

			pos++

			lastWasSymbol = true

			continue
		} else {
			lastWasSymbol = false
		}

		// if it's the end of an array subexpression, decrement
		// the subexpression counter and keep going
		if t == "]" {
			subexpr--
			pos++

			continue
		}

		// If it's the start of an array subexpression, increment
		// the subexpression counter and keep going.
		if t == "[" {
			subexpr++
			pos++

			continue
		}

		// If it's a member dereference, keep on going.
		if t == "." {
			pos++

			continue
		}

		// If we're just in a subexpression, keep consuming tokens.
		if subexpr > 0 {
			pos++

			continue
		}

		// Nope, not a (valid) function invocation
		return false
	}

	return false
}

func (c *Compiler) compilePanic() *errors.EgoError {
	if !c.t.IsNext("(") {
		return errors.New(errors.ErrMissingParenthesis)
	}

	err := c.expressionAtom()
	if !errors.Nil(err) {
		return err
	}

	c.b.Emit(bytecode.Panic)

	if !c.t.IsNext(")") {
		return errors.New(errors.ErrMissingParenthesis)
	}

	return nil
}
