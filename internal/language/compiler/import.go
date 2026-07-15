package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/packages"
	"github.com/tucats/ego/internal/runtime"
)

// This table contains the map of native (Go) package names that are remapped to the
// equivalent Ego package names. The user can still use the short name, but if they
// reference the full name, it will be internally converted to the short name.
var nativePackageNames = map[string]string{
	"os/exec":                "exec",
	"encode/base64":          "base64",
	"encode/json":            "json",
	"database/sql":           "sql",
	"github.com/google/uuid": "uuid",
	"math/cmplx":             "cmplx",
}

// compileImport compiles an import statement. The "import" keyword has already
// been consumed by the caller. Two syntactic forms are accepted:
//
//	import "packageName"
//	import alias "packageName"
//	import (
//	    "pkg1"
//	    alias2 "pkg2"
//	)
//
// Processing steps for each package:
//  1. Native Go-style paths (e.g. "os/exec") are remapped to their Ego
//     equivalents using the nativePackageNames table.
//  2. If the package is already in the compiled-package cache it is referenced
//     directly with an Import bytecode instruction — no recompilation needed.
//  3. Otherwise the package source file (or directory of .ego files) is read
//     from disk, compiled by a temporary child compiler, executed immediately
//     to register its symbols, and stored in the cache.
//  4. An Import bytecode instruction is emitted for each package so that the
//     runtime can bind the package into the executing program's symbol table.
//
// Imports are only allowed at the top level (blockDepth == 0) unless the
// compiler is in test mode. Circular imports are detected via an import stack.
func (c *Compiler) compileImport() error {
	var (
		err        error
		packageDef *data.Package
	)

	// Unless we are in test mode, constrain imports to
	// only occur at the top-level before blocks are
	// compiled (i.e. not inside a function, etc.)
	if !c.TestMode() {
		if c.blockDepth > 0 {
			return c.compileError(errors.ErrInvalidImport)
		}

		if c.loops != nil {
			return c.compileError(errors.ErrInvalidImport)
		}
	}

	isList := false
	if c.t.IsNext(tokenizer.StartOfListToken) {
		isList = true

		ui.Log(ui.PackageLogger, "pkg.compiler.import", nil)
	}

	parsing := true
	for parsing {
		// Make sure that if this isn't the list format of an import, we only do this once.
		if !isList {
			parsing = false
		}

		var packageName, aliasName string

		fileName := c.t.Next()
		if fileName.Is(tokenizer.EndOfListToken) {
			break
		}

		foundEndOfList := false

		// If this is an identifier and it is followed by a string,
		// save this identifier as the package name.
		if fileName.IsIdentifier() {
			aliasName = fileName.Spelling()
			fileName = c.t.Next()
		}

		for isList && fileName.Is(tokenizer.SemicolonToken) {
			fileName = c.t.Next()
			if fileName.Is(tokenizer.EndOfListToken) {
				foundEndOfList = true

				break
			}
		}

		if foundEndOfList {
			break
		}

		if !fileName.IsString() {
			packageName = fileName.Spelling()
			fileName = c.t.Next()
		}

		// Let's work on the file path. First, Ego allows you to specify native package names
		// that conform to Go standards, which will be masked to the equivalent Ego package name.
		// For example, "os/exec" is remapped to "exec".
		filePath := fileName.Spelling()
		if nativePackageName, found := nativePackageNames[filePath]; found {
			if ui.IsActive(ui.PackageLogger) {
				ui.Log(ui.PackageLogger, "pkg.compiler.remap", ui.A{
					"gopath":  filePath,
					"egopath": nativePackageName})
			}

			filePath = nativePackageName
		}

		// Get the package name from the given string if it wasn't
		// explicitly given in the import statement. If this is
		// a file system name, remove the extension if present.
		if packageName == "" {
			packageName = filepath.Base(filePath)
			if extension := filepath.Ext(packageName); extension != "" {
				packageName = strings.TrimSuffix(packageName, extension)
			}
		}

		if isList && fileName.Is(tokenizer.EndOfListToken) {
			break
		}

		packageName = strings.ToLower(packageName)

		// Figure out the "local name" up front, before any of the branches
		// below run. The local name is what Ego code in *this* file
		// will type to reach the package (e.g. "str" in `str.ToUpper(...)`).
		// If the import statement gave an alias (`import str "strings"`), the
		// local name is that alias. Otherwise it is just the package's own
		// name (`import "strings"` -> local name "strings").
		//
		// This must be computed before the "already cached" shortcut just
		// below, because that shortcut used to skip straight past the alias
		// and always bind the package under its real name. See the comment
		// on that branch for why that was wrong.
		localName := aliasName
		if localName == "" {
			localName = packageName
		}

		if err := c.circularImportCheck(filePath); err != nil {
			return err
		}

		// Does this package already exist? If so, just reference it.
		//
		// "Already exist" means some earlier import (anywhere in this process
		// -- a different file, a different request, even a different alias
		// for the same package) already compiled the package's source and
		// stored the *contents* (its functions, constants, types) in the
		// global package cache (see internal/packages/cache.go). That cache
		// is shared by every goroutine in the process -- the server can be
		// handling many requests at once, all sharing the same cached copy
		// of, say, the "strings" package -- so it would be unsafe to store
		// a single, process-wide alias name there. Aliases are a per-file,
		// per-import-statement choice, not a property of the package itself.
		//
		// So even though the package's contents are reused from the cache,
		// we still must bind THIS import statement's local name (localName,
		// which honors the alias) into the symbol table for the program
		// currently being compiled. The old code used "packageName" (the
		// package's real name) here instead of the alias, so an aliased
		// import of an already-cached package silently lost its alias and
		// the alias name was never defined as a usable symbol.
		if packages.Get(filePath) != nil {
			if ui.IsActive(ui.PackageLogger) {
				ui.Log(ui.PackageLogger, "pkg.compiler.import.found", ui.A{
					"name": filePath})
			}

			c.DefineGlobalSymbol(localName)
			c.b.Emit(bytecode.Import, data.NewList(localName, filePath))

			continue
		}

		// Not a package we've seen. Start building the package. Is it a builtin
		// package we can start with, or do we make a new one from whole cloth...
		// Note that we only try this for names without an explicit path.
		if packageName == filePath {
			packageDef = runtime.AddPackage(packageName)
		}

		if packageDef == nil {
			packageDef = data.NewPackage(packageName, filePath)
			packages.Save(packageDef)
		}

		c.DefineGlobalSymbol(packageName)

		if err := c.ReferenceSymbol(packageName); err != nil {
			return err
		}

		if ui.IsActive(ui.PackageLogger) {
			ui.Log(ui.PackageLogger, "pkg.compiler.importing", ui.A{
				"name": filePath})
		}

		// If this is an import of the package we're currently importing, no work to do.
		if packageName == c.activePackageName {
			continue
		}

		if ui.IsActive(ui.PackageLogger) {
			if !packageDef.Builtins {
				ui.Log(ui.PackageLogger, "pkg.compiler.builtins.add", ui.A{
					"name": fileName.Spelling()})
			} else {
				ui.Log(ui.PackageLogger, "pkg.compiler.builtins.already", ui.A{
					"name": fileName.Spelling()})
			}
		}

		if !packageDef.Builtins {
			if ui.IsActive(ui.PackageLogger) {
				ui.Log(ui.PackageLogger, "pkg.compiler.builtins.none", ui.A{
					"name": fileName.Spelling()})
			}

			c.packageMutex.Lock()
			c.packages[packageName] = data.NewPackage(packageName, fileName.Spelling())
			c.packageMutex.Unlock()
		}

		// Read the imported object as a file path if we haven't already done this
		// for this package.
		savedSourceFile := c.sourceFile
		savedLineNumber := c.t.CurrentLine()

		// Define this package being on the package stack and compile the source.
		if !packageDef.Source {
			text, err := c.readPackageFile(fileName.Spelling())
			if err != nil {
				// If it wasn't found but we did add some builtins, good enough.
				// Skip past the filename that was rejected by c.Readfile()...
				// (This happens for builtin packages like "fmt" that have no
				// lib/packages/<name> directory of extra .ego source on disk --
				// there's nothing more to compile, so we just bind the package
				// that runtime.AddPackage() already built. As elsewhere in this
				// function, bind it under localName so an alias is honored.)
				if !packageDef.IsEmpty() {
					c.t.Advance(1)

					c.b.Emit(bytecode.Import, data.NewList(localName, fileName.Spelling()))

					if !isList || c.t.IsNext(tokenizer.EndOfListToken) {
						break
					}

					continue
				}

				// Nope, import had no effect.
				return err
			}

			c.pushImportPath(filePath, savedSourceFile, int(savedLineNumber))

			err = compileImportSource(packageName, filePath, c, text, fileName, err, packageDef)
			if err != nil {
				return err
			}

			// If no errors, pop this back off the import package stack
			c.popImportPath()
		} else if ui.IsActive(ui.PackageLogger) {
			ui.Log(ui.PackageLogger, "pkg.compiler.import.already", ui.A{
				"name": fileName.Spelling()})
		}

		// Rewrite the package to the cache with all up-to-date info.
		packages.Save(packageDef)

		c.sourceFile = savedSourceFile

		// Now that the package is in the cache, add the instruction to the active
		// program to import that cached info at runtime. As above, we bind the
		// package under localName (the alias, if one was given) rather than
		// packageName, so that `import str "strings"` makes the package
		// reachable as "str", not "strings".
		c.DefineGlobalSymbol(localName)
		c.b.Emit(bytecode.Import, data.NewList(localName, filePath))

		// If this is a list, keep going until we run out of tokens. Otherwise, done.
		if !isList {
			break
		}

		// Eat the semicolon if present
		if c.t.IsNext(tokenizer.SemicolonToken) {
			continue
		}

		if c.t.Next().Is(tokenizer.EndOfListToken) {
			break
		}
	}

	return err
}

