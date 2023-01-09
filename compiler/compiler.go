package compiler

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// requiredPackages is the list of packages that are always imported, regardless
// of user import statements or auto-import profile settings.
var requiredPackages []string = []string{
	"os",
	"profile",
}

// loop is a structure that defines a loop type.
type loop struct {
	parent   *loop
	loopType int
	// Fixup locations for break or continue statements in a
	// loop. These are the addresses that must be fixed up with
	// a target address pointing to exit point or start of the loop.
	breaks    []int
	continues []int
}

// flagSet contains flags that generally identify the state of
// the compiler at any given moment. For example, when parsing
// something like a switch conditional value, the value cannot
// be a struct initializer, though that is allowed elsewhere.
type flagSet struct {
	disallowStructInits bool
	extensionsEnabled   bool
}

// Compiler is a structure defining what we know about the compilation.
type Compiler struct {
	activePackageName     string
	sourceFile            string
	b                     *bytecode.ByteCode
	t                     *tokenizer.Tokenizer
	s                     *symbols.SymbolTable
	rootTable             *symbols.SymbolTable
	loops                 *loop
	coercions             []*bytecode.ByteCode
	constants             []string
	deferQueue            []int
	packages              map[string]*data.EgoPackage
	packageMutex          sync.Mutex
	types                 map[string]*data.Type
	functionDepth         int
	blockDepth            int
	statementCount        int
	disasm                bool
	testMode              bool
	normalizedIdentifiers bool
	flags                 flagSet // Use to hold parser state flags
	exitEnabled           bool    // Only true in interactive mode
}

// New creates a new compiler instance.
func New(name string) *Compiler {
	cInstance := Compiler{
		b:                     nil,
		t:                     nil,
		s:                     symbols.NewRootSymbolTable(name),
		constants:             make([]string, 0),
		deferQueue:            make([]int, 0),
		types:                 map[string]*data.Type{},
		packageMutex:          sync.Mutex{},
		packages:              map[string]*data.EgoPackage{},
		normalizedIdentifiers: false,
		flags: flagSet{
			extensionsEnabled: settings.GetBool(defs.ExtensionsEnabledSetting),
		},
		rootTable: &symbols.RootSymbolTable,
	}

	return &cInstance
}

// NormalizedIdentifiers returns true if this instance of the compiler is folding
// all identifiers to a common (lower) case.
func (c *Compiler) NormalizedIdentifiers() bool {
	return c.normalizedIdentifiers
}

// SetNormalizedIdentifiers sets the flag indicating if this compiler instance is
// folding all identifiers to a common case. This function supports attribute
// chaining for a compiler instance.
func (c *Compiler) SetNormalizedIdentifiers(flag bool) *Compiler {
	c.normalizedIdentifiers = flag

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
func (c *Compiler) ExitEnabled(b bool) *Compiler {
	c.exitEnabled = b

	return c
}

// TesetMode returns whether the compiler is being used under control
// of the Ego "test" command, which has slightly different rules for
// block constructs.
func (c *Compiler) TestMode() bool {
	return c.testMode
}

// SetTestMode is used to set the test mode indicator for the compiler.
// This is set to true only when running in Ego "test" mode. This
// function supports attribute chaining for a compiler instance.
func (c *Compiler) SetTestMode(b bool) *Compiler {
	c.testMode = b

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
func (c *Compiler) ExtensionsEnabled(b bool) *Compiler {
	c.flags.extensionsEnabled = b

	return c
}

// WithTokens supplies the token stream to a compiler. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) WithTokens(t *tokenizer.Tokenizer) *Compiler {
	c.t = t

	return c
}

// WithNormalization sets the normalization flag. This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) WithNormalization(f bool) *Compiler {
	c.normalizedIdentifiers = f

	return c
}

// Disasm sets the disassembler flag.This function supports
// attribute chaining for a compiler instance.
func (c *Compiler) Disasm(f bool) *Compiler {
	c.disasm = f

	return c
}

// CompileString turns a string into a compilation unit. This is a helper function
// around the Compile() operation that removes the need for the caller
// to provide a tokenizer.
func (c *Compiler) CompileString(name string, source string) (*bytecode.ByteCode, error) {
	t := tokenizer.New(source)

	return c.Compile(name, t)
}

// Compile starts a compilation unit, and returns a bytecode
// of the compiled material.
func (c *Compiler) Compile(name string, t *tokenizer.Tokenizer) (*bytecode.ByteCode, error) {
	c.b = bytecode.New(name)
	c.t = t

	c.t.Reset()

	for !c.t.AtEnd() {
		err := c.compileStatement()
		if err != nil {
			return nil, err
		}
	}

	return c.b.Seal(), nil
}

