package compiler

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// requiredPackages is the list of packages that are always imported, regardless
// of user import statements or auto-import profile settings. These are needed to
// exit the program, for example.
var requiredPackages []string = []string{
	"os",
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
	exitEnabled           bool // Only true in interactive mode
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
	coercions         []*bytecode.ByteCode
	constants         []string
	deferQueue        []deferStatement
	returnVariables   []returnVariable
	packages          map[string]*data.Package
	packageMutex      sync.Mutex
	types             map[string]*data.Type
	functionDepth     int
	blockDepth        int
	statementCount    int
	flags             flagSet // Use to hold parser state flags
}

// New creates a new compiler instance.
func New(name string) *Compiler {
	// Are language extensions enabled? We use the default unless the value is
	// overridden in the root symbol table.
	extensions := settings.GetBool(defs.ExtensionsEnabledSetting)
	if v, ok := symbols.RootSymbolTable.Get(defs.ExtensionsVariable); ok {
		extensions = data.Bool(v)
	}

	// What is the status of type checking? Use the default value unless the value
	// is overridden in the root symbol table.
	typeChecking := settings.GetBool(defs.StaticTypesSetting)
	if v, ok := symbols.RootSymbolTable.Get(defs.TypeCheckingVariable); ok {
		typeChecking = (data.Int(v) == defs.StrictTypeEnforcement)
	}

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
		flags: flagSet{
			normalizedIdentifiers: false,
			extensionsEnabled:     extensions,
			strictTypes:           typeChecking,
		},
		rootTable: &symbols.RootSymbolTable,
	}
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

	return c.Compile(name, t)
}

// Compile processes a token stream and generates a bytecode stream. This is the basic
// operation of the compiler.
func (c *Compiler) Compile(name string, t *tokenizer.Tokenizer) (*bytecode.ByteCode, error) {
	c.b = bytecode.New(name)
	c.t = t

	c.t.Reset()

	// Iterate over the tokens, compiling each statement.
	for !c.t.AtEnd() {
		if err := c.compileStatement(); err != nil {
			return nil, err
		}
	}

	// Return the slice of the generated code for this compilation. The Seal
	// operation truncates the bytecode array to the smallest size possible.
	return c.b.Seal(), nil
}

// AddBuiltins adds the builtins for the named package (or prebuilt builtins if the package name
// is empty).
func (c *Compiler) AddBuiltins(pkgname string) bool {
	added := false

	// Get the symbol table for the named package.
	pkg, _ := bytecode.GetPackage(pkgname)
	symV, _ := pkg.Get(data.SymbolsMDKey)
	syms := symV.(*symbols.SymbolTable)

	ui.Log(ui.CompilerLogger, "### Adding builtin packages to %s package", pkgname)

	functionNames := make([]string, 0)
	for k := range builtins.FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	sort.Strings(functionNames)

	for _, name := range functionNames {
		f := builtins.FunctionDictionary[name]

		if dot := strings.Index(name, "."); dot >= 0 {
			f.Pkg = name[:dot]
			f.Name = name[dot+1:]
			name = f.Name
		} else {
			f.Name = name
		}

		if f.Pkg == pkgname {
			if ui.IsActive(ui.CompilerLogger) {
				debugName := name
				if f.Pkg != "" {
					debugName = f.Pkg + "." + name
				}

				ui.Log(ui.CompilerLogger, "... processing builtin %s", debugName)
			}

			added = true

			if pkgname == "" && c.s != nil {
				syms.SetAlways(name, f.F)
				pkg.Set(name, f.F)
			} else {
				if f.F != nil {
					syms.SetAlways(name, f.F)
					pkg.Set(name, f.F)
				} else {
					syms.SetAlways(name, f.V)
					pkg.Set(name, f.V)
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

	logger := ui.CompilerLogger

	ui.Log(logger, "Adding standard functions to %s (%v)", s.Name, s.ID())

	for name, f := range builtins.FunctionDictionary {
		if dot := strings.Index(name, "."); dot < 0 {
			_ = s.SetConstant(name, f.F)
			ui.Log(logger, "    adding %s", name)

			added = true
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

var packageMerge sync.Mutex

// AddPackageToSymbols adds all the defined packages for this compilation
// to the given symbol table. This function supports attribute chaining
// for a compiler instance.
func (c *Compiler) AddPackageToSymbols(s *symbols.SymbolTable) *Compiler {
	ui.Log(ui.CompilerLogger, "Adding compiler packages to %s(%v)", s.Name, s.ID())
	packageMerge.Lock()
	defer packageMerge.Unlock()

	for packageName, packageDictionary := range c.packages {
		// Skip over any metadata
		if strings.HasPrefix(packageName, data.MetadataPrefix) {
			continue
		}

		m := data.NewPackage(packageName)

		keys := packageDictionary.Keys()
		if len(keys) == 0 {
			continue
		}

		for _, k := range keys {
			v, _ := packageDictionary.Get(k)
			// Do we already have a package of this name defined?
			_, found := s.Get(k)
			if found {
				ui.Log(ui.CompilerLogger, "Duplicate package %s already in table", k)
			}

			// If the package name is empty, we add the individual items
			if packageName == "" {
				_ = s.SetConstant(k, v)
			} else {
				// Otherwise, copy the entire map
				m.Set(k, v)
			}
		}
		// Make sure the package is marked as readonly so the user can't modify
		// any function definitions, etc. that are built in.
		m.Set(data.TypeMDKey, data.PackageType(packageName))
		m.Set(data.ReadonlyMDKey, true)

		if packageName != "" {
			s.SetAlways(packageName, m)
		}
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
	ui.Log(ui.CompilerLogger, "+++ Starting auto-import all=%v", all)

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
		for fn := range builtins.FunctionDictionary {
			dot := strings.Index(fn, ".")
			if dot > 0 {
				fn = fn[:dot]
				uniqueNames[fn] = true
			}
		}

		// Add the list of packages that live in the runtime package tree.
		for _, name := range []string{
			"base64",
			"cipher",
			"db",
			"errors",
			"exec",
			"filepath",
			"io",
			"json",
			"math",
			"os",
			"reflect",
			"rest",
			"sort",
			"strconv",
			"strings",
			"tables",
			"time",
			"util",
			"uuid",
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
		text := tokenizer.ImportToken.Spelling() + " " + strconv.Quote(packageName)

		_, err := c.CompileString(packageName, text)
		if err != nil && firstError == nil {
			firstError = err
		}
	}

	c.b = savedBC
	c.t = savedT
	c.sourceFile = savedSource

	// Finally, traverse the package cache to move the symbols to the
	// given symbol table
	bytecode.CopyPackagesToSymbols(s)

	return firstError
}
