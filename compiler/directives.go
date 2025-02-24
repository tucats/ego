package compiler

import (
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

const (
	AssertDirective       = "assert"
	AuthentiatedDirective = "authenticated"
	DebugDirective        = "debug"
	DefineDirective       = "define"
	EndPointDirective     = "endpoint"
	EntryPointDirective   = "entrypoint"
	ErrorDirective        = "error"
	ErrorsDirective       = "dump_errors"
	ExtensionsDirective   = "extensions"
	FileDirective         = "file"
	FailDirective         = "fail"
	GlobalDirective       = "global"
	HandlerDirective      = "handler"
	JSONDirective         = "json"
	LineDirective         = "line"
	LocalizationDirective = "localization"
	LogDirective          = "log"
	PackagesDirective     = "packages"
	PassDirective         = "pass"
	ProfileDirective      = "profile"
	SerializeDirective    = "serialize"
	StatusDirective       = "status"
	SymbolsDirective      = "symbols"
	TemplateDirective     = "template"
	TestDirective         = "test"
	TextDirective         = "text"
	TypeDirective         = "type"
	WaitDirective         = "wait"
)

// compileDirective processes a compiler directive. These either take immediate action
// to modify the active compiler, or generate stateless code into the active bytecode.
func (c *Compiler) compileDirective() error {
	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.compileError(errors.ErrInvalidDirective, name)
	}

	if name.Spelling() != defs.Main {
		line, _ := name.Location()
		c.b.Emit(bytecode.AtLine, line)
	}

	switch name.Spelling() {
	case AssertDirective:
		return c.Assert()

	case AuthentiatedDirective:
		return c.authenticatedDirective()

	case DebugDirective:
		ui.Log(ui.InternalLogger, "runtime.debug.directive", nil)

		return nil

	case DefineDirective:
		return c.defineDirective()

	case EndPointDirective:
		return c.endpointDirective()

	case EntryPointDirective:
		return c.entrypointDirective()

	case ErrorDirective:
		return c.errorDirective()

	case ErrorsDirective:
		return i18n.DumpClass("error.")

	case ExtensionsDirective:
		return c.extensionsDirective()

	case FailDirective:
		return c.Fail()

	case FileDirective:
		return c.File()

	case GlobalDirective:
		return c.globalDirective()

	case HandlerDirective:
		return c.handlerDirective()

	case JSONDirective:
		return c.jsonDirective()

	case LineDirective:
		return c.lineDirective()

	case LocalizationDirective:
		return c.localizationDirective()

	case LogDirective:
		return c.logDirective()

	case PackagesDirective:
		return c.packagesDirective()

	case PassDirective:
		return c.TestPass()

	case ProfileDirective:
		return c.profileDirective()

	case SerializeDirective:
		return c.serializeDirective()

	case SymbolsDirective:
		return c.symbolsDirective()

	case TemplateDirective:
		return c.templateDirective()

	case TestDirective:
		return c.testDirective()

	case TextDirective:
		return c.textDirective()

	case TypeDirective:
		return c.typeDirective()

	case WaitDirective:
		return c.waitDirective()

	default:
		return c.compileError(errors.ErrInvalidDirective, name)
	}
}

// Compile the @symbols directive. This directive is optionally followed
// by an expression that is used to create a label for the symbol dump
// in the output.
func (c *Compiler) symbolsDirective() error {
	flag := false

	if c.t.Peek(1).IsNot(tokenizer.SemicolonToken) {
		if e, err := c.Expression(true); err == nil {
			c.b.Append(e)
		} else {
			return err
		}

		flag = true
	}

	c.b.Emit(bytecode.DumpSymbols, flag)

	return nil
}
func (c *Compiler) serializeDirective() error {
	var (
		targetCode *bytecode.ByteCode
		err        error
	)

	if c.t.EndofStatement() {
		return c.compileError(errors.ErrMissingExpression)
	}

	// Is this an assignment?
	tokenPosition := c.t.Mark()
	bcPosition := c.b.Mark()

	if c.t.Peek(1).IsIdentifier() {
		targetCode, err = c.assignmentTarget()
		operation := c.t.Peek(1)

		if err == nil && (operation.IsNot(tokenizer.AssignToken) && operation.IsNot(tokenizer.DefineToken)) {
			targetCode = nil

			c.t.Set(tokenPosition)
			c.b.Delete(bcPosition) // There is a stray marker push we need to remove.
		} else {
			c.t.Advance(1)
		}
	}

	// Get the expression of the item to serialize
	expr, err := c.Expression(true)
	if err != nil {
		return err
	}

	// Emit the expression describing the bytecode function to serialize,
	// followed by the serialization operator and a print operation.
	c.b.Append(expr)
	c.b.Emit(bytecode.Serialize)

	if targetCode != nil {
		c.b.Append(targetCode)
	} else {
		c.b.Emit(bytecode.Print, 1)
		c.b.Emit(bytecode.Newline)
	}

	return nil
}

