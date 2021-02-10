package compiler

import (
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
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
	EgoPathSetting    = "ego.path"
)

// RequiredPackages is the list of packages that are always imported, regardless
// of user import statements or auto-import profile settings.
var RequiredPackages []string = []string{
	"util",
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

// FunctionDictionary is a list of functions and the bytecode or native function pointer.
type FunctionDictionary map[string]interface{}

// PackageDictionary is a list of packages each with a FunctionDictionary.
type PackageDictionary map[string]FunctionDictionary

// Compiler is a structure defining what we know about the compilation.
type Compiler struct {
	PackageName          string
	SourceFile           string
	b                    *bytecode.ByteCode
	t                    *tokenizer.Tokenizer
	s                    *symbols.SymbolTable
	loops                *Loop
	coercions            []*bytecode.ByteCode
	constants            []string
	deferQueue           []int
	packages             PackageDictionary
	blockDepth           int
	statementCount       int
	LowercaseIdentifiers bool
	extensionsEnabled    bool
	exitEnabled          bool // Only true in interactive mode
}

// New creates a new compiler instance.
func New() *Compiler {
	cInstance := Compiler{
		b:                    nil,
		t:                    nil,
		s:                    &symbols.SymbolTable{Name: "compile-unit"},
		constants:            make([]string, 0),
		deferQueue:           make([]int, 0),
		packages:             PackageDictionary{},
		LowercaseIdentifiers: false,
		extensionsEnabled:    persistence.GetBool(ExtensionsSetting),
	}

	return &cInstance
}

// If set to true, the compiler allows the EXIT statement.
func (c *Compiler) ExitEnabled(b bool) *Compiler {
	c.exitEnabled = b

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
		err := c.Statement()
		if err != nil {
			return nil, err
		}
	}

	// Merge in any package definitions
	c.AddPackageToSymbols(c.b.Symbols)

	// Also merge in any other symbols created for this function
	c.b.Symbols.Merge(c.Symbols())

	return c.b, nil
}

// AddBuiltins adds the builtins for the named package (or prebuilt builtins if the package name
// is empty).
func (c *Compiler) AddBuiltins(pkgname string) bool {
	added := false

	for name, f := range functions.FunctionDictionary {
		if dot := strings.Index(name, "."); dot >= 0 {
			f.Pkg = name[:dot]
			name = name[dot+1:]
		}

		if f.Pkg == pkgname {
			if f.F != nil {
				_ = c.AddPackageFunction(pkgname, name, f.F)
				added = true
			} else {
				_ = c.AddPackageValue(pkgname, name, f.V)
			}
		}
	}

	return added
}

// Get retrieves a compile-time symbol value.
func (c *Compiler) Get(name string) (interface{}, bool) {
	return c.s.Get(name)
}

// Normalize performs case-normalization based on the current
// compiler settings.
func (c *Compiler) Normalize(name string) string {
	if c.LowercaseIdentifiers {
		return strings.ToLower(name)
	}

	return name
}

// AddPackageFunction adds a new package function to the compiler's package dictionary. If the
// package name does not yet exist, it is created. The function name and interface are then used
// to add an entry for that package.
func (c *Compiler) AddPackageFunction(pkgname string, name string, function interface{}) *errors.EgoError {
	fd, found := c.packages[pkgname]
	if !found {
		fd = FunctionDictionary{}
		fd[datatypes.MetadataKey] = map[string]interface{}{
			datatypes.TypeMDKey:     "dictionary",
			datatypes.ReadonlyMDKey: true,
		}
	}

	if _, found := fd[name]; found {
		return c.NewError(errors.FunctionAlreadyExistsError)
	}

	fd[name] = function
	c.packages[pkgname] = fd

	return nil
}

// AddPackageFunction adds a new package function to the compiler's package dictionary. If the
// package name does not yet exist, it is created. The function name and interface are then used
// to add an entry for that package.
func (c *Compiler) AddPackageValue(pkgname string, name string, value interface{}) *errors.EgoError {
	fd, found := c.packages[pkgname]
	if !found {
		fd = FunctionDictionary{}
		datatypes.SetMetadata(fd, datatypes.TypeMDKey, "package")
		datatypes.SetMetadata(fd, datatypes.ReadonlyMDKey, true)
	}

	if _, found := fd[name]; found {
		return c.NewError(errors.FunctionAlreadyExistsError)
	}

	fd[name] = value
	c.packages[pkgname] = fd

	return nil
}

// AddPackageToSymbols adds all the defined packages for this compilation to the named symbol table.
func (c *Compiler) AddPackageToSymbols(s *symbols.SymbolTable) {
	for pkgname, dict := range c.packages {
		m := map[string]interface{}{}

		for k, v := range dict {
			// If the package name is empty, we add the individual items
			if pkgname == "" {
				_ = s.SetConstant(k, v)
			} else {
				// Otherwise, copy the entire map
				m[k] = v
			}
		}
		// Make sure the package is marked as readonly so the user can't modify
		// any function definitions, etc. that are built in.
		datatypes.SetMetadata(m, datatypes.TypeMDKey, "package")
		datatypes.SetMetadata(m, datatypes.ReadonlyMDKey, true)

		if pkgname != "" {
			_ = s.SetConstant(pkgname, m)
		}
	}
}

// StatementEnd returns true when the next token is
// the end-of-statement boundary.
func (c *Compiler) StatementEnd() bool {
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
		if err == nil {
			firstError = err
		}
	}

	c.b = savedBC
	c.t = savedT
	c.SourceFile = savedSource

	return firstError
}
