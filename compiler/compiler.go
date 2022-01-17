package compiler

import (
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

const (
	indexLoopType       = 1
	rangeLoopType       = 2
	forLoopType         = 3
	conditionalLoopType = 4

	ExtensionsSetting = "ego.compiler.extensions"
	EgoPathSetting    = defs.EgoPathSetting
)

// RequiredPackages is the list of packages that are always imported, regardless
// of user import statements or auto-import profile settings.
var RequiredPackages []string = []string{
	"os",
	"profile",
}

// Loop is a structure that defines a loop type.
type Loop struct {
	Parent *Loop
	Type   int
	// Fixup locations for break or continue statements in a
	// loop. These are the addresses that must be fixed up with
	// a target address pointing to exit point or start of the loop.
	breaks    []int
	continues []int
}

// PackageDictionary is a list of packages each with a function dictionary.
type PackageDictionary struct {
	Mutex   sync.Mutex
	Package map[string]datatypes.EgoPackage
}

// Compiler is a structure defining what we know about the compilation.
type Compiler struct {
	PackageName          string
	SourceFile           string
	b                    *bytecode.ByteCode
	t                    *tokenizer.Tokenizer
	s                    *symbols.SymbolTable
	RootTable            *symbols.SymbolTable
	loops                *Loop
	coercions            []*bytecode.ByteCode
	constants            []string
	deferQueue           []int
	packages             PackageDictionary
	Types                map[string]datatypes.Type
	functionDepth        int
	blockDepth           int
	statementCount       int
	disasm               bool
	LowercaseIdentifiers bool
	extensionsEnabled    bool
	exitEnabled          bool // Only true in interactive mode
}

// New creates a new compiler instance.
func New(name string) *Compiler {
	cInstance := Compiler{
		b:          nil,
		t:          nil,
		s:          symbols.NewRootSymbolTable(name),
		constants:  make([]string, 0),
		deferQueue: make([]int, 0),
		Types:      map[string]datatypes.Type{},
		packages: PackageDictionary{
			Mutex:   sync.Mutex{},
			Package: map[string]datatypes.EgoPackage{},
		},
		LowercaseIdentifiers: false,
		extensionsEnabled:    persistence.GetBool(ExtensionsSetting),
		RootTable:            &symbols.RootSymbolTable,
	}

	return &cInstance
}

// Override the default root symbol table for this compilation. This determines
// where package names are stored/found, for example. This is overridden by the
// web service handlers as they have per-call instances of root.
func (c *Compiler) SetRoot(s *symbols.SymbolTable) *Compiler {
	c.RootTable = s
	c.s.Parent = s

	return c
}

// If set to true, the compiler allows the EXIT statement.
func (c *Compiler) ExitEnabled(b bool) *Compiler {
	c.exitEnabled = b

	return c
}

// Set the given symbol table as the default symbol table for
// compilation. This mostly affects how builtins are processed.
func (c *Compiler) WithSymbols(s *symbols.SymbolTable) *Compiler {
	c.s = s

	return c
}

// If set to true, the compiler allows the PRINT, TRY/CATCH, etc. statements.
func (c *Compiler) ExtensionsEnabled(b bool) *Compiler {
	c.extensionsEnabled = b

	return c
}

// WithTokens supplies the token stream to a compiler.
func (c *Compiler) WithTokens(t *tokenizer.Tokenizer) *Compiler {
	c.t = t

	return c
}

// WithNormalization sets the normalization flag and can be chained
// onto a compiler.New...() operation.
func (c *Compiler) WithNormalization(f bool) *Compiler {
	c.LowercaseIdentifiers = f

	return c
}

// Disasm sets the disassembler flag and can be chained
// onto a compiler.New...() operation.
func (c *Compiler) Disasm(f bool) *Compiler {
	c.disasm = f

	return c
}

// CompileString turns a string into a compilation unit. This is a helper function
// around the Compile() operation that removes the need for the caller
// to provide a tokenizer.
func (c *Compiler) CompileString(name string, source string) (*bytecode.ByteCode, *errors.EgoError) {
	t := tokenizer.New(source)

	return c.Compile(name, t)
}

// Compile starts a compilation unit, and returns a bytecode
// of the compiled material.
func (c *Compiler) Compile(name string, t *tokenizer.Tokenizer) (*bytecode.ByteCode, *errors.EgoError) {
	c.b = bytecode.New(name)
	c.t = t

	c.t.Reset()

	for !c.t.AtEnd() {
		err := c.compileStatement()
		if !errors.Nil(err) {
			return nil, err
		}
	}

	return c.b, nil
}

// AddBuiltins adds the builtins for the named package (or prebuilt builtins if the package name
// is empty).
func (c *Compiler) AddBuiltins(pkgname string) bool {
	added := false

	compilerName := "compiler"
	if c.s != nil {
		compilerName = c.s.Name
	}

	ui.Debug(ui.CompilerLogger, "### Adding builtin packages to %s compilation unit", compilerName)

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
			if ui.LoggerIsActive(ui.CompilerLogger) {
				debugName := name
				if f.Pkg != "" {
					debugName = f.Pkg + "." + name
				}

				ui.Debug(ui.CompilerLogger, "... processing builtin %s", debugName)
			}

			added = true

			if pkgname == "" && c.s != nil {
				_ = c.s.SetAlways(name, f.F)
			} else {
				if f.F != nil {
					_ = c.addPackageFunction(pkgname, name, f.F)
				} else {
					_ = c.addPackageValue(pkgname, name, f.V)
				}
			}
		}
	}

	// If we added one or more functions, update the package definition
	// in the root symbol table for this builtin package.
	if added {
		_ = c.RootTable.Root().SetAlways(pkgname, c.packages.Package[pkgname])
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

	ui.Debug(ui.CompilerLogger, "Adding standard functions to %s (%v)", s.Name, s.ID)

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
	if c.LowercaseIdentifiers {
		return strings.ToLower(name)
	}

	return name
}