// Identify the endpoint for this service module, if it is other
// than the default provided by the service file directory path.
func (c *Compiler) endpointDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidEndPointString)
	}

	endpoint := c.t.Next()
	if !endpoint.IsString() {
		return c.compileError(errors.ErrInvalidEndPointString)
	}

	// We do no work here, this text is processed during server
	// initialization only.
	return nil
}

// Generate the call to the main program, and the exit code.
func (c *Compiler) entrypointDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrMissingFunctionName)
	}

	mainName := c.t.Next()
	if mainName.Is(tokenizer.EndOfTokens) || mainName.Is(tokenizer.SemicolonToken) {
		mainName = tokenizer.NewIdentifierToken(defs.Main)
	}

	c.b.Emit(bytecode.Push, mainName)
	c.b.Emit(bytecode.Dup)
	c.b.Emit(bytecode.StoreAlways, defs.MainVariable)
	c.b.Emit(bytecode.EntryPoint)
	c.b.Emit(bytecode.AtLine, -1)
	c.b.Emit(bytecode.Push, 0)
	c.b.Emit(bytecode.Load, "os")
	c.b.Emit(bytecode.Member, "Exit")
	c.b.Emit(bytecode.Push, 0)
	c.b.Emit(bytecode.Call, 1)

	return nil
}

func (c *Compiler) handlerDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrMissingSymbol)
	}

	handlerName := c.t.Next()
	if handlerName.Is(tokenizer.EndOfTokens) || handlerName.Is(tokenizer.SemicolonToken) {
		handlerName = tokenizer.NewIdentifierToken("handler")
	}

	if !handlerName.IsIdentifier() {
		return c.compileError(errors.ErrInvalidIdentifier)
	}

	stackMarker := bytecode.NewStackMarker("handler")

	// Plant a stack marker and load the handler function value
	c.b.Emit(bytecode.Push, stackMarker)
	c.b.Emit(bytecode.Load, handlerName)

	// Generate a new request and put it on the stack
	c.b.Emit(bytecode.Load, "_request")

	// Generate a new response and put it on the stack.
	c.b.Emit(bytecode.Load, "_responseWriter")

	// Call the handler with the request and response
	c.b.Emit(bytecode.Call, 2)

	// Drop the marker and we're done. The _response variable
	// should contain the product of the handler.
	c.b.Emit(bytecode.DropToMarker, stackMarker)
	c.b.Emit(bytecode.Stop)

	return nil
}

// globalDirective parses the @global directive which sets a symbol
// value in the root symbol table, global to all execution.
func (c *Compiler) globalDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidSymbolName)
	}

	name := c.t.Next()
	if strings.HasPrefix(name.Spelling(), defs.ReadonlyVariablePrefix) || !name.IsIdentifier() {
		return c.compileError(errors.ErrInvalidSymbolName, name)
	}

	symbolName := c.normalize(name.Spelling())

	if c.t.AtEnd() {
		c.b.Emit(bytecode.Push, "")
	} else {
		if err := c.emitExpression(); err != nil {
			return err
		}
	}

	c.b.Emit(bytecode.StoreGlobal, symbolName)
	c.DefineGlobalSymbol(symbolName)

	return nil
}

// Parse the @json directive.
func (c *Compiler) jsonDirective() error {
	_ = c.modeCheck("server")
	c.b.Emit(bytecode.Load, "_json")

	if c.t.EndofStatement() {
		return c.compileError(errors.ErrMissingStatement)
	}

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchFalse, 0)

	if err := c.compileStatement(); err != nil {
		return err
	}

	return c.b.SetAddressHere(branch)
}

