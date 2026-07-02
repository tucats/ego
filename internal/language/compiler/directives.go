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
		trueFlag        = "true"
		falseFlag       = "false"
		onFlag          = "on"
		offFlag         = "off"
	)

	var (
		savedUnusedVars  = settings.GetBool(defs.UnusedVarLoggingSetting)
		savedUnknownVars = settings.GetBool(defs.UnknownVarSetting)
		savedOptimize    = settings.GetInt(defs.OptimizerSetting)

		unusedVars  = savedUnusedVars
		unknownVars = savedUnknownVars
		optimize    = savedOptimize
		blockMode   = false
	)

	if c.t.EndOfStatement() {
		return c.compileError(errors.ErrMissingStatement)
	}

	// Process any optional flags that modify the behavior of the @compile directive.
	for !c.t.Peek(1).Is(tokenizer.BlockBeginToken) {
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

		default:
			return c.compileError(errors.ErrInvalidKeyword).Context(c.t.Peek(1).Spelling())
		}
	}

	// Must start with opening braces with a block to compile.
	if !c.t.IsNext(tokenizer.BlockBeginToken) {
		return c.compileError(errors.ErrMissingStatement)
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

	// Collect up all the tokens in the exiting token stream up to the
	// mismatched closing brace. These will be the tokens sent to the
	// sub-compiler for compilation.
	tokens := tokenizer.New("", true)
	braces := 1

	var t tokenizer.Token

	for !c.t.AtEnd() {
		t = c.t.Next()

		if t.Is(tokenizer.BlockBeginToken) {
			braces++
		} else if t.Is(tokenizer.BlockEndToken) {
			braces--

			if braces <= 1 {
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
	_ = c.t.IsNext(tokenizer.SemicolonToken)

	if !blockMode && !c.t.IsNext(tokenizer.BlockEndToken) {
		return c.compileError(errors.ErrMissingStatement)
	}

	// We owe the token stream we compile a closing brace.
	if !blockMode {
		tokens.Append(tokenizer.BlockEndToken)
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

	if err := c.compileRequiredBlock(false); err != nil {
		return err
	}
	// Need extra PopScope because we're still running in the scope of the try{} block
	//c.b.Emit(bytecode.PopScope)

	// This marks the end of the try/catch
	_ = c.b.SetAddressHere(b2)
	c.b.Emit(bytecode.TryPop)

	return nil
}
