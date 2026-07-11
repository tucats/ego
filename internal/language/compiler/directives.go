package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/packages"
	"github.com/tucats/ego/internal/util"
	"github.com/tucats/ego/internal/util/validate"
)

const (
	AssertDirective        = "assert"
	AuthenticatedDirective = "authenticated"
	CaptureDirective       = "capture"
	CompileDirective       = "compile"
	DebugDirective         = "debug"
	DefineDirective        = "define"
	EndPointDirective      = "endpoint"
	EntryPointDirective    = "entrypoint"
	ErrorDirective         = "error"
	ErrorsDirective        = "dump_errors"
	ExtensionsDirective    = "extensions"
	FileDirective          = "file"
	FailDirective          = "fail"
	GlobalDirective        = "global"
	HandlerDirective       = "handler"
	JSONDirective          = "json"
	LineDirective          = "line"
	LocalizationDirective  = "localization"
	LogDirective           = "log"
	OptimizerDirective     = "optimizer"
	PackageDirective       = "package"
	PackagesDirective      = "packages"
	PassDirective          = "pass"
	ProfileDirective       = "profile"
	StatusDirective        = "status"
	SymbolsDirective       = "symbols"
	TemplateDirective      = "template"
	TestDirective          = "test"
	TextDirective          = "text"
	TypeDirective          = "type"
	ValidationDirective    = "validation"
	WaitDirective          = "wait"
)

// compileDirective processes a compiler directive — a statement that starts with "@".
// The "@" token has already been consumed; this function reads the directive name and
// dispatches to the appropriate handler.
//
// Directives fall into two categories:
//   - Compile-time actions that modify the compiler state immediately (e.g. @line,
//     @extensions, @define, @type).
//   - Code-generation directives that emit special bytecode (e.g. @global, @log,
//     @json, @handler, @entrypoint).
//
// Unrecognized directive names are tried as user-defined compile-time macros (via
// compilerMacro). If no macro matches, ErrInvalidDirective is returned.
func (c *Compiler) compileDirective() error {
	name := c.t.Next()
	if !name.IsIdentifier() && (name.Spelling() != PackageDirective) {
		return c.compileError(errors.ErrInvalidDirective, name)
	}

	if name.Spelling() != defs.Main {
		line, _ := name.Location()
		c.b.Emit(bytecode.AtLine, line)
	}

	switch name.Spelling() {
	case AssertDirective:
		return c.Assert()

	case AuthenticatedDirective:
		return c.authenticatedDirective()

	case CaptureDirective:
		return c.captureDirective()

	case CompileDirective:
		return c.compileBlockDirective()

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

	case OptimizerDirective:
		return c.optimizerDirective()

	case PackageDirective:
		return c.packagesDirective(true)

	case PackagesDirective:
		return c.packagesDirective(false)

	case PassDirective:
		return c.TestPass()

	case ProfileDirective:
		return c.profileDirective()

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

	case ValidationDirective:
		return c.validationDirective()

	case WaitDirective:
		return c.waitDirective()

	default:
		return c.compilerMacro(name.Spelling(), false)
	}
}

func (c *Compiler) validationDirective() error {
	var err error

	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrMissingStatement)
	}

	verb := c.t.Next()

	switch verb.Spelling() {
	case "dump":
		b, err := validate.EncodeDictionary()
		if err == nil {
			fmt.Printf("%s\n", string(b))
		}

	default:
		return c.compileError(errors.ErrInvalidDirective, verb.Spelling())
	}

	return err
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