// AddBuiltins adds the builtins for the named package (or prebuilt builtins if the package name
// is empty).
func (c *Compiler) AddBuiltins(pkgname string) bool {
	added := false

	pkg, _ := bytecode.GetPackage(pkgname)
	symV, _ := pkg.Get(data.SymbolsMDKey)
	syms := symV.(*symbols.SymbolTable)

	ui.Debug(ui.CompilerLogger, "### Adding builtin packages to %s package", pkgname)

	functionNames := make([]string, 0)
	for k := range functions.FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	sort.Strings(functionNames)

	for _, name := range functionNames {
		f := functions.FunctionDictionary[name]

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

				ui.Debug(ui.CompilerLogger, "... processing builtin %s", debugName)
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
func (c *Compiler) AddStandard(s *symbols.SymbolTable) bool {
	added := false

	if s == nil {
		return false
	}

	ui.Debug(ui.CompilerLogger, "Adding standard functions to %s (%v)", s.Name, s.ID())

	for name, f := range functions.FunctionDictionary {
		if dot := strings.Index(name, "."); dot < 0 {
			_ = s.SetConstant(name, f.F)
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
	if c.normalizedIdentifiers {
		return strings.ToLower(name)
	}

	return name
}

// normalizeToken performs case-normalization based on the current
// compiler settings for an identifier token.
func (c *Compiler) normalizeToken(t tokenizer.Token) tokenizer.Token {
	if t.IsIdentifier() && c.normalizedIdentifiers {
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
	ui.Debug(ui.CompilerLogger, "Adding compiler packages to %s(%v)", s.Name, s.ID())
	packageMerge.Lock()
	defer packageMerge.Unlock()

	for packageName, packageDictionary := range c.packages {
		// Skip over any metadata
		if strings.HasPrefix(packageName, data.MetadataPrefix) {
			continue
		}

		m := data.NewPackage(packageName)
		keys := packageDictionary.Keys()

		for _, k := range keys {
			v, _ := packageDictionary.Get(k)
			// Do we already have a package of this name defined?
			_, found := s.Get(k)
			if found {
				ui.Debug(ui.CompilerLogger, "Duplicate package %s already in table", k)
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
		data.SetMetadata(m, data.TypeMDKey, data.Package(packageName))
		data.SetMetadata(m, data.ReadonlyMDKey, true)

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
	ui.Debug(ui.CompilerLogger, "+++ Starting auto-import all=%v", all)

	// We do not want to dump tokens during import processing (there are a lot)
	// so turn of token logging during auto-import, and set it back on when done.
	savedTokenLogging := ui.IsActive(ui.TokenLogger)
	savedOptimizerLogging := ui.IsActive(ui.OptimizerLogger)
	savedTraceLogging := ui.IsActive(ui.TraceLogger)

	ui.SetLogger(ui.TokenLogger, false)
	ui.SetLogger(ui.OptimizerLogger, false)
	ui.SetLogger(ui.TraceLogger, false)

	defer func(token, opt, trace bool) {
		ui.SetLogger(ui.TokenLogger, token)
		ui.SetLogger(ui.OptimizerLogger, opt)
		ui.SetLogger(ui.TraceLogger, trace)
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
		for fn := range functions.FunctionDictionary {
			dot := strings.Index(fn, ".")
			if dot > 0 {
				fn = fn[:dot]
				uniqueNames[fn] = true
			}
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
		text := fmt.Sprintf("import \"%s\"", packageName)

		_, err := c.CompileString(packageName, text)
		if err == nil {
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

// Clone makes a new copy of the current compiler. The withLock flag
// indicates if the clone should respect symbol table locking. This
// function supports attribute chaining for a compiler instance.
func (c *Compiler) Clone(withLock bool) *Compiler {
	cx := Compiler{
		activePackageName:     c.activePackageName,
		sourceFile:            c.sourceFile,
		b:                     c.b,
		t:                     c.t,
		s:                     c.s.Clone(withLock),
		rootTable:             c.s.Clone(withLock),
		coercions:             c.coercions,
		constants:             c.constants,
		packageMutex:          sync.Mutex{},
		deferQueue:            []int{},
		normalizedIdentifiers: c.normalizedIdentifiers,
		flags: flagSet{
			extensionsEnabled: c.flags.extensionsEnabled,
		},
		exitEnabled: c.exitEnabled,
	}

	packages := map[string]*data.EgoPackage{}

	c.packageMutex.Lock()
	defer c.packageMutex.Unlock()

	for n, m := range c.packages {
		packageDef := data.NewPackage(n)

		keys := m.Keys()
		for _, k := range keys {
			v, _ := m.Get(k)
			packageDef.Set(k, v)
		}

		packages[n] = packageDef
	}

	// Put the newly created data in the copy of the compiler, with
	// it's own mutex
	cx.packageMutex = sync.Mutex{}
	cx.packages = packages

	return &cx
}
