package compiler

import (
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

var DebugMode bool = true

// compileStatement compiles a single statement.
func (c *Compiler) compileStatement() error {
	// Start every statement with an initialized flag set.
	c.flags.hasUnwrap = false

	// We just eat statement separators and empty blocks, and also
	// terminate processing when we hit the end of the token stream
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return nil
	}

	// Check for the possibility of an empty block.
	if c.compileEmptyBlock() {
		return nil
	}

	// Is it a directive token? These really just store data in the compiler
	// symbol table that is used to extend features. These symbols end up in
	// the runtime context of the running code
	if c.t.IsNext(tokenizer.DirectiveToken) {
		return c.compileDirective()
	}

	c.statementCount = c.statementCount + 1

	// Is it a function definition? These aren't compiled inline,
	// so we call a special compile unit that will compile the
	// function and store it in the bytecode symbol table.
	if c.t.IsNext(tokenizer.FuncToken) {
		return c.compileFunctionDefinition(c.isLiteralFunction())
	}

	// Is it a "panic" statement when extensions are enabled?
	if c.t.IsNext(tokenizer.PanicToken) && settings.GetBool(defs.ExtensionsEnabledSetting) {
		return c.compilePanic()
	}

	// At this point, we know we're trying to compile a statement,
	// so store the current line number in the stream to help us
	// form runtime error messages as needed.
	if c.t.TokenP < c.t.Len() {
		c.emitLineInfo()
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

	// First, check for the statements that can appear outside or inside
	// a block.
	switch {
	case verb.Is(tokenizer.ConstToken):
		return c.compileConst()

	case verb.Is(tokenizer.ImportToken):
		return c.compileImport()

	case verb.Is(tokenizer.PackageToken):
		return c.compilePackage()

	case verb.Is(tokenizer.TypeToken):
		return c.compileTypeDefinition()

	case verb.Is(tokenizer.VarToken):
		return c.compileVar()
	}

	// If we are in the body of a function, the rest of these are
	// also valid.
	if c.functionDepth > 0 {
		switch {
		case verb.Is(tokenizer.BlockBeginToken):
			return c.compileBlock()

		case verb.Is(tokenizer.AssertToken):
			if c.flags.extensionsEnabled {
				return c.Assert()
			}

			return c.compileError(errors.ErrUnrecognizedStatement, c.t.Peek(0))

		case verb.Is(tokenizer.BreakToken):
			return c.compileBreak()

		case verb.Is(tokenizer.CallToken):
			if c.flags.extensionsEnabled {
				return c.compileFunctionCall()
			}

		case verb.Is(tokenizer.ContinueToken):
			return c.compileContinue()

		case verb.Is(tokenizer.DeferToken):
			return c.compileDefer()

		case verb.Is(tokenizer.ExitToken):
			if c.flags.exitEnabled {
				return c.compileExit()
			}

		case verb.Is(tokenizer.ForToken):
			return c.compileFor()

		case verb.Is(tokenizer.GoToken):
			return c.compileGo()

		case verb.Is(tokenizer.IfToken):
			return c.compileIf()

		case verb.Is(tokenizer.PrintToken):
			if c.flags.extensionsEnabled {
				return c.compilePrint()
			}

		case verb.Is(tokenizer.ReturnToken):
			return c.compileReturn()

		case verb.Is(tokenizer.SwitchToken):
			return c.compileSwitch()

		case verb.Is(tokenizer.TryToken):
			if c.flags.extensionsEnabled {
				return c.compileTry()
			}
		}
	}

	// Unknown statement, return an error
	return c.compileError(errors.ErrUnrecognizedStatement, c.t.Peek(0))
}

func (c *Compiler) emitLineInfo() {
	lineNumber := c.t.CurrentLine() + c.lineNumberOffset

	if DebugMode || c.flags.debuggerActive {
		source := c.t.GetLine(lineNumber)
		c.b.Emit(bytecode.AtLine,
			[]interface{}{
				lineNumber,
				source,
			},
		)
	} else {
		c.b.Emit(bytecode.AtLine, lineNumber)
	}
}

// Check to see if the next item is an empty block. If so, check to see
// how to handle the line number and report it's been handled.
func (c *Compiler) compileEmptyBlock() bool {
	if c.t.IsNext(tokenizer.EmptyBlockToken) {
		c.emitLineInfo()

		return true
	}

	return false
}

// isFunctionCall indicates if the token stream points to a function call.
func (c *Compiler) isFunctionCall() bool {
	// Skip through any referencing tokens to see if we find a function
	// invocation.
	pos := 1
	subExpressions := 0
	lastWasSymbol := false

	for pos < len(c.t.Tokens) {
		// Are we at the end?
		t := c.t.Peek(pos)
		if t.Is(tokenizer.EndOfTokens) {
			return false
		}

		// If this is a paren and there are no
		// pending sub-expression tokens, then this
		// is a function calls
		if t.Is(tokenizer.StartOfListToken) && subExpressions == 0 {
			return true
		}

		// Is this a reserved word or delimiter punctuation? If so we've shot past the statement
		if subExpressions == 0 && tokenizer.InList(t,
			tokenizer.SemicolonToken,
			tokenizer.DirectiveToken,
			tokenizer.DataBeginToken,
			tokenizer.DefineToken,
			tokenizer.AddToken,
			tokenizer.DivideToken,
			tokenizer.MultiplyToken,
			tokenizer.SubtractToken,
			tokenizer.ExponentToken,
			tokenizer.AndToken,
			tokenizer.OrToken,
			tokenizer.DataBeginToken,
			tokenizer.EqualsToken,
			tokenizer.GreaterThanToken,
			tokenizer.GreaterThanOrEqualsToken,
			tokenizer.LessThanToken,
			tokenizer.LessThanOrEqualsToken,
			tokenizer.NotEqualsToken,
			tokenizer.EqualsToken,
			tokenizer.AssignToken,
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
		if t.IsIdentifier() {
			if lastWasSymbol {
				return false
			}

			lastWasSymbol = true

			pos++

			continue
		} else {
			lastWasSymbol = false
		}

		// if it's the end of an array sub-expression, decrement
		// the sub-expression counter and keep going
		if t.Is(tokenizer.EndOfArrayToken) {
			subExpressions--
			pos++

			continue
		}

		// If it's the start of an array sub-expression, increment
		// the sub-expression counter and keep going.
		if t.Is(tokenizer.StartOfArrayToken) {
			subExpressions++
			pos++

			continue
		}

		// If it's a member dereference, keep on going.
		if t.Is(tokenizer.DotToken) {
			pos++

			continue
		}

		// If we're just in a sub-expression, keep consuming tokens.
		if subExpressions > 0 {
			pos++

			continue
		}

		// Nope, not a (valid) function invocation
		return false
	}

	return false
}
