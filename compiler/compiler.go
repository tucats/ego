package compiler

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// requiredPackages is the list of packages that are always imported, regardless
// of user import statements or auto-import profile settings. These are needed to
// exit the program, for example.
var requiredPackages []string = []string{
	"os",
	"cipher",
	"profile",
}

// loop is a structure that defines a loop type.
type loop struct {
	// The parent loop, if any. This creates a single-linked
	// list that represents the active loop stack.
	parent *loop

	// The type of loop this is. This is used to determine if the
	// iterator is a range or calculated value. Valid values are
	// for loop, index loop, range loop, continditional loop.
	loopType runtimeLoopType

	// Fixup locations for break statements in a loop. These are
	//  the addresses that must be fixed up with a target address
	// pointing to exit point of the loop.
	breaks []int

	// Fixup locations for continue statements in a loop. These are
	// the addresses that must be fixed up with a target address
	// pointing to the start of the loop.
	continues []int
}

// flagSet contains flags that generally identify the state of
// the compiler at any given moment. For example, when parsing
// something like a switch conditional value, the value cannot
// be a struct initializer, though that is allowed elsewhere.
type flagSet struct {
	disallowStructInits   bool
	extensionsEnabled     bool
	normalizedIdentifiers bool
	strictTypes           bool
	testMode              bool
	mainSeen              bool
	hasUnwrap             bool
	inAssignment          bool
	multipleTargets       bool
	debuggerActive        bool
	closed                bool // True if the copmiler Close() has already been called
	trial                 bool // True if this is a trial compilation
	unusedVars            bool // True if unused variables are an error
	silent                bool // This compilation unit is not logged
	exitEnabled           bool // Only true in interactive mode
}

type importElement struct {
	err  error
	path string
}

type deferStatement struct {
	Name    string
	Address int
}

// returnVariable is a structure that defines a return variable that is
// explicitly identified in the function signature.
type returnVariable struct {
	Name string
	Type *data.Type
}

// Compiler is a structure defining what we know about the compilation.
type Compiler struct {
	activePackageName string
	sourceFile        string
	id                string
	b                 *bytecode.ByteCode
	t                 *tokenizer.Tokenizer
	s                 *symbols.SymbolTable
	rootTable         *symbols.SymbolTable
	loops             *loop
	parent            *Compiler
	coercions         []*bytecode.ByteCode
	constants         []string
	deferQueue        []deferStatement
	returnVariables   []returnVariable
	packages          map[string]*data.Package
	importStack       []importElement
	packageMutex      sync.Mutex
	types             map[string]*data.Type
	symbolErrors      map[string]*errors.Error
	started           time.Time
	scopes            []scope
	functionDepth     int
	blockDepth        int
	statementCount    int
	lineNumberOffset  int
	flags             flagSet // Use to hold parser state flags
}

// This is a list of the packages that were successfully auto-imported.
var AutoImportedPackages = make([]string, 0)

// This flag should only be turned on when tracking down issues with mismatched
// compiler open and close operations.
var debugCompilerLifetimes = false

// New creates a new compiler instance.
func New(name string) *Compiler {
	// Are language extensions enabled? We use the default unless the value is
	// overridden in the root symbol table.
	extensions := settings.GetBool(defs.ExtensionsEnabledSetting)
	if v, ok := symbols.RootSymbolTable.Get(defs.ExtensionsVariable); ok {
		extensions, _ = data.Bool(v)
	}

	// What is the status of type checking? Use the default value unless the value
	// is overridden in the root symbol table.
	typeChecking := settings.GetBool(defs.StaticTypesSetting)

	if v, ok := symbols.RootSymbolTable.Get(defs.TypeCheckingVariable); ok {
		typeChecking = (data.IntOrZero(v) == defs.StrictTypeEnforcement)
	}

	unusedVarsErr := settings.GetBool(defs.UnusedVarsSetting)

	// Create a new instance of the compiler.
	return &Compiler{
		b:            bytecode.New(name),
		t:            nil,
		s:            symbols.NewRootSymbolTable(name),
		id:           uuid.NewString(),
		constants:    make([]string, 0),
		deferQueue:   make([]deferStatement, 0),
		types:        map[string]*data.Type{},
		packageMutex: sync.Mutex{},
		packages:     map[string]*data.Package{},
		importStack:  make([]importElement, 0),
		symbolErrors: map[string]*errors.Error{},
		started:      time.Now(),
		flags: flagSet{
			normalizedIdentifiers: false,
			extensionsEnabled:     extensions,
			strictTypes:           typeChecking,
			unusedVars:            unusedVarsErr,
		},
	}
}