// compileImportSource compiles the Ego source text of a package that is being
// imported for the first time. It creates a dedicated child compiler, parses
// every statement in the package text, then runs the resulting bytecode in a
// fresh symbol table so that all package-level declarations (types, functions,
// constants) are registered. The populated symbol table is attached to the
// package definition so subsequent imports can reuse it without recompilation.
func compileImportSource(packageName string, filePath string, c *Compiler, text string, fileName tokenizer.Token, err error, packageDef *data.Package) error {
	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.compiler.source", ui.A{
			"name": packageName})
	}

	importCompiler := New(tokenizer.ImportToken.Spelling() + " " + filePath).
		SetRoot(c.rootTable).
		SetTestMode(c.flags.testMode)

	importCompiler.b = bytecode.New(tokenizer.ImportToken.Spelling() + " " + filepath.Base(filePath))
	importCompiler.t = tokenizer.New(text, true)
	importCompiler.activePackageName = packageName
	importCompiler.sourceFile = c.sourceFile
	importCompiler.flags.debuggerActive = c.flags.debuggerActive
	importCompiler.importStack = append([]importElement{}, c.importStack...)

	defer importCompiler.Close()

	// Scan the package definition to see if there are any package types we need to elevate
	// to the compiler's type list.
	for _, key := range packageDef.Keys() {
		item, _ := packageDef.Get(key)
		if itemType, ok := item.(*data.Type); ok {
			importCompiler.types[key] = itemType
		}
	}

	for !importCompiler.t.AtEnd() {
		if err := importCompiler.compileStatement(); err != nil {
			return err
		}
	}

	// If we are disassembling, do it now for the imported definitions.
	importCompiler.b.Disasm(false)

	// If after the import we ended with mismatched block markers, complain
	if importCompiler.blockDepth != 0 {
		return c.compileError(errors.ErrMissingEndOfBlock, packageName)
	}

	// The import will have generated code that must be run to actually register
	// package contents.
	importSymbols := symbols.NewChildSymbolTable(tokenizer.ImportToken.Spelling()+" "+fileName.Spelling(), c.rootTable)
	ctx := bytecode.NewContext(importSymbols, importCompiler.b)

	if err = ctx.Run(); err != nil && !errors.Equals(err, errors.ErrStop) {
		return err
	}

	// Scoop up all the items in the package definition and add them to the package
	// symbol table. This is also a good time to clean out the items in the package
	// definition we don't really need any more (placed there during initialization)
	keys := packageDef.Keys()
	for _, key := range keys {
		if !strings.HasPrefix(key, defs.ReadonlyVariablePrefix) {
			item, _ := packageDef.Get(key)
			importSymbols.SetAlways(key, item)
		}
	}

	// The symbol table is now populated with the imported symbols, so save it in the package.
	packageDef.Set(data.SymbolsMDKey, importSymbols)
	packageDef.SetImported(true)

	// If the package declares an init() function, run it now, exactly once, the
	// first time the package is imported -- mirroring Go's package init() semantics.
	if err := runPackageInit(packageName, importSymbols, packageDef); err != nil {
		return err
	}

	return nil
}