// Parse the @text directive.
func (c *Compiler) textDirective() error {
	_ = c.modeCheck("server")
	c.b.Emit(bytecode.Load, "_json")

	if c.t.EndofStatement() {
		return c.compileError(errors.ErrMissingStatement)
	}

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchTrue, 0)

	if err := c.compileStatement(); err != nil {
		return err
	}

	return c.b.SetAddressHere(branch)
}

func (c *Compiler) lineDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidInteger)
	}

	// The next token must be an integer value
	lineNumberToken := c.t.Next()
	if !lineNumberToken.IsValue() {
		return c.compileError(errors.ErrInvalidInteger).Context(lineNumberToken)
	}

	// If the debugger is active, we ignore this directive
	if c.flags.debuggerActive {
		return nil
	}

	// Extract the value from the token and store it as the current line number
	// in the tokenizer. Also generate a bytecode to store this data at runtime.
	line := int(lineNumberToken.Integer())

	c.b.ClearLineNumbers()
	c.b.Emit(bytecode.AtLine, line)

	// If @line 1, no offset. Additionall, take off one for the @line directive itself.
	c.lineNumberOffset = line - 2

	return nil
}

func (c *Compiler) defineDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidInteger)
	}

	// Accept a list of identifiers and define them as "used" symbol names
	// so they don't throw an error during compilation if not referenced later.
	for !c.t.EndofStatement() {
		// The next token must be an identifier
		name := c.t.Next()

		if !name.IsIdentifier() {
			return c.compileError(errors.ErrInvalidIdentifier).Context(name)
		}

		c.DefineGlobalSymbol(name.Spelling())

		c.t.IsNext(tokenizer.CommaToken)
	}

	return nil
}

// profileDirective parses the @profile directive.
func (c *Compiler) profileDirective() error {
	// Next token must be the command verb.
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidProfileAction)
	}

	verb := c.t.Next()
	if !verb.IsIdentifier() {
		return c.compileError(errors.ErrInvalidIdentifier).Context(verb)
	}

	command := strings.ToLower(verb.Spelling())
	if !util.InList(command, "start", "enable", "on", "stop", "disable", "off", "report", "dump", "print") {
		return c.compileError(errors.ErrInvalidProfileAction, verb).Context(verb)
	}

	c.b.Emit(bytecode.Profile, command)

	return nil
}

// logDirective parses the @log directive.
func (c *Compiler) logDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidSymbolName)
	}

	next := c.t.Next()
	if !next.IsIdentifier() {
		return c.compileError(errors.ErrInvalidSymbolName, next)
	}

	if c.t.AtEnd() {
		c.b.Emit(bytecode.Push, "")
	} else {
		if err := c.emitExpression(); err != nil {
			return err
		}
	}

	c.b.Emit(bytecode.Log, c.normalize(next.Spelling()))

	return nil
}

func (c *Compiler) authenticatedDirective() error {
	var token string

	_ = c.modeCheck("server")

	if c.t.EndofStatement() {
		token = defs.Any
	} else {
		token = c.t.NextText()
	}

	if !util.InList(token,
		defs.UserAuthenticationRequired,
		defs.AdminAuthenticationRequired,
		defs.NoAuthenticationRequired,
	) {
		return c.compileError(errors.ErrInvalidAuthenticationType, token)
	}

	return nil
}

// respHEaderDirective processes the @response directive.
func (c *Compiler) respHeaderDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidSymbolName)
	}

	_ = c.modeCheck("server")

	// Parse the header name expression and emit the code.
	if err := c.emitExpression(); err != nil {
		return err
	}

	// Parse the header value expression and emit the code.
	if err := c.emitExpression(); err != nil {
		return err
	}

	c.b.Emit(bytecode.RespHeader)

	return nil
}

// templateDirective implements the template compiler directive.
func (c *Compiler) templateDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidSymbolName)
	}

	// Get the template name
	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.compileError(errors.ErrInvalidSymbolName, name)
	}

	nameSpelling := c.normalize(name.Spelling())

	// Get the template string definition
	if err := c.emitExpression(); err != nil {
		return err
	}

	c.b.Emit(bytecode.Template, nameSpelling)
	c.b.Emit(bytecode.SymbolCreate, nameSpelling)
	c.b.Emit(bytecode.Store, nameSpelling)

	c.DefineGlobalSymbol(nameSpelling)

	return nil
}

