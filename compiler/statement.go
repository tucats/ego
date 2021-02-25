package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// compileStatement compiles a single statement.
func (c *Compiler) compileStatement() *errors.EgoError {
	// We just eat statement separators and empty blocks, and also
	// terminate processing when we hit the end of the token stream
	if c.t.IsNext(";") {
		return nil
	}

	if c.t.IsNext("{}") {
		// Empty body at end of token array means no more atlines...
		if c.t.TokenP < len(c.t.Line) {
			c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])
		}

		return nil
	}

	if c.t.IsNext(tokenizer.EndOfTokens) {
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
	if c.t.IsNext("func") {
		return c.compileFunctionDefinition(false)
	}

	// At this point, we know we're trying to compile a statement,
	// so store the current line number in the stream to help us
	// form runtime error messages as needed.
	c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])

	if c.isFunctionCall() {
		return c.compileFunctionCall()
	}

	// If the next item(s) constitute a value LValue, then this is
	// an assignment statement.
	if c.isAssignmentTarget() {
		return c.compileAssignment()
	}

	// Remaining statement types all have a starting term that defines
	// which compiler unit to call. For each term, call the appropriate
	// handler (which assumes the leading verb has already been consumed)
	switch c.t.Next() {
	case "{":
		return c.compileBlock()

	case "assert":
		if c.extensionsEnabled {
			return c.Assert()
		}

		return c.newError(errors.UnrecognizedStatementError, c.t.Peek(0))

	case "break":
		return c.compileBreak()

	case "call":
		if c.extensionsEnabled {
			return c.compileFunctionCall()
		}

	case "const":
		return c.compileConst()

	case "continue":
		return c.compileContinue()

	case "defer":
		return c.compileDefer()

	case "exit":
		if c.exitEnabled {
			return c.compileExit()
		}

	case "for":
		return c.compileFor()

	case "go":
		return c.compileGo()

	case "if":
		return c.compileIf()

	case "import":
		return c.compileImport()

	case "package":
		return c.compilePackage()

	case "print":
		if c.extensionsEnabled {
			return c.compilePrint()
		}

	case "return":
		return c.compileReturn()

	case "switch":
		return c.compileSwitch()

	case "try":
		if c.extensionsEnabled {
			return c.compileTry()
		}

	case "type":
		return c.compileTypeDefinition()

	case "var":
		return c.compileVar()
	}

	// Unknown statement, return an error
	return c.newError(errors.UnrecognizedStatementError, c.t.Peek(0))
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
			";",
			"@",
			"{",
			",",
			"+",
			"/",
			"*",
			"-",
			"^",
			"&",
			"|",
			"{",
			"==",
			">=",
			"<=",
			"!=",
			"==",
			"=",
			"assert",
			"break",
			"call",
			"const",
			"continue",
			"exit",
			"for",
			"func",
			"go",
			"if",
			"import",
			"package",
			"return",
			"switch",
			"try",
			"type",
			"var",
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
		// th esubexpression counter and keep going.
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