// runPackageInit invokes a package's init() function, if it has one and it has
// not already been run. This is called once, immediately after a package's
// source has been compiled and executed for the first time; later imports of
// the same package reuse the cached *data.Package and never reach this code
// again, so packageDef.Initialized is mostly a defensive guard against
// re-entrant/concurrent imports of the same package.
func runPackageInit(packageName string, importSymbols *symbols.SymbolTable, packageDef *data.Package) error {
	if packageDef.IsInitialized() {
		return nil
	}

	initFn, found := importSymbols.Get(defs.InitFunctionName)
	if !found {
		return nil
	}

	fn, ok := initFn.(*bytecode.ByteCode)
	if !ok {
		return nil
	}

	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.compiler.init.run", ui.A{
			"name": packageName})
	}

	initSymbols := symbols.NewChildSymbolTable(defs.InitFunctionName+" "+packageName, importSymbols)
	initSymbols.SetAlways(defs.ArgumentListVariable, data.NewArrayFromInterfaces(data.InterfaceType))

	// The compiled init() body starts with an InPackage opcode (like every
	// package-level function) that resolves packageName back to this same
	// *data.Package so unqualified references to sibling package symbols
	// work. Normally that binding is created by the Import opcode, which
	// hasn't run yet at this point -- init() runs before the package is
	// even fully registered with the caller -- so bind it here directly.
	initSymbols.SetAlways(packageName, packageDef)

	ctx := bytecode.NewContext(initSymbols, fn)

	if err := ctx.Run(); err != nil && !errors.Equals(err, errors.ErrStop) {
		return err
	}

	packageDef.SetInitialized(true)

	return nil
}