// errorDirective implements the @error directive.
func (c *Compiler) errorDirective() error {
	c.b.Emit(bytecode.Push, bytecode.NewStackMarker("call"))
	c.b.Emit(bytecode.Load, "error")

	if !c.atStatementEnd() {
		if err := c.emitExpression(); err != nil {
			return err
		}
	} else {
		c.b.Emit(bytecode.Push, errors.ErrPanic)
	}

	c.b.Emit(bytecode.Call, 1) // Does not cause fatal error
	c.b.Emit(bytecode.DropToMarker)

	return nil
}

func (c *Compiler) extensionsDirective() error {
	var extensions bool

	if c.t.IsNext(tokenizer.NewIdentifierToken("default")) {
		extensions = settings.GetBool(defs.ExtensionsEnabledSetting)
	} else if c.t.IsNext(tokenizer.TrueToken) {
		extensions = true
	} else if c.t.IsNext(tokenizer.FalseToken) {
		extensions = false
	} else if c.t.EndofStatement() {
		extensions = true
	} else {
		return c.compileError(errors.ErrInvalidBooleanValue)
	}

	c.b.Emit(bytecode.Push, extensions)
	c.b.Emit(bytecode.StoreGlobal, defs.ExtensionsVariable)

	c.SetExtensionsEnabled(extensions)
	symbols.RootSymbolTable.SetAlways(defs.ExtensionsVariable, extensions)

	return nil
}

// typeDirective implements the @type directive which must be followed by the
// keyword "strict" or "dynamic", indicating the type of type checking.
func (c *Compiler) typeDirective() error {
	var err error

	if c.t.EndofStatement() {
		return c.compileError(errors.ErrInvalidTypeCheck)
	}

	if t := c.t.NextText(); util.InList(t, defs.Strict, defs.Relaxed, defs.Dynamic) {
		value := 0

		switch strings.ToLower(t) {
		case defs.Strict:
			value = 0

		case defs.Relaxed:
			value = 1

		case defs.Dynamic:
			value = 2
		}

		c.b.Emit(bytecode.Push, value)
	} else {
		err = c.compileError(errors.ErrInvalidTypeCheck, t)
	}

	c.b.Emit(bytecode.StaticTyping)

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// atStatementEnd checks the next token in the stream to see if it indicates
// that we have parsed all of the statement.
func (c *Compiler) atStatementEnd() bool {
	return tokenizer.InList(c.t.Peek(1), tokenizer.SemicolonToken, tokenizer.BlockBeginToken, tokenizer.BlockEndToken, tokenizer.EndOfTokens)
}

// modeCheck emits the code to verify that we are running
// in the given mode. If check is true, we require that we
// are in the given mode. If check is false, we require that
// we are not in the given mode.
func (c *Compiler) modeCheck(mode string) error {
	c.b.Emit(bytecode.ModeCheck, mode)

	return nil
}

// Implement the @wait directive which waits for any outstanding
// go routines to finish.
func (c *Compiler) waitDirective() error {
	c.b.Emit(bytecode.Wait)

	return nil
}

func (c *Compiler) localizationDirective() error {
	if c.t.EndofStatement() {
		return c.compileError(errors.ErrMissingExpression)
	}

	if err := c.parseStruct(true); err != nil {
		return err
	}

	c.b.Emit(bytecode.StoreGlobal, defs.LocalizationVariable)

	return nil
}

func (c *Compiler) packagesDirective() error {
	if c.t.EndofStatement() {
		c.b.Emit(bytecode.DumpPackages)

		return nil
	}

	names := make([]interface{}, 0, 5)

	for {
		if c.t.EndofStatement() {
			break
		}

		name := c.t.Next()
		if !name.IsIdentifier() {
			return c.compileError(errors.ErrInvalidPackageName).Context(name)
		}

		names = append(names, c.normalize(name.Spelling()))

		c.t.IsNext(tokenizer.CommaToken)
	}

	c.b.Emit(bytecode.DumpPackages, data.NewList(names...))

	return nil
}