// addPackageFunction adds a new package function to the compiler's package dictionary. If the
// package name does not yet exist, it is created. The function name and interface are then used
// to add an entry for that package.
func (c *Compiler) addPackageFunction(pkgname string, name string, function interface{}) *errors.EgoError {
	c.packages.Mutex.Lock()
	defer c.packages.Mutex.Unlock()

	fd, found := c.packages.Package[pkgname]
	if !found || fd.IsEmpty() {
		fx, found := c.RootTable.Get(pkgname)
		if !found {
			fd = datatypes.NewPackage(pkgname)
		} else {
			fd = fx.(datatypes.EgoPackage)
			c.packages.Package[pkgname] = fd
		}

		datatypes.SetMetadata(fd, datatypes.TypeMDKey, datatypes.Package(pkgname))
		datatypes.SetMetadata(fd, datatypes.ReadonlyMDKey, true)
	}

	if _, found := fd.Get(name); found {
		return c.newError(errors.ErrFunctionAlreadyExists)
	}

	fd.Set(name, function)

	c.packages.Package[pkgname] = fd

	// Keep the global symbol version in sync also. If it exists already, get its values
	// and merge into this new definition, then write out the value.
	// @tomcole This MAY be able to completly supplan the compiler package structures at
	// some point in the future.
	if oldPackage, found := symbols.RootSymbolTable.Get(pkgname); found {
		fd.Merge(oldPackage.(datatypes.EgoPackage))
	}

	_ = symbols.RootSymbolTable.SetAlways(pkgname, fd)

	ui.Debug(ui.CompilerLogger, "... added function %s", name)

	return nil
}

