package compiler

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

const (
	AssertDirective       = "assert"
	AuthentiatedDirective = "authenticated"
	DebugDirective        = "debug"
	EndPointDirective     = "endpoint"
	EntryPointDirective   = "entrypoint"
	ErrorDirective        = "error"
	ExtensionsDirective   = "extensions"
	FileDirective         = "file"
	FailDirective         = "fail"
	GlobalDirective       = "global"
	HandlerDirective      = "handler"
	JSONDirective         = "json"
	LineDirective         = "line"
	LocalizationDirective = "localization"
	LogDirective          = "log"
	PassDirective         = "pass"
	ResponseDirective     = "response"
	RespHeaderDirective   = "respheader"
	SerializeDirective    = "serialize"
	StatusDirective       = "status"
	TemplateDirective     = "template"
	TestDirective         = "test"
	TextDirective         = "text"
	TypeDirective         = "type"
	URLDirective          = "url"
	WaitDirective         = "wait"
)

// compileDirective processes a compiler directive. These become symbols generated
// at compile time that are copied to the compiler's symbol table for processing
// elsewhere.
func (c *Compiler) compileDirective() error {
	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.error(errors.ErrInvalidDirective, name)
	}

	if name.Spelling() != "main" {
		c.b.Emit(bytecode.AtLine, c.t.Line[c.t.TokenP-1])
	}

	switch name.Spelling() {
	case AssertDirective:
		return c.Assert()

	case AuthentiatedDirective:
		return c.authenticatedDirective()

	case DebugDirective:
		ui.Log(ui.InternalLogger, "DEBUG DIRECTIVE")

		return nil

	case EndPointDirective:
		return c.endpointDirective()

	case EntryPointDirective:
		return c.entrypointDirective()

	case ErrorDirective:
		return c.errorDirective()

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

	case PassDirective:
		return c.TestPass()

	case RespHeaderDirective:
		return c.respHeaderDirective()

	case ResponseDirective:
		return c.responseDirective()

	case SerializeDirective:
		return c.serializeDirective()

	case StatusDirective:
		return c.statusDirective()

	case TemplateDirective:
		return c.templateDirective()

	case TestDirective:
		return c.testDirective()

	case TextDirective:
		return c.textDirective()

	case TypeDirective:
		return c.typeDirective()

	case URLDirective:
		return c.urlDirective()

	case WaitDirective:
		return c.waitDirective()

	default:
		return c.error(errors.ErrInvalidDirective, name)
	}
}
func (c *Compiler) serializeDirective() error {
	name := c.t.Next().Spelling()
	c.b.Emit(bytecode.Load, name)
	c.b.Emit(bytecode.Serialize)
	c.b.Emit(bytecode.Print, 1)

	return nil
}

// Identify the endpoint for this service module, if it is other
// than the default provided by the service file directory path.
func (c *Compiler) endpointDirective() error {
	endpoint := c.t.Next()
	if !endpoint.IsString() {
		return c.error(errors.ErrInvalidEndPointString)
	}

	// We do no work here, this text is processed during server
	// initialization only.
	return nil
}