// Clone creates a new compiler instance that is a copy of the current one, setting
// the new compiler's parent to the current commpiler.
func (c *Compiler) Clone(name string) *Compiler {
	if name == "" {
		name = "clone of " + c.id
	}

	clone := New(name)

	// Copy the fields from the current compiler to the clone.
	clone.activePackageName = c.activePackageName
	clone.sourceFile = c.sourceFile
	clone.id = uuid.NewString() + ", clone of " + c.id
	clone.t = c.t
	clone.s = c.s
	clone.rootTable = c.rootTable
	clone.loops = c.loops
	clone.parent = c
	clone.flags = c.flags
	clone.flags.closed = false
	clone.functionDepth = c.functionDepth
	clone.blockDepth = c.blockDepth
	clone.statementCount = c.statementCount
	clone.started = c.started

	// Make a few copy of the slices.
	clone.constants = append([]string(nil), c.constants...)
	clone.deferQueue = append([]deferStatement(nil), c.deferQueue...)
	clone.returnVariables = append(clone.returnVariables, c.returnVariables...)
	clone.scopes = append([]scope(nil), c.scopes...)
	clone.coercions = append(clone.coercions, c.coercions...)
	clone.importStack = append([]importElement{}, c.importStack...)

	// Make copies of the maps
	clone.symbolErrors = map[string]*errors.Error{}
	for k, v := range c.symbolErrors {
		clone.symbolErrors[k] = v
	}

	clone.types = map[string]*data.Type{}
	for k, v := range c.types {
		clone.types[k] = v
	}

	// Copy the packages from the current compiler to the clone.
	clone.packages = map[string]*data.Package{}
	for k, v := range c.packages {
		clone.packages[k] = v
	}

	if debugCompilerLifetimes {
		ui.Log(ui.CompilerLogger, "compiler.open", ui.A{
			"id": c.id})
	}

	return clone
}

func (c *Compiler) Open() *Compiler {
	if c == nil {
		panic("Compiler.Open called on nil compiler")
	}

	c.flags.closed = false

	if debugCompilerLifetimes {
		ui.Log(ui.CompilerLogger, "compiler.open", ui.A{
			"id": c.id})
	}

	return c
}

// Close terminates the use of a copmiler. If it was a clone, the state is copied
// back to the parent compiler. If it was not a clone, then deferred errors generated
// during the compilation are reported.
func (c *Compiler) Close() (*bytecode.ByteCode, error) {
	var err error

	if c == nil {
		return nil, nil
	}

	if c.flags.closed {
		ui.Log(ui.CompilerLogger, "compiler.closed", ui.A{
			"id": c.id})

		return nil, nil
	}

	// If we are a clone, restore everything back to the parent compiler except
	// the bytecode and coercions.
	if c.parent != nil {
		c.parent.statementCount = c.statementCount
		c.parent.flags = c.flags
		c.parent.functionDepth = c.functionDepth
		c.parent.blockDepth = c.blockDepth

		c.parent.constants = c.constants
		c.parent.deferQueue = c.deferQueue
		c.parent.returnVariables = c.returnVariables
		c.parent.scopes = c.scopes

		c.parent.types = c.types
		c.parent.packages = c.packages
		c.parent.symbolErrors = c.symbolErrors
	}

	result := c.b.Seal()

	// If this isn't a clone, time to check on any deferred error conditions.
	if c.parent == nil {
		err = c.Errors()

		if err == nil && !c.flags.silent {
			ui.Log(ui.CompilerLogger, "compiler.success", ui.A{
				"name":     c.b.Name(),
				"duration": time.Since(c.started).String()})
		}
	}

	if debugCompilerLifetimes {
		ui.Log(ui.CompilerLogger, "compiler.close", ui.A{
			"id": c.id})
	}

	c.flags.closed = true

	return result, err
}