// Identify the endpoint for this service module, if it is other
// than the default provided by the service file directory path.
func (c *Compiler) endpointDirective() error {
	if c.t.EndOfStatement() {
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
	if c.t.EndOfStatement() {
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
	if c.t.EndOfStatement() {
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
	c.b.Emit(bytecode.Load, defs.RequestVariable)

	// Generate a new response and put it on the stack.
	c.b.Emit(bytecode.Load, defs.ResponseWriterVariable)

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
	if c.t.EndOfStatement() {
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

	if c.t.EndOfStatement() {
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

	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrMissingStatement)
	}

	branch := c.b.Mark()
	c.b.Emit(bytecode.BranchTrue, 0)

	if err := c.compileStatement(); err != nil {
		return err
	}

	return c.b.SetAddressHere(branch)
}

// lineDirective implements "@line N", which resets the source-line counter to N.
// This is used by tools that generate Ego source from a template so that error
// messages refer to the original file's line numbers rather than the generated
// line numbers. When the debugger is active the directive is silently ignored
// because the debugger manages its own line-number tracking.
func (c *Compiler) lineDirective() error {
	if c.t.EndOfStatement() {
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

	// If @line 1, no offset. Additionally, take off one for the @line directive itself.
	c.lineNumberOffset = line - 2

	return nil
}

// defineDirective implements "@define name1, name2, …", which pre-declares one
// or more identifiers as globally known symbols. This suppresses "unknown symbol"
// or "unused variable" warnings for names that are provided by the runtime
// environment (e.g. injected by a server handler) rather than declared in
// the Ego source itself.
func (c *Compiler) defineDirective() error {
	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrInvalidInteger)
	}

	// Accept a list of identifiers and define them as "used" symbol names
	// so they don't throw an error during compilation if not referenced later.
	for !c.t.EndOfStatement() {
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
	if c.t.EndOfStatement() {
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

// optimizerDirective parses the @optimizer directive, which turns the compiler
// optimizer on and off. It must be followed by a token "on" or "off".
func (c *Compiler) optimizerDirective() error {
	var (
		err  error
		mode int
	)

	err = c.compileError(errors.ErrInvalidDirective.Context("@optimizer on|off"))

	if c.t.EndOfStatement() {
		return err
	}

	switch next := strings.ToLower(c.t.Next().Spelling()); next {
	case "on", "true", "1":
		mode = 1

	case "off", "false", "0":
		mode = 0

	case "always":
		mode = 2
	default:
		return err
	}

	settings.SetDefault(defs.OptimizerSetting, strconv.Itoa(mode))

	return nil
}

// logDirective parses the @log directive.
func (c *Compiler) logDirective() error {
	if c.t.EndOfStatement() {
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

	if c.t.EndOfStatement() {
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

// templateDirective implements the template compiler directive.
func (c *Compiler) templateDirective() error {
	if c.t.EndOfStatement() {
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

// errorDirective implements the @error directive.  It generates a runtime
// error whose message is the evaluated expression (or a generic panic error
// when no expression is supplied).  Unlike @fail, the error can be caught and
// inspected inside a try/catch block.
func (c *Compiler) errorDirective() error {
	if !c.atStatementEnd() {
		if err := c.emitExpression(); err != nil {
			return err
		}
	} else {
		c.b.Emit(bytecode.Push, errors.ErrPanic)
	}

	// Signal pops the value from the stack and throws it as a trappable error.
	// If the value is a string, it becomes errors.Message(string).
	// If it is already an *errors.Error it is used directly.
	c.b.Emit(bytecode.Signal, nil)

	return nil
}

// extensionsDirective implements "@extensions [true|false|default]". It controls
// whether the Ego language extensions (print, try/catch, exit, etc.) are available
// for the remainder of the compilation. The change takes effect both in the
// compiler (so subsequent tokens are parsed correctly) and at runtime (a
// StoreGlobal instruction records the setting in the root symbol table so that
// any sub-compilations inherit it).
func (c *Compiler) extensionsDirective() error {
	var extensions bool

	if c.t.IsNext(tokenizer.NewIdentifierToken("default")) {
		extensions = settings.GetBool(defs.ExtensionsEnabledSetting)
	} else if c.t.IsNext(tokenizer.TrueToken) {
		extensions = true
	} else if c.t.IsNext(tokenizer.FalseToken) {
		extensions = false
	} else if c.t.EndOfStatement() {
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

	if c.t.EndOfStatement() {
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
	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrMissingExpression)
	}

	if err := c.parseStruct(true); err != nil {
		return err
	}

	c.b.Emit(bytecode.StoreGlobal, defs.LocalizationVariable)

	return nil
}

func (c *Compiler) packagesDirective(singular bool) error {
	if c.t.EndOfStatement() {
		if singular {
			return c.compileError(errors.ErrMissingPackageName)
		}

		c.b.Emit(bytecode.DumpPackages)

		return nil
	}

	if !singular {
		return c.compileError(errors.ErrUnexpectedToken).Context(c.t.Peek(1))
	}

	names := make([]any, 0, 5)

	for {
		if c.t.EndOfStatement() {
			break
		}

		name := c.t.Next()
		if name.Is(tokenizer.MultiplyToken) {
			packageList := packages.List()
			names = make([]any, len(packageList))

			for i, p := range packageList {
				names[i] = p
			}

			break
		}

		if !name.IsIdentifier() {
			return c.compileError(errors.ErrInvalidPackageName).Context(name)
		}

		names = append(names, c.normalize(name.Spelling()))

		c.t.IsNext(tokenizer.CommaToken)
	}

	c.b.Emit(bytecode.DumpPackages, data.NewList(names...))

	return nil
}

// compileBlockDirective processes the @compile directive.
// The directive is followed by bracketed code that is
// compiled. If the compilation fails, the generated
// code signals the error from the compiler. If the
// code does compile, the compiled code is inserted
// into the bytecode stream where the @compile directive
// was found.
//
// The intent is to allow control of compiler errors,
// so you could compile code in a try/catch block and
// validate the compile error in the catch block,
// without stopping the test. This would allow Ego
// @test jobs that include validation of compiler
// fixes.
func (c *Compiler) compileBlockDirective() error {
	// Supported directive modifiers
	const (
		unusedVarsFlag  = "unused"
		unknownVarsFlag = "unknown"
		optimizeFlag    = "optimize"
		eofFlag         = "eof"
		trueFlag        = "true"
		falseFlag       = "false"
		onFlag          = "on"
		offFlag         = "off"
	)

	var (
		savedUnusedVars  = settings.GetBool(defs.UnusedVarsSetting)
		savedUnknownVars = settings.GetBool(defs.UnknownVarSetting)
		savedOptimize    = settings.GetInt(defs.OptimizerSetting)

		unusedVars  = savedUnusedVars
		unknownVars = savedUnknownVars
		optimize    = savedOptimize
		blockMode   = false

		// eofMarker holds the value of an "eof=" option, e.g.
		// @compile eof="$EOF". It stays empty ("") unless that option is
		// given. When it IS given, the code to compile is delimited by a
		// text marker instead of matching "{" "}" braces -- see the long
		// comment above collectTokensUntilEOFMarker for why a test author
		// would want that.
		eofMarker = ""
	)

	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrMissingStatement)
	}

	// Process any optional flags that modify the behavior of the @compile
	// directive. Normally this loop reads flags until it reaches the "{"
	// that opens a traditional brace-delimited block. But "eof=" mode has
	// no opening brace at all -- the code to compile is just the plain
	// statements that follow, up to a marker string -- so the loop must
	// also stop cleanly at the end of the @compile statement itself (the
	// synthetic ";" the tokenizer inserts at the end of the source line).
	for !c.t.Peek(1).Is(tokenizer.BlockBeginToken) && !c.t.EndOfStatement() {
		switch c.t.Peek(1).Spelling() {
		case "block":
			// "block" means this is block of code and doesn't require
			// a full program prolog and main function.
			blockMode = true

			c.t.Advance(1)

		case unusedVarsFlag:
			c.t.Advance(1)

			if !c.t.IsNext(tokenizer.AssignToken) {
				return c.compileError(errors.ErrUnexpectedToken).Context(c.t.Peek(1))
			}

			flag := c.t.Next().Spelling()
			switch strings.ToLower(flag) {
			case trueFlag, onFlag, "1":
				unusedVars = true

			case falseFlag, offFlag, "0":
				unusedVars = false

			default:
				return c.compileError(errors.ErrInvalidBooleanValue).Context(flag)
			}

		case unknownVarsFlag:
			c.t.Advance(1)

			if !c.t.IsNext(tokenizer.AssignToken) {
				return c.compileError(errors.ErrUnexpectedToken).Context(c.t.Peek(1))
			}

			flag := c.t.Next().Spelling()
			switch strings.ToLower(flag) {
			case trueFlag, onFlag, "1":
				unknownVars = true

			case falseFlag, offFlag, "0":
				unknownVars = false

			default:
				return c.compileError(errors.ErrInvalidBooleanValue).Context(flag)
			}

		case optimizeFlag:
			c.t.Advance(1)

			if !c.t.IsNext(tokenizer.AssignToken) {
				return c.compileError(errors.ErrUnexpectedToken).Context(c.t.Peek(1))
			}

			flag := c.t.Next()
			switch strings.ToLower(flag.Spelling()) {
			case offFlag, falseFlag:
				optimize = 0

			case "low":
				optimize = 1

			case "high":
				optimize = 2

			default:
				if flag.Class() != tokenizer.IntegerTokenClass {
					return c.compileError(errors.ErrInvalidValue).Context(flag)
				}

				var err error

				optimize, err = strconv.Atoi(flag.Spelling())
				if err != nil || optimize < 0 || optimize > 2 {
					return c.compileError(errors.ErrInvalidValue).Context(flag)
				}
			}

		case eofFlag:
			// eof="<marker>" switches this @compile directive from the
			// traditional "{ ... }" brace-delimited block to EOF-marker
			// mode: the code to compile is every statement that follows,
			// up to (but not including) a run of tokens whose spellings,
			// concatenated together, exactly equal <marker>. The value
			// must be a quoted string, e.g. eof="$EOF" or eof="###".
			c.t.Advance(1)

			if !c.t.IsNext(tokenizer.AssignToken) {
				return c.compileError(errors.ErrUnexpectedToken).Context(c.t.Peek(1))
			}

			markerToken := c.t.Next()
			if !markerToken.IsString() {
				return c.compileError(errors.ErrInvalidValue).Context(markerToken)
			}

			eofMarker = markerToken.Spelling()
			if eofMarker == "" {
				// An empty marker could never be distinguished from "the
				// very next token", which would make every @compile eof=
				// directive stop immediately with zero lines of code. This
				// is always a test-authoring mistake, so reject it early
				// with a clear error rather than silently compiling an
				// empty program.
				return c.compileError(errors.ErrInvalidValue).Context(markerToken)
			}

		default:
			return c.compileError(errors.ErrInvalidKeyword).Context(c.t.Peek(1).Spelling())
		}
	}

	// How the directive's own statement ends depends on which mode we're
	// in. Traditional mode ends with the block's opening "{", which must
	// be consumed here so the tokens that follow are just the block's
	// contents. EOF-marker mode has no such brace: the flag loop above
	// already stopped at the end of the @compile statement, so all that is
	// left to do is consume the synthetic ";" the tokenizer placed there.
	if eofMarker == "" {
		// Must start with opening braces with a block to compile.
		if !c.t.IsNext(tokenizer.BlockBeginToken) {
			return c.compileError(errors.ErrMissingStatement)
		}
	} else {
		_ = c.t.IsNext(tokenizer.SemicolonToken)
	}

	// Create a new compiler to compile the code in the block. Compile
	// into a new block so if the compilation fails, we won't have added
	// debris to the current bytecode stream.
	subCompiler := New("@compile")
	subCompiler.flags.extensionsEnabled = c.flags.extensionsEnabled

	// Load up the other values from the @compile directive flags, which may
	// just hold the original unchanged values...
	settings.SetDefault(defs.UnusedVarsSetting, strconv.FormatBool(unusedVars))
	settings.SetDefault(defs.UnknownVarSetting, strconv.FormatBool(unknownVars))
	settings.SetDefault(defs.OptimizerOption, strconv.Itoa(optimize))

	// Make sure we put everything back when we're done.
	defer func() {
		settings.SetDefault(defs.UnusedVarsSetting, strconv.FormatBool(savedUnusedVars))
		settings.SetDefault(defs.UnknownVarSetting, strconv.FormatBool(savedUnknownVars))
		settings.SetDefault(defs.OptimizerOption, strconv.Itoa(savedOptimize))
	}()

	// If block mode was enabled in the @compile directive, the code in
	// the block is a true block. OTherwise, we leave the block dept at 0
	// so the sub-compiler must compile a full program with prolog and main
	// function.
	if blockMode {
		subCompiler.functionDepth = c.functionDepth
		subCompiler.blockDepth = c.blockDepth + 1
		subCompiler.flags = c.flags
	}
	// Must wait til now to set this value so 'block' mode doesn't override
	// it from the outer compiler. Also, even though the parent compiler may
	// be evaluating a code fragment, we have to do the sub-compile as a first
	// class compiler, so disable fragment mode if on.
	subCompiler.flags.unusedVars = unusedVars
	subCompiler.flags.fragment = false
	subCompiler.flags.trial = false

	// Collect up all the tokens that make up the code to compile. These
	// will be the tokens sent to the sub-compiler for compilation. Exactly
	// how they are collected depends on which delimiter mode this
	// @compile directive used.
	var tokens *tokenizer.Tokenizer

	if eofMarker != "" {
		// EOF-marker mode: scan forward until a run of tokens spells out
		// eofMarker exactly; everything before that run is the code to
		// compile. See collectTokensUntilEOFMarker for the full algorithm.
		// Unlike brace-delimited mode, this never needs to look at "{" or
		// "}" at all, so mismatched braces inside the collected code are
		// simply passed through to the sub-compiler like any other token
		// -- which is the whole point of offering this mode (see the
		// comment on eofMarker above).
		found := false

		tokens, found = c.collectTokensUntilEOFMarker(eofMarker)
		if !found {
			// Ran out of source before ever seeing the marker text. This
			// is always a test-authoring mistake (a typo in the marker,
			// or a forgotten terminator line), so report it clearly
			// instead of silently compiling whatever was left.
			return c.compileError(errors.ErrMissingEOFMarker).Context(eofMarker)
		}

		// The marker itself may be followed by the tokenizer's own
		// synthetic ";" (inserted because the marker text ended a source
		// line). Consume it if present so a following "catch(e) { ... }"
		// clause, if any, is seen cleanly by the code below.
		_ = c.t.IsNext(tokenizer.SemicolonToken)
	} else {
		// Traditional brace-delimited mode: collect up all the tokens in
		// the existing token stream up to the matching closing brace.
		//
		// braces starts at 1 because the directive's own opening "{" was
		// already consumed above (the "must start with opening braces"
		// check a few lines up) -- it is not re-counted here. Each "{"
		// encountered while scanning the block's contents (an "if", "for",
		// nested "func", etc.) pushes the count up by one; each "}"
		// pops it back down by one. The token stream is only fully
		// balanced again -- i.e. we have reached the "}" that matches the
		// directive's own opening "{" -- when the count returns all the
		// way to 0, not merely back down to 1.
		//
		// fix BUG-39: the original condition was "braces <= 1", which
		// treated the closing brace of ANY nested block (e.g. the "}" that
		// ends an "if" body, or the "}" that ends a nested "func" body) as
		// if it were the directive's own closing brace, because after that
		// first nested block the count is back down to 1. That silently
		// truncated the collected token stream partway through the block,
		// leaving the rest of the source (including a trailing "catch(e)"
		// clause) to desynchronize the surrounding parse. For the
		// non-"block" (full-program) form specifically, exactly one
		// dropped "}" happened to be invisibly patched back on by the
		// "tokens.Append(tokenizer.BlockEndToken)" a few lines below,
		// which is why a single top-level construct (one function
		// declaration) appeared to work while two or more did not: only
		// one dropped brace can ever be compensated for that way.
		tokens = tokenizer.New("", true)
		braces := 1

		var t tokenizer.Token

		for !c.t.AtEnd() {
			t = c.t.Next()

			if t.Is(tokenizer.BlockBeginToken) {
				braces++
			} else if t.Is(tokenizer.BlockEndToken) {
				braces--

				if braces == 0 {
					break
				}
			}

			tokens.Append(t)
		}

		// There must be a closing brace to match the opening brace of the block.
		// It may be preceded by a semicolon, but it must be present.
		// With this operation, all tokens for the @compile directive have been
		// consumed, so the statement tokenizer that called us can continue on to
		// the next statement.
		//
		// Note: with the BUG-39 fix above, the loop now always consumes
		// exactly the one closing brace that matches the directive's own
		// opening brace -- for both "block" and full-program mode alike.
		// There is no second closing brace to look for afterward, and no
		// dropped brace to patch back onto `tokens`, so the compensating
		// "require/append one more BlockEndToken" logic that used to
		// follow this comment (see the git history for BUG-39) has been
		// removed as dead weight rather than a second, independent bug.
		_ = c.t.IsNext(tokenizer.SemicolonToken)
	}

	// Generate start of a try block. The @compile directive is an implied
	// try statement, with an optional catch to let a test validate a compile
	// issue.
	b1 := c.b.Mark()
	tryMarker := bytecode.NewStackMarker("try")

	c.b.Emit(bytecode.Try, 0)
	c.b.Emit(bytecode.Push, tryMarker)

	// Compile the block of code.
	bc, err := subCompiler.Compile("@compile", tokens)
	if err == nil {
		err = subCompiler.Errors()
	}

	if err != nil {
		c.b.Emit(bytecode.Push, err)
		c.b.Emit(bytecode.Signal, nil)
	} else {
		// If the compilation succeeded, add the compiled code to the
		// current bytecode stream.
		c.b.Append(bc)

		// For full-program mode (@compile without "block"), propagate the names
		// declared at the top level of the sub-compilation into the OUTER
		// compiler's static symbol tracker. This is necessary because Ego
		// compilation is a single sequential pass: by the time the @compile
		// directive is processed, the outer compiler has already started
		// compiling the statements that come after it (e.g. `result := foo(...)`),
		// and those statements are validated at compile time — before any bytecode
		// ever runs. Without this registration, the outer compiler's
		// validateSymbol call in compileSymbolValue rejects the name with
		// "unknown symbol", even though the inline-spliced bytecode has already
		// arranged to store the value in the live runtime symbol table via a
		// StoreAlways instruction.
		//
		// NOTE: The names come from subCompiler.scopes[0] (the outermost scope
		// frame of the sub-compiler, which captures every DefineSymbol and
		// DefineGlobalSymbol call made during top-level statement compilation).
		// The compiler's root-level symbol table c.s is NOT used here because
		// compilation never writes values into c.s — it is a read-only lookup
		// table for resolving pre-existing package and type names.
		//
		// Only full-program mode (@compile without "block") is affected.
		// Block mode (@compile block) is deliberately left unchanged: block mode
		// tests rely on being able to declare scratch variables inside the block
		// whose unused-variable status the compiler can check. Exporting those
		// names here would not interfere with unused-variable detection (detection
		// happens inside the sub-compilation before this success path runs), but
		// the scope boundary between block-mode content and the enclosing test is
		// intentionally kept strict to avoid polluting the test's symbol scope.
		if !blockMode && len(subCompiler.scopes) > 0 {
			for name := range subCompiler.scopes[0].usage {
				// Register the name with the outer compiler so that source code
				// written after the @compile block can reference it without
				// triggering a false "unknown symbol" compile-time error.
				// The runtime value is already bound by StoreAlways/Store
				// instructions in the inline-spliced bytecode — no additional
				// CreateAndStore bytecode is needed here.
				//
				// DefineGlobalSymbol is idempotent on already-known names, and
				// it marks the entry as "already used" (nil) so no spurious
				// unused-variable warning is generated for names the caller
				// happens not to reference in every code path. This is the same
				// technique used in the BUG-09 fix in import.go.
				c.DefineGlobalSymbol(name)
			}
		}
	}

	c.b.Emit(bytecode.DropToMarker, tryMarker)
	b2 := c.b.Mark()

	// The catch block is optional. If not found, patch up the try destination to here
	// and generate a pop of the try stack.
	if !c.t.IsNext(tokenizer.CatchToken) {
		_ = c.b.SetAddressHere(b1)
		c.b.Emit(bytecode.TryPop)

		return nil
	}

	// Need to generate a branch around the catch block for success cases.
	c.b.Emit(bytecode.Branch, 0)
	_ = c.b.SetAddressHere(b1)

	// Is there a named variable that will hold the error?

	if c.t.IsNext(tokenizer.StartOfListToken) {
		errName := c.t.Next()
		if !errName.IsIdentifier() {
			return c.compileError(errors.ErrInvalidSymbolName)
		}

		if !c.t.IsNext(tokenizer.EndOfListToken) {
			return c.compileError(errors.ErrMissingParenthesis)
		}

		c.b.Emit(bytecode.Load, defs.ErrorVariable)
		c.b.Emit(bytecode.StoreAlways, errName)
		c.DefineSymbol(errName.Spelling())
	}

	if err := c.compileRequiredBlock(false, true); err != nil {
		return err
	}
	// Need extra PopScope because we're still running in the scope of the try{} block
	//c.b.Emit(bytecode.PopScope)

	// This marks the end of the try/catch
	_ = c.b.SetAddressHere(b2)
	c.b.Emit(bytecode.TryPop)

	return nil
}

// collectTokensUntilEOFMarker implements the "eof=" option of the @compile
// directive (see compileBlockDirective). Instead of the traditional
// "{ ... }" brace-delimited block, this mode lets a test write the code to
// compile as plain statements terminated by an arbitrary marker string, e.g.:
//
//	@compile eof="$EOF"
//	    fmt.Println(1,,2)
//	$EOF
//
// WHY THIS EXISTS (read this if you're new to the codebase):
//
// The normal "{ ... }" form finds the end of its block by counting braces:
// every "{" it sees adds one to a counter, every "}" subtracts one, and it
// stops when the counter falls back to the starting level. That works fine
// for well-formed code, but it makes it very hard to write a test whose
// whole POINT is that the code has mismatched braces (for example, testing
// that the compiler reports a sensible error for a missing "}"). With
// brace-counting, a stray extra "{" or a missing "}" inside the test code
// throws off the count and the @compile directive itself may fail to find
// its own end, confusing the test in ways that have nothing to do with what
// the test is actually trying to check.
//
// "eof=" mode sidesteps all of that: it doesn't look at "{" or "}" tokens
// specially at all. It just keeps reading tokens and gluing their spellings
// together, watching for the moment that glued-together text exactly
// matches the marker string the test author chose. Everything read before
// that point -- braces, mismatched or not -- is simply handed to the
// sub-compiler as the code to test. The marker itself is discarded (it is
// punctuation for the test file, not code to compile).
//
// The marker can be one token or several: the comparison is done against
// concatenated *spellings*, not against a single token, so a marker like
// "$EOF" will actually be matched as two tokens ("$" and "EOF") glued
// together, and a marker like "END" might be one identifier token. This is
// deliberate -- it means the test author can pick literally any marker text
// that won't appear naturally in their test code, without needing to know
// how the Ego tokenizer would lex it.
//
// Parameters:
//   - marker: the exact text to watch for, e.g. "$EOF". Must be non-empty
//     (compileBlockDirective rejects an empty marker before calling this).
//
// Returns:
//   - a new *tokenizer.Tokenizer containing every token read BEFORE the
//     marker was found (this is what gets handed to the sub-compiler).
//   - true if the marker was found; false if the input ran out first (the
//     caller should treat this as a test-authoring error and not attempt to
//     compile the partial result).
//
// Note on limits: this uses a simple "try to match, and if it stops
// matching, flush what we were holding and start over from right here"
// strategy. It does not do full backtracking (the way, say, a regular
// expression engine would) to look for a match that starts in the *middle*
// of a run of tokens we already gave up on. For an arbitrary marker chosen
// specifically to be unlikely to appear in ordinary code -- which is the
// whole point of letting the test author pick it -- this is never an issue
// in practice, and keeping the algorithm simple makes it easy to verify by
// reading it.
func (c *Compiler) collectTokensUntilEOFMarker(marker string) (*tokenizer.Tokenizer, bool) {
	// This is the token stream we will hand back to the caller: everything
	// that turned out to be real code, not part of the marker.
	tokens := tokenizer.New("", true)

	// While we are in the middle of a possible match, we hold the tokens we
	// have read so far in "pending" instead of putting them in "tokens"
	// right away -- we don't yet know if they are the marker or just code
	// that happens to start with the same letters.
	var pending []tokenizer.Token

	// "candidate" is always the concatenation of the spellings of every
	// token currently sitting in "pending". It starts empty, and it is
	// always either empty or a proper (partial) prefix of "marker".
	candidate := ""

	for !c.t.AtEnd() {
		next := c.t.Next()

		// What would our candidate become if we added this new token?
		attempt := candidate + next.Spelling()

		if attempt == marker {
			// Complete match! The tokens in "pending" plus this one spell
			// out the marker exactly, so none of them are code -- we just
			// throw them away and report success. Note we do NOT append
			// "next" to "tokens" here.
			return tokens, true
		}

		if strings.HasPrefix(marker, attempt) {
			// Not a complete match yet, but still consistent with the
			// marker so far (e.g. marker is "$EOF" and attempt is "$E").
			// Keep holding these tokens back and keep looking.
			pending = append(pending, next)
			candidate = attempt

			continue
		}

		// This token broke the match. Everything we were holding in
		// "pending" turns out to have been ordinary code after all (it
		// just happened to start the same way the marker does), so release
		// it into the real result now, in the order it was read.
		tokens.Append(pending...)
		pending = nil
		candidate = ""

		// The token that broke the match might, on its own, be the start
		// of a brand new match attempt (this matters if the marker's first
		// character can also appear elsewhere in ordinary code). Check it
		// against a fresh, empty candidate before deciding it's just plain
		// code.
		if next.Spelling() == marker {
			return tokens, true
		}

		if next.Spelling() != "" && strings.HasPrefix(marker, next.Spelling()) {
			pending = append(pending, next)
			candidate = next.Spelling()
		} else {
			tokens.Append(next)
		}
	}

	// We reached the end of the source without ever completing a match.
	// The caller treats this as an error, so it doesn't matter that
	// "pending" (a false start) never made it into "tokens" -- the whole
	// result is going to be discarded anyway.
	return tokens, false
}