// Generate the call to the main program, and the the exit code.
func (c *Compiler) entrypointDirective() error {
	mainName := c.t.Next()
	if mainName == tokenizer.EndOfTokens || mainName == tokenizer.SemicolonToken {
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
	handlerName := c.t.Next()
	if handlerName == tokenizer.EndOfTokens || handlerName == tokenizer.SemicolonToken {
		handlerName = tokenizer.NewIdentifierToken("handler")
	}

	if !handlerName.IsIdentifier() {
		return c.error(errors.ErrInvalidIdentifier)
	}

	// Determine if we are in "real" http mode. This is true if there is an
	// import of "http" and/or use of the @entrypoint directive at the start
	// of the service file.
	httpMode := false
	if c.t.Tokens[0] == tokenizer.ImportToken || util.InList(c.t.Tokens[1].Spelling(), "http", "\"http\"") {
		httpMode = true
	}

	if c.t.Tokens[0] == tokenizer.DirectiveToken || c.t.Tokens[1].Spelling() == EntryPointDirective {
		httpMode = true
	}

	ui.Log(ui.ByteCodeLogger, "@handler invocation uses real http mode: %v", httpMode)

	stackMarker := bytecode.NewStackMarker("handler")

	// Plant a stack marker and load the handler function value
	c.b.Emit(bytecode.Push, stackMarker)
	c.b.Emit(bytecode.Load, handlerName)

	// Generate a new request and put it on the stack
	if httpMode {
		c.b.Emit(bytecode.LoadThis, "http")
		c.b.Emit(bytecode.Member, "NewRequest")
	} else {
		c.b.Emit(bytecode.Load, "NewRequest")
	}

	c.b.Emit(bytecode.Call, 0)

	// Generate a new response and put it on the stack.
	if httpMode {
		c.b.Emit(bytecode.LoadThis, "http")
		c.b.Emit(bytecode.Member, "NewResponse")
	} else {
		c.b.Emit(bytecode.Load, "NewResponse")
	}

	c.b.Emit(bytecode.Call, 0)

	// Make a copy of the response and store as $response. We intentionally
	// create a new symbol table scope to hold this value, so that it is
	// not visible to the handler function or any shared cached version of
	// the same code. This scope is never removed (it isolates the execution
	// of this instance of the service) and will be discarded when the
	// service exits.
	c.b.Emit(bytecode.Dup)
	c.b.Emit(bytecode.PushScope)
	c.b.Emit(bytecode.StoreAlways, defs.RestStructureName)

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
	if c.t.AtEnd() {
		return c.error(errors.ErrInvalidSymbolName)
	}

	name := c.t.Next()
	if strings.HasPrefix(name.Spelling(), defs.ReadonlyVariablePrefix) || !name.IsIdentifier() {
		return c.error(errors.ErrInvalidSymbolName, name)
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

	return nil
}

// Parse the @json directive.
func (c *Compiler) jsonDirective() error {
	_ = c.modeCheck("server")
	c.b.Emit(bytecode.Load, "_json")

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

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchTrue, 0)

	if err := c.compileStatement(); err != nil {
		return err
	}

	return c.b.SetAddressHere(branch)
}

func (c *Compiler) lineDirective() error {
	// The next token must be an integer value
	lineNumberToken := c.t.Next()
	if !lineNumberToken.IsValue() {
		return c.error(errors.ErrInvalidInteger).Context(lineNumberToken)
	}

	// Extract the value from the token and store it as the current line number
	// in the tokenizer. Also generate a bytecode to store this data at runtime.
	line := int(lineNumberToken.Integer())

	c.b.ClearLineNumbers()
	_ = c.t.SetLineNumber(line)
	c.b.Emit(bytecode.AtLine, line)

	return nil
}

// logDirective parses the @log directive.
func (c *Compiler) logDirective() error {
	if c.t.AtEnd() {
		return c.error(errors.ErrInvalidSymbolName)
	}

	next := c.t.Next()
	if !next.IsIdentifier() {
		return c.error(errors.ErrInvalidSymbolName, next)
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

// statusDirective parses the @status directive which sets a symbol
// value in the root symbol table with the REST call status value.
func (c *Compiler) statusDirective() error {
	if c.t.AtEnd() {
		return c.error(errors.ErrInvalidSymbolName)
	}

	_ = c.modeCheck("server")

	if c.t.AtEnd() {
		c.b.Emit(bytecode.Push, http.StatusOK)
	} else {
		if err := c.emitExpression(); err != nil {
			return err
		}
	}

	c.b.Emit(bytecode.StoreGlobal, defs.RestStatusVariable)

	return nil
}

func (c *Compiler) authenticatedDirective() error {
	var token string

	_ = c.modeCheck("server")

	if c.t.AtEnd() {
		token = defs.Any
	} else {
		token = c.t.NextText()
	}

	if !util.InList(token,
		defs.UserAuthenticationRequired,
		defs.AdminAuthneticationRequired,
		defs.Any,
		defs.TokenRequired,
		defs.AdminTokenRequired,
	) {
		return c.error(errors.ErrInvalidAuthenticationType, token)
	}

	c.b.Emit(bytecode.Auth, token)

	return nil
}

// respHEaderDirective processes the @response directive.
func (c *Compiler) respHeaderDirective() error {
	if c.t.AtEnd() {
		return c.error(errors.ErrInvalidSymbolName)
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

// responseDirective processes the @response directive.
func (c *Compiler) responseDirective() error {
	if c.t.AtEnd() {
		return c.error(errors.ErrInvalidSymbolName)
	}

	_ = c.modeCheck("server")

	if err := c.emitExpression(); err != nil {
		return err
	}

	c.b.Emit(bytecode.Response)

	return nil
}

// templateDirective implements the template compiler directive.
func (c *Compiler) templateDirective() error {
	// Get the template name
	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.error(errors.ErrInvalidSymbolName, name)
	}

	nameSpelling := c.normalize(name.Spelling())

	// Get the template string definition
	if err := c.emitExpression(); err != nil {
		return err
	}

	c.b.Emit(bytecode.Template, nameSpelling)
	c.b.Emit(bytecode.SymbolCreate, nameSpelling)
	c.b.Emit(bytecode.Store, nameSpelling)

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
	} else if c.t.IsNext(tokenizer.SemicolonToken) {
		c.t.Advance(-1)

		extensions = true
	} else {
		return c.error(errors.ErrInvalidBooleanValue)
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
		err = c.error(errors.ErrInvalidTypeCheck, t)
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
	if err := c.parseStruct(); err != nil {
		return err
	}

	c.b.Emit(bytecode.StoreGlobal, defs.LocalizationVariable)

	return nil
}