// Get any deferred error state from this compilation unit. Doing so also resets
// the internal error state to nil.
func (c *Compiler) Errors() error {
	var err error

	if len(c.symbolErrors) > 0 {
		// Make a list of the error codes so we can sort them by variable name
		var sortedErrors []string

		for k := range c.symbolErrors {
			sortedErrors = append(sortedErrors, k)
		}

		sort.Strings(sortedErrors)

		// Format the errors into a single string
		for _, k := range sortedErrors {
			err = errors.Chain(errors.New(err), c.symbolErrors[k])
		}

		// Report the errors to the user
		ui.Log(ui.CompilerLogger, "compiler.errors", ui.A{
			"error": err.Error()})
	}

	// Reset the deferred error list.
	c.symbolErrors = map[string]*errors.Error{}

	return err
}

// NormalizedIdentifiers returns true if this instance of the compiler is folding
// all identifiers to a common (lower) case.
func (c *Compiler) NormalizedIdentifiers() bool {
	return c.flags.normalizedIdentifiers
}

// SetNormalizedIdentifiers sets the flag indicating if this compiler instance is
// folding all identifiers to a common case. This function supports attribute
// chaining for a compiler instance.
func (c *Compiler) SetNormalizedIdentifiers(flag bool) *Compiler {
	c.flags.normalizedIdentifiers = flag

	return c
}

// Override the default root symbol table for this compilation. This determines
// where package names are stored/found, for example. This is overridden by the
// web service handlers as they have per-call instances of root. This function
// supports attribute chaining for a compiler instance.
func (c *Compiler) SetRoot(s *symbols.SymbolTable) *Compiler {
	c.rootTable = s
	c.s.SetParent(s)

	return c
}

// If set to true, the compiler knows we are running in debugger mode,
// and disallows the @line directive and other actions that would
// prevent the debugger form functioning correctly.
func (c *Compiler) SetDebuggerActive(b bool) *Compiler {
	c.flags.debuggerActive = b

	return c
}

// If set to true, the compiler allows the "exit" statement. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) SetExitEnabled(b bool) *Compiler {
	c.flags.exitEnabled = b

	return c
}

// TesetMode returns whether the compiler is being used under control
// of the Ego "test" command, which has slightly different rules for
// block constructs.
func (c *Compiler) TestMode() bool {
	return c.flags.testMode
}

// MainSeen indicates if a "package main" has been seen in this
// compilation.
func (c *Compiler) MainSeen() bool {
	return c.flags.mainSeen
}

// SetTestMode is used to set the test mode indicator for the compiler.
// This is set to true only when running in Ego "test" mode. This
// function supports attribute chaining for a compiler instance.
func (c *Compiler) SetTestMode(b bool) *Compiler {
	c.flags.testMode = b

	return c
}

// Set the given symbol table as the default symbol table for
// compilation. This mostly affects how builtins are processed.
// This function supports attribute chaining for a compiler instance.
func (c *Compiler) WithSymbols(s *symbols.SymbolTable) *Compiler {
	c.s = s

	return c
}

// If set to true, the compiler allows the PRINT, TRY/CATCH, etc. statements.
// This function supports attribute chaining for a compiler instance.
func (c *Compiler) SetExtensionsEnabled(b bool) *Compiler {
	c.flags.extensionsEnabled = b

	return c
}

// WithTokens supplies the token stream to a compiler. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) WithTokens(t *tokenizer.Tokenizer) *Compiler {
	c.t = t

	return c
}

// SetNormalization sets the normalization flag. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) SetNormalization(f bool) *Compiler {
	c.flags.normalizedIdentifiers = f

	return c
}

// CompileString turns a string into a compilation unit. This is a helper function
// around the Compile() operation that removes the need for the caller
// to provide a tokenizer.
func (c *Compiler) CompileString(name string, source string) (*bytecode.ByteCode, error) {
	t := tokenizer.New(source, true)

	defer t.Close()

	return c.Compile(name, t)
}