// AddPackageFunction adds a new package function to the compiler's package dictionary. If the
// package name does not yet exist, it is created. The function name and interface are then used
// to add an entry for that package.
func (c *Compiler) addPackageValue(pkgname string, name string, value interface{}) *errors.EgoError {
	c.packages.Mutex.Lock()
	defer c.packages.Mutex.Unlock()

	fd, found := c.packages.Package[pkgname]
	if !found || fd.IsEmpty() {
		fx, found := c.RootTable.Get(pkgname)
		if !found {
			fd = datatypes.NewPackage(pkgname)
		} else {
			fd = fx.(datatypes.EgoPackage)
			c.packages.Package[pkgname] = fd
		}

		datatypes.SetMetadata(fd, datatypes.TypeMDKey, datatypes.Package(pkgname))
		datatypes.SetMetadata(fd, datatypes.ReadonlyMDKey, true)
	}

	if _, found := fd.Get(name); found {
		return c.newError(errors.ErrFunctionAlreadyExists)
	}

	fd.Set(name, value)

	c.packages.Package[pkgname] = fd

	return nil
}

func (c *Compiler) SetInteractive(b bool) {
	if b {
		c.functionDepth++
	}
}

var packageMerge sync.Mutex

// AddPackageToSymbols adds all the defined packages for this compilation
// to the given symbol table.
func (c *Compiler) AddPackageToSymbols(s *symbols.SymbolTable) {
	ui.Debug(ui.CompilerLogger, "Adding compiler packages to %s(%v)", s.Name, s.ID)
	packageMerge.Lock()
	defer packageMerge.Unlock()

	for packageName, packageDictionary := range c.packages.Package {
		// Skip over any metadata
		if strings.HasPrefix(packageName, datatypes.MetadataPrefix) {
			continue
		}

		m := datatypes.NewPackage(packageName)
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
		datatypes.SetMetadata(m, datatypes.TypeMDKey, datatypes.Package(packageName))
		datatypes.SetMetadata(m, datatypes.ReadonlyMDKey, true)

		if packageName != "" {
			_ = s.SetAlways(packageName, m)
		}
	}
}

// isStatementEnd returns true when the next token is
// the end-of-statement boundary.
func (c *Compiler) isStatementEnd() bool {
	next := c.t.Peek(1)

	return util.InList(next, tokenizer.EndOfTokens, ";", "}")
}

// Symbols returns the symbol table map from compilation.
func (c *Compiler) Symbols() *symbols.SymbolTable {
	return c.s
}

// AutoImport arranges for the import of built-in packages. The
// parameter indicates if all available packages (including those
// found in the ego path) are imported, versus just essential
// packages like "util".
func (c *Compiler) AutoImport(all bool) *errors.EgoError {
	ui.Debug(ui.CompilerLogger, "+++ Starting auto-import all=%v", all)

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
		for _, p := range RequiredPackages {
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
	savedSource := c.SourceFile

	var firstError *errors.EgoError

	for _, packageName := range sortedPackageNames {
		text := "import " + packageName

		_, err := c.CompileString(packageName, text)
		if errors.Nil(err) {
			firstError = err
		}
	}

	c.b = savedBC
	c.t = savedT
	c.SourceFile = savedSource

	return firstError
}

func (c *Compiler) Clone(withLock bool) *Compiler {
	cx := Compiler{
		PackageName:          c.PackageName,
		SourceFile:           c.SourceFile,
		b:                    c.b,
		t:                    c.t,
		s:                    c.s.Clone(withLock),
		RootTable:            c.s.Clone(withLock),
		coercions:            c.coercions,
		constants:            c.constants,
		deferQueue:           []int{},
		LowercaseIdentifiers: c.LowercaseIdentifiers,
		extensionsEnabled:    c.extensionsEnabled,
		exitEnabled:          c.exitEnabled,
	}

	packages := PackageDictionary{
		Mutex:   sync.Mutex{},
		Package: map[string]datatypes.EgoPackage{},
	}

	c.packages.Mutex.Lock()
	defer c.packages.Mutex.Unlock()

	for n, m := range c.packages.Package {
		packData := datatypes.NewPackage(n)

		keys := m.Keys()
		for _, k := range keys {
			v, _ := m.Get(k)
			packData.Set(k, v)
		}

		packages.Package[n] = packData
	}

	// Put the newly created data in the copy of the compiler, with
	// it's own mutex
	cx.packages.Mutex = sync.Mutex{}
	cx.packages.Package = packages.Package

	return &cx
}
