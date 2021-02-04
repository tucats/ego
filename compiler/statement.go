package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Predefined names used by statement processing.
const (
	DirectiveStructureName = "_directives"
)

// Statement compiles a single statement.
func (c *Compiler) Statement() error {
	// We just eat statement separators and empty blocks, and also
	// terminate processing when we hit the end of the token stream
	if c.t.IsNext(";") {
		return nil
	}

	if c.t.IsNext("{}") {
		c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])

		return nil
	}

	if c.t.IsNext(tokenizer.EndOfTokens) {
		return nil
	}

	// Is it a directive token? These really just store data in the compiler
	// symbol table that is used to extend features. These symbols end up in
	// the runtime context of the running code
	if c.t.IsNext("@") {
		return c.Directive()
	}

	c.statementCount = c.statementCount + 1

	// Is it a function definition? These aren't compiled inline,
	// so we call a special compile unit that will compile the
	// function and store it in the bytecode symbol table.
	if c.t.IsNext("func") {
		return c.Function(false)
	}

	// At this point, we know we're trying to compile a statement,
	// so store the current line number in the stream to help us
	// form runtime error messages as needed.
	c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP])

	if c.IsFunctionCall() {
		return c.Call()
	}

	// If the next item(s) constitute a value LValue, then this is
	// an assignment statement.
	if c.IsLValue() {
		return c.Assignment()
	}

	// Remaining statement types all have a starting term that defines
	// which compiler unit to call. For each term, call the appropriate
	// handler (which assumes the leading verb has already been consumed)
	switch c.t.Next() {
	case "{":
		return c.Block()

	case "array":
		if c.extensionsEnabled {
			return c.Array()
		}

		return c.NewError(UnrecognizedStatementError, c.t.Peek(0))

	case "assert":
		if c.extensionsEnabled {
			return c.Assert()
		}

		return c.NewError(UnrecognizedStatementError, c.t.Peek(0))

	case "break":
		return c.Break()

	case "call":
		if c.extensionsEnabled {
			return c.Call()
		}

	case "const":
		return c.Constant()

	case "continue":
		return c.Continue()

	case "defer":
		return c.Defer()

	case "exit":
		if c.exitEnabled {
			return c.Exit()
		}

	case "for":
		return c.For()

	case "go":
		return c.Go()

	case "if":
		return c.If()

	case "import":
		return c.Import()

	case "package":
		return c.Package()

	case "print":
		if c.extensionsEnabled {
			return c.Print()
		}

	case "return":
		return c.Return()

	case "switch":
		return c.Switch()

	case "try":
		if c.extensionsEnabled {
			return c.Try()
		}

	case "type":
		return c.Type()

	case "var":
		return c.Var()
	}

	// Unknown statement, return an error
	return c.NewError(UnrecognizedStatementError, c.t.Peek(0))
}

// IsFunctionCall indicates if the token stream points to a function call.
func (c *Compiler) IsFunctionCall() bool {
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
			"array",
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
