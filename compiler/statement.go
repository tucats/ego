package compiler

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// DebugMode controls whether the compiler emits full source-line information
// alongside each AtLine instruction. When true, the source text of the line is
// embedded in the bytecode so the debugger and runtime error messages can show
// the original source. In production builds this can be set to false to reduce
// bytecode size.
var DebugMode bool = true

// compileStatement compiles a single statement from the token stream. It is
// called in a loop by Compile() until the token stream is exhausted. The
// function works by peeking at the leading token(s) to figure out which kind
// of statement it is looking at, then delegates to the appropriate sub-compiler
// function.
//
// The broad categories handled here are:
//   - statement separators (semicolons, end-of-tokens) — silently skipped
//   - compiler directives (@line, @test, etc.) — handled by compileDirective
//   - function definitions (func keyword) — handled by compileFunctionDefinition
//   - assignment statements (e.g. x = 1, x := 2) — detected by isAssignmentTarget
//   - function-call statements (e.g. fmt.Println("hi")) — detected by isFunctionCall
//   - keyword-led statements (if, for, return, …) — dispatched by a switch on the verb
//
// Statements that are only legal inside a function body (if, for, return, etc.)
// are guarded by a check on c.functionDepth > 0.
func (c *Compiler) compileStatement() error {
	// Reset the unwrap flag at the start of every statement. An unwrap
	// (the x.(T) type-assertion syntax) is only valid within the same
	// statement as the assignment it applies to.
	c.flags.hasUnwrap = false

	// Silently consume statement separators (semicolons inserted by the
	// tokenizer) and stop when there is nothing left to parse.
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return nil
	}

	// An empty block token ({}) requires special handling because it has
	// no body to compile — just emit the line-number information and move on.
	if c.compileEmptyBlock() {
		return nil
	}

	// Compiler directives start with the "@" character (DirectiveToken).
	// They are meta-instructions to the compiler itself, not runtime code.
	if c.t.IsNext(tokenizer.DirectiveToken) {
		return c.compileDirective()
	}

	c.statementCount = c.statementCount + 1

	// A "func" keyword starts a function definition. Function definitions
	// are compiled into a separate bytecode object and then emitted as a
	// single "push function value / store" pair, so the function body is
	// not executed inline. isLiteralFunction() returns true when the func
	// appears as an expression value (e.g. assigned to a variable) rather
	// than a top-level declaration.
	if c.t.IsNext(tokenizer.FuncToken) {
		return c.compileFunctionDefinition(c.isLiteralFunction())
	}

	// "panic" is now a standard built-in (not an extension). It is always recognized
	// at the statement level so code does not require extensions to be enabled.
	if c.t.IsNext(tokenizer.PanicToken) {
		return c.compilePanic()
	}

	// "recover()" may appear as a statement with the return value discarded.
	// Parse it the same way as an expression but discard the result via Drop.
	if c.t.IsNext(tokenizer.RecoverToken) {
		if !c.t.IsNext(tokenizer.StartOfListToken) {
			return errors.ErrMissingParenthesis
		}

		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return errors.ErrMissingParenthesis
		}

		c.b.Emit(bytecode.Recover)
		c.b.Emit(bytecode.Drop)

		return nil
	}

	// Emit an AtLine instruction so that runtime errors and the debugger
	// can report the correct source line number for the statement we are
	// about to compile.
	if c.t.TokenP < c.t.Len() {
		c.emitLineInfo()
	}

	// A function call used as a statement (e.g. fmt.Println("hi")) is only
	// valid inside a function body. isFunctionCall() looks ahead to confirm
	// the token sequence matches a call pattern before committing.
	if c.functionDepth > 0 && c.isFunctionCall() {
		return c.compileFunctionCall()
	}

	// An assignment (x = 1, x, y := f(), x += 3, x++, etc.) is also only
	// valid inside a function body. isAssignmentTarget() peeks ahead to
	// confirm the pattern before we consume any tokens.
	if c.functionDepth > 0 && c.isAssignmentTarget() {
		return c.compileAssignment()
	}

	// All remaining statement types are introduced by a reserved keyword.
	// Consume the keyword token so the sub-compilers don't need to skip it.
	verb := c.t.Next()

	// These statements are legal both at the top level (outside any function)
	// and inside a function body. They declare things: constants, imports,
	// packages, types, and variables.
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

	// The statements below are only legal inside a function body
	// (functionDepth > 0 means we are inside at least one function).
	if c.functionDepth > 0 {
		switch {
		// An opening brace starts an anonymous nested block with its own scope.
		case verb.Is(tokenizer.BlockBeginToken):
			return c.compileBlock()

		// assert(expr) is an extension that panics when the expression is false.
		case verb.Is(tokenizer.AssertToken):
			if c.flags.extensionsEnabled {
				return c.Assert()
			}

			return c.compileError(errors.ErrUnrecognizedStatement, c.t.Peek(0))

		// break exits the innermost loop.
		case verb.Is(tokenizer.BreakToken):
			return c.compileBreak()

		// call expr() is an extension that discards the return value of a call.
		case verb.Is(tokenizer.CallToken):
			if c.flags.extensionsEnabled {
				return c.compileFunctionCall()
			}

		// continue skips the rest of the current loop iteration.
		case verb.Is(tokenizer.ContinueToken):
			return c.compileContinue()

		// defer schedules a function call to run when the enclosing function returns.
		case verb.Is(tokenizer.DeferToken):
			return c.compileDefer()

		// exit is an extension that terminates the program (like os.Exit).
		case verb.Is(tokenizer.ExitToken):
			if c.flags.exitEnabled {
				return c.compileExit()
			}

		// for begins one of the four loop variants (index, range, conditional, infinite).
		case verb.Is(tokenizer.ForToken):
			return c.compileFor()

		// go starts a goroutine, running a function concurrently.
		case verb.Is(tokenizer.GoToken):
			return c.compileGo()

		// if compiles a conditional branch, optionally with an else clause.
		case verb.Is(tokenizer.IfToken):
			return c.compileIf()

		// print is an extension shorthand for writing to stdout.
		case verb.Is(tokenizer.PrintToken):
			if c.flags.extensionsEnabled {
				return c.compilePrint()
			}

		// return exits the current function, optionally with one or more values.
		case verb.Is(tokenizer.ReturnToken):
			return c.compileReturn()

		// switch compiles a multi-way conditional branch.
		case verb.Is(tokenizer.SwitchToken):
			return c.compileSwitch()

		// try/catch is an extension for structured error handling.
		case verb.Is(tokenizer.TryToken):
			if c.flags.extensionsEnabled {
				return c.compileTry()
			}
		}
	}

	// Nothing matched — the token is not the start of any known statement.
	return c.compileError(errors.ErrUnrecognizedStatement, c.t.Peek(0))
}