// Compile processes a token stream and generates a bytecode stream. This is the basic
// operation of the compiler.
func (c *Compiler) Compile(name string, t *tokenizer.Tokenizer) (*bytecode.ByteCode, error) {
	start := time.Now()

	c.Open()

	c.b = bytecode.New(name)
	c.t = t

	c.t.Reset()

	// Iterate over the tokens, compiling each statement.
	for !c.t.AtEnd() {
		if err := c.compileStatement(); err != nil {
			end := time.Now()

			ui.Log(ui.CompilerLogger, "compiler.error", ui.A{
				"name":     name,
				"error":    err,
				"duration": end.Sub(start).String()})

			c.t.DumpTokens()

			return nil, err
		}
	}

	// Return the slice of the generated code for this compilation. The Seal
	// operation truncates the bytecode array to the smallest size possible.
	return c.Close()
}

// AddBuiltins adds the builtins for the named package (or prebuilt builtins if the package name
// is empty).
func (c *Compiler) AddBuiltins(pkgname string) bool {
	added := false

	// Get the symbol table for the named package.
	pkg, _ := bytecode.GetPackage(pkgname)
	syms := symbols.GetPackageSymbolTable(pkg)

	ui.Log(ui.PackageLogger, "pkg.builtins.package.add", ui.A{
		"name": pkgname})

	functionNames := make([]string, 0)
	for k := range builtins.FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	sort.Strings(functionNames)

	for _, name := range functionNames {
		f := builtins.FunctionDictionary[name]

		if dot := strings.Index(name, "."); dot >= 0 {
			f.Package = name[:dot]
			f.Name = name[dot+1:]
			name = f.Name
		} else {
			f.Name = name
		}

		if f.Package == pkgname {
			if ui.IsActive(ui.PackageLogger) {
				debugName := name
				if f.Package != "" {
					debugName = f.Package + "." + name
				}

				ui.Log(ui.PackageLogger, "pkg.builtins.processing", ui.A{
					"name": debugName})
			}

			added = true

			if pkgname == "" && c.s != nil {
				syms.SetAlways(name, f.FunctionAddress)
				pkg.Set(name, f.FunctionAddress)
			} else {
				if f.FunctionAddress != nil {
					syms.SetAlways(name, f.FunctionAddress)
					pkg.Set(name, f.FunctionAddress)
				} else {
					syms.SetAlways(name, f.Value)
					pkg.Set(name, f.Value)
				}
			}
		}
	}

	return added
}

// AddStandard adds the package-independent standard functions (like len() or make()) to the
// given symbol table.
func AddStandard(s *symbols.SymbolTable) bool {
	added := false

	if s == nil {
		return false
	}

	for name, f := range builtins.FunctionDictionary {
		if dot := strings.Index(name, "."); dot < 0 {
			// See if this is already found and defined as the function. If not, add it.
			v, found := s.Get(name)
			if _, ok := v.(data.Immutable); !ok || !found {
				_ = s.SetConstant(name, f.FunctionAddress)
				added = true
			}
		}
	}

	return added
}

// Get retrieves a compile-time symbol value.
func (c *Compiler) Get(name string) (interface{}, bool) {
	return c.s.Get(name)
}

// normalize performs case-normalization based on the current
// compiler settings.
func (c *Compiler) normalize(name string) string {
	if c.flags.normalizedIdentifiers {
		return strings.ToLower(name)
	}

	return name
}

// normalizeToken performs case-normalization based on the current
// compiler settings for an identifier token.
func (c *Compiler) normalizeToken(t tokenizer.Token) tokenizer.Token {
	if t.IsIdentifier() && c.flags.normalizedIdentifiers {
		return tokenizer.NewIdentifierToken(strings.ToLower(t.Spelling()))
	}

	return t
}

// SetInteractive indicates if the compilation is happening in interactive
// (i.e. REPL) mode. This function supports attribute chaining for a compiler
// instance.
func (c *Compiler) SetInteractive(b bool) *Compiler {
	if b {
		c.functionDepth++
	}

	return c
}

// isStatementEnd returns true when the next token is
// the end-of-statement boundary.
func (c *Compiler) isStatementEnd() bool {
	next := c.t.Peek(1)

	return tokenizer.InList(next, tokenizer.EndOfTokens, tokenizer.SemicolonToken, tokenizer.BlockEndToken)
}