// readPackageFile reads the text from a file into a string.
func (c *Compiler) readPackageFile(name string) (string, error) {
	s, err := c.directoryContents(name)
	if err == nil {
		return s, nil
	}

	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.compiler.read.file", ui.A{
			"path": name})
	}

	// Not a directory, try to read the file
	fn := name

	content, e2 := os.ReadFile(fn)
	if e2 != nil {
		content, e2 = os.ReadFile(name + defs.EgoFilenameExtension)
		if e2 != nil {
			// Path name did not resolve. Get the Ego path and try
			// variations on that.
			r := os.Getenv(defs.EgoPathEnv)
			if r == "" {
				r = settings.Get(defs.EgoPathSetting)
			}

			// Try to see if it's in the lib directory under EGO path
			path := ""
			if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
				path = libpath
			} else {
				path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
			}

			path = filepath.Join(path, "packages")
			fn = filepath.Join(path, name+defs.EgoFilenameExtension)
			content, e2 = os.ReadFile(fn)

			// Nope, see if it's in the path relative to EGO path
			if e2 != nil {
				fn = filepath.Join(r, name+defs.EgoFilenameExtension)
				content, e2 = os.ReadFile(fn)
			}

			if e2 != nil {
				c.t.Advance(-1)

				return "", c.compileError(e2)
			}
		} else {
			fn = name + defs.EgoFilenameExtension
		}
	}

	if e2 == nil {
		c.sourceFile = fn
	}

	// Convert []byte to string. Prefix each source file with a reset of
	// the line number in the aggregate source string.
	return "@line 1  ;\n" + string(content), nil
}

// directoryContents reads all the files in a directory into a single string.
func (c *Compiler) directoryContents(name string) (string, error) {
	var (
		b    strings.Builder
		path string
	)

	if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
		path = libpath
	} else {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	path = filepath.Join(path, "packages")
	dirname := name

	if !strings.HasPrefix(dirname, path) {
		dirname = filepath.Join(path, name)

		if ui.IsActive(ui.PackageLogger) {
			ui.Log(ui.PackageLogger, "pkg.compiler.apply.path", ui.A{
				"path": dirname})
		}
	}

	fi, err := os.ReadDir(dirname)
	if err != nil {
		return "", errors.New(err)
	}

	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.compiler.dir.read", ui.A{
			"path": name})
	}

	if len(fi) == 0 {
		ui.Log(ui.PackageLogger, "pkg.compiler.dir.empty", nil)
	}

	// For all the items that aren't directories themselves, and
	// for file names ending in ".Ego", read them into the master
	// result string. Note that recursive directory reading is
	// not supported.
	savedSourceFile := c.sourceFile
	defer func() {
		c.sourceFile = savedSourceFile
	}()

	for _, f := range fi {
		if !f.IsDir() && strings.HasSuffix(f.Name(), defs.EgoFilenameExtension) {
			fileName := filepath.Join(dirname, f.Name())

			t, err := c.readPackageFile(fileName)
			if err != nil {
				return "", err
			}

			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}

// pushImportPath records the given package path on the import stack. The
// stack is used by circularImportCheck to detect and report import cycles.
// Each entry also stores the source location so that the cycle error message
// can show exactly where each import was requested.
func (c *Compiler) pushImportPath(path string, source string, line int) {
	c.importStack = append(c.importStack, importElement{
		err:  errors.ErrCircularImport.Clone().In(source).At(line, 0),
		path: path,
	})
}

// popImportPath removes the most-recently-pushed entry from the import stack,
// indicating that the compilation of that package is complete.
func (c *Compiler) popImportPath() {
	if len(c.importStack) == 0 {
		return
	}

	c.importStack = c.importStack[:len(c.importStack)-1]
}

// circularImportCheck scans the current import stack to see if filePath is
// already being compiled. If it is, the import would be circular (A imports B
// which imports A again), so an ErrCircularImport error is returned with a
// human-readable trace of the full import chain.
func (c *Compiler) circularImportCheck(filePath string) error {
	// Check for circular imports.
	for _, importedPackage := range c.importStack {
		if importedPackage.path == filePath {
			importPath := "\n"

			for index, path := range c.importStack {
				text := fmt.Sprintf("%-30s import %s", errors.New(path.err).GetLocation(), strconv.Quote(path.path))

				importPath += text
				if index < len(c.importStack)-1 {
					importPath += "\n"
				}
			}

			return c.compileError(errors.ErrCircularImport).Context(importPath)
		}
	}

	return nil
}