// emitLineInfo emits an AtLine bytecode instruction that records the current
// source line number (and, in debug mode, the source text) into the bytecode
// stream. This information is used at runtime to produce human-readable error
// messages and to power the interactive debugger.
//
// In debug mode (or when the debugger is active), the full source line is
// embedded alongside the line number so that the runtime can display it
// verbatim. In release mode only the line number is stored to keep the
// bytecode compact.
func (c *Compiler) emitLineInfo() {
	lineNumber := c.t.CurrentLine() + c.lineNumberOffset

	if DebugMode || c.flags.debuggerActive {
		source := c.t.GetLine(lineNumber)
		c.b.Emit(bytecode.AtLine,
			[]any{
				lineNumber,
				source,
			},
		)
	} else {
		c.b.Emit(bytecode.AtLine, lineNumber)
	}
}

// compileEmptyBlock checks whether the next token is an empty block ({}).
// If it is, a line-number marker is emitted (so that step-debugging still
// lands on the right line) and true is returned so the caller can skip
// further processing. If the token is anything else, false is returned and
// no tokens are consumed.
func (c *Compiler) compileEmptyBlock() bool {
	if c.t.IsNext(tokenizer.EmptyBlockToken) {
		c.emitLineInfo()

		return true
	}

	return false
}

// isFunctionCall returns true when the upcoming tokens look like a function
// call statement rather than an assignment or other statement form. It does
// this by scanning ahead without consuming tokens, looking for the opening
// parenthesis "(" that marks the argument list. The scan bails out early if
// it encounters any token that could not appear between the function name and
// its argument list (reserved words, operators, block braces, etc.).
//
// Because Ego (like Go) allows chained member accesses (a.b.c()) and indexed
// expressions (a[i]()), the scanner tracks open/close bracket nesting via the
// subExpressions counter so that an inner "[" does not fool the lookahead.
func (c *Compiler) isFunctionCall() bool {
	// Walk ahead through the token stream without consuming anything.
	pos := 1
	subExpressions := 0
	lastWasSymbol := false

	for pos < len(c.t.Tokens) {
		// Stop at the end of the token stream.
		t := c.t.Peek(pos)
		if t.Is(tokenizer.EndOfTokens) {
			return false
		}

		// A "(" with no pending bracket nesting means we found the argument
		// list of a call expression — this is a function call statement.
		if t.Is(tokenizer.StartOfListToken) && subExpressions == 0 {
			return true
		}

		// If we see any operator or keyword that cannot appear inside a
		// call-expression prefix, we have gone past the statement boundary
		// without finding a "(" — this is not a function call.
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

		// Two adjacent identifiers without an operator between them cannot
		// be part of a single function-call expression, so bail out.
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

		// "]" closes one level of bracket nesting (e.g. a[i]).
		if t.Is(tokenizer.EndOfArrayToken) {
			subExpressions--
			pos++

			continue
		}

		// "[" opens one level of bracket nesting (e.g. a[i]).
		if t.Is(tokenizer.StartOfArrayToken) {
			subExpressions++
			pos++

			continue
		}

		// "." separates package/struct members (e.g. fmt.Println).
		if t.Is(tokenizer.DotToken) {
			pos++

			continue
		}

		// Inside a bracketed sub-expression, any token is acceptable.
		if subExpressions > 0 {
			pos++

			continue
		}

		// None of the above matched — this is not a function invocation.
		return false
	}

	return false
}