// Symbols returns the symbol table map from compilation.
func (c *Compiler) Symbols() *symbols.SymbolTable {
	return c.s
}

// AutoImport arranges for the import of built-in packages. The
// parameter indicates if all available packages (including those
// found in the ego path) are imported, versus just essential
// packages like "util".
func (c *Compiler) AutoImport(all bool, s *symbols.SymbolTable) error {
	ui.Log(ui.PackageLogger, "pkg.compiler.autoimport", ui.A{
		"flag": all})

	// We do not want to dump tokens during import processing (there are a lot)
	// so turn of token logging during auto-import, and set it back on when done.
	savedTokenLogging := ui.IsActive(ui.TokenLogger)
	savedOptimizerLogging := ui.IsActive(ui.OptimizerLogger)
	savedTraceLogging := ui.IsActive(ui.TraceLogger)

	ui.Active(ui.TokenLogger, false)
	ui.Active(ui.OptimizerLogger, false)
	ui.Active(ui.TraceLogger, false)

	defer func(token, opt, trace bool) {
		ui.Active(ui.TokenLogger, token)
		ui.Active(ui.OptimizerLogger, opt)
		ui.Active(ui.TraceLogger, trace)
	}(savedTokenLogging, savedOptimizerLogging, savedTraceLogging)

	// Start by making a list of the packages. If we need all packages,
	// scan all the built-in function names for package names. We ignore
	// functions that don't have package names as those are already
	// available.
	//
	// If we aren't loading all packages, at least always load "util"
	// which is required for the exit command to function.
	uniqueNames := map[string]bool{}

	if all {
		// Add the list of packages that live in the runtime package tree. The number
		// indicates the order of the packages. All packages with "1" are processed first,
		// then all packages with "2", etc. This allows the list to control any dependencies
		// that might exist.
		for _, name := range []string{
			"1:base64",
			"1:cipher",
			"1:db",
			"1:errors",
			"1:exec",
			"1:filepath",
			"1:fmt",
			"1:io",
			"1:json",
			"1:math",
			"2:os",
			"1:profile",
			"1:reflect",
			"1:rest",
			"1:sort",
			"1:strconv",
			"1:strings",
			"1:tables",
			"1:time",
			"1:util",
			"1:uuid",
		} {
			uniqueNames[name] = true
		}
	} else {
		for _, p := range requiredPackages {
			uniqueNames[p] = true
		}
	}

	// Make the order stable
	sortedPackageNames := []string{}

	for k := range uniqueNames {
		sortedPackageNames = append(sortedPackageNames, k)
	}

	sort.Strings(sortedPackageNames)

	savedBC := c.b
	savedT := c.t
	savedSource := c.sourceFile

	var firstError error

	for _, packageName := range sortedPackageNames {
		// If the package name has a number followed by ":" at the start of the name,
		// remove that number.
		if colon := strings.Index(packageName, ":"); colon >= 0 {
			packageName = packageName[colon+1:]
		}

		// Add it to the list of packages we automimported. This is used for TEST mode
		// which needs to define these packages for each test compilation.
		AutoImportedPackages = append(AutoImportedPackages, packageName)

		// For an import statement and compile it.
		text := tokenizer.ImportToken.Spelling() + " " + strconv.Quote(packageName)

		_, err := c.CompileString(packageName, text)
		if err != nil {
			if firstError == nil {
				firstError = errors.New(err).In(packageName)

				ui.Log(ui.InternalLogger, "pkg.auto.import", nil)
			}

			ui.Log(ui.InternalLogger, "pkg.auto.import.error", ui.A{
				"name":  packageName,
				"error": err.Error()})
		}
	}

	c.b = savedBC
	c.t = savedT
	c.sourceFile = savedSource

	// Finally, traverse the package cache to move the symbols to the
	// given symbol table

	for _, packageName := range sortedPackageNames {
		// If the package name has a number followed by ":" at the start of the name,
		// remove that number.
		if colon := strings.Index(packageName, ":"); colon >= 0 {
			packageName = packageName[colon+1:]
		}

		pkgDef := packages.Get(packageName)
		if pkgDef != nil {
			s.SetAlways(packageName, pkgDef)
		}
	}

	return firstError
}
