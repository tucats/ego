package compiler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// This table contains the map of native (Go) package names that are remapped to the
// equivalent Ego package names.
var nativePackageNames = map[string]string{
	"os/exec":       "exec",
	"encode/base64": "base64",
	"encode/json":   "json",
}

// compileImport handles the import statement.
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
			return c.error(errors.ErrInvalidImport)
		}

		if c.loops != nil {
			return c.error(errors.ErrInvalidImport)
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
		if fileName == tokenizer.EndOfListToken {
			break
		}

		foundEndOfList := false

		// If this is an identifier and it is followed by a string,
		// save this identifier as the package name.
		if fileName.IsIdentifier() {
			aliasName = fileName.Spelling()
			fileName = c.t.Next()
		}

		for isList && (fileName == tokenizer.SemicolonToken) {
			fileName = c.t.Next()
			if fileName == tokenizer.EndOfListToken {
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
			ui.Log(ui.PackageLogger, "pkg.compiler.remap", ui.A{
				"gopath":  filePath,
				"egopath": nativePackageName})

			filePath = nativePackageName
		}

		// Get the package name from the given string if it wasn't
		// explicitly given in the import statement. If this is
		// a file system name, remove the extension if present.
		if packageName == "" {
			packageName = filepath.Base(filePath)
			if filepath.Ext(packageName) != "" {
				packageName = packageName[:len(filepath.Ext(packageName))]
			}
		}

		if isList && fileName == tokenizer.EndOfListToken {
			break
		}

		packageName = strings.ToLower(packageName)
		packageDef, _ = bytecode.GetPackage(packageName)

		wasBuiltin := packageDef.Builtins
		wasImported := packageDef.Source

		c.DefineGlobalSymbol(packageName)

		if err := c.ReferenceSymbol(packageName); err != nil {
			return err
		}

		ui.Log(ui.PackageLogger, "pkg.compiler.importing", ui.A{
			"name": filePath})

		// If this is an import of the package we're currently importing, no work to do.
		if packageName == c.activePackageName {
			continue
		}

		// Special case -- if we did not do an auto-import on intialization, then
		// we need to rebuild the entire package now that it's explicitly imported.
		if !settings.GetBool(defs.AutoImportSetting) {
			if fpI, _ := symbols.RootSymbolTable.Get("__AddPackages"); fpI != nil {
				fp := fpI.(func(name string, s *symbols.SymbolTable))
				fp(packageName, &symbols.RootSymbolTable)
			}

			// If it's already in the cache, use the cached one, else we'll need
			// to create a new one.
			if p, found := bytecode.GetPackage(packageDef.Name); found {
				packageDef = p
			} else {
				pkg, _ := c.Get(packageDef.Name)
				packageDef = pkg.(*data.Package)
			}
		}

		if !packageDef.Builtins {
			ui.Log(ui.PackageLogger, "pkg.compiler.builtins.add", ui.A{
				"name": fileName.Spelling()})
		} else {
			ui.Log(ui.PackageLogger, "pkg.compiler.builtins.already", ui.A{
				"name": fileName.Spelling()})
		}

		if !packageDef.Builtins {
			// The nil in the packages list just prevents this from being read again
			// if it was already processed once.
			ui.Log(ui.PackageLogger, "pkg.compiler.builtins.none", ui.A{
				"name": fileName.Spelling()})

			c.packageMutex.Lock()
			c.packages[packageName] = data.NewPackage(fileName.Spelling())
			c.packageMutex.Unlock()
		}

		// Read the imported object as a file path if we haven't already done this
		// for this package.
		savedSourceFile := c.sourceFile

		if !packageDef.Source {
			text, err := c.readPackageFile(fileName.Spelling())
			if err != nil {
				// If it wasn't found but we did add some builtins, good enough.
				// Skip past the filename that was rejected by c.Readfile()...
				if !packageDef.IsEmpty() {
					c.t.Advance(1)

					if !isList || c.t.IsNext(tokenizer.EndOfListToken) {
						break
					}

					continue
				}

				// Nope, import had no effect.
				return err
			}

			ui.Log(ui.PackageLogger, "pkg.compiler.source", ui.A{
				"name": packageName})

			importCompiler := New(tokenizer.ImportToken.Spelling() + " " + filePath).SetRoot(c.rootTable).SetTestMode(c.flags.testMode)
			importCompiler.b = bytecode.New(tokenizer.ImportToken.Spelling() + " " + filepath.Base(filePath))
			importCompiler.t = tokenizer.New(text, true)
			importCompiler.activePackageName = packageName
			importCompiler.sourceFile = c.sourceFile
			importCompiler.flags.debuggerActive = c.flags.debuggerActive

			defer importCompiler.Close()

			for !importCompiler.t.AtEnd() {
				if err := importCompiler.compileStatement(); err != nil {
					return err
				}
			}

			importCompiler.b.Emit(bytecode.PopPackage, packageName)

			// If we are disassembling, do it now for the imported definitions.
			importCompiler.b.Disasm()

			// If after the import we ended with mismatched block markers, complain
			if importCompiler.blockDepth != 0 {
				return c.error(errors.ErrMissingEndOfBlock, packageName)
			}

			// The import will have generate code that must be run to actually register
			// package contents.
			importSymbols := symbols.NewChildSymbolTable(tokenizer.ImportToken.Spelling()+" "+fileName.Spelling(), c.rootTable)
			ctx := bytecode.NewContext(importSymbols, importCompiler.b)

			if err = ctx.Run(); err != nil && !errors.Equals(err, errors.ErrStop) {
				break
			}

			packageDef.SetImported(true)
		} else {
			ui.Log(ui.PackageLogger, "pkg.compiler.import.already", ui.A{
				"name": fileName.Spelling()})
		}

		c.sourceFile = savedSourceFile

		// Rewrite the package if we've added stuff to it.
		if wasImported != packageDef.Source || wasBuiltin != packageDef.Builtins {
			if ui.IsActive(ui.PackageLogger) {
				ui.Log(ui.PackageLogger, "pkg.compiler.import.update", ui.A{
					"name": fileName.Spelling()})

				ui.Log(ui.PackageLogger, "pkg.compiler.import.keys", ui.A{
					"keys": packageDef.Keys()})
			}

			symbols.RootSymbolTable.SetAlways(packageName, packageDef)
		}

		// Now that the package is in the cache, add the instruction to the active
		// program to import that cached info at runtime.
		c.b.Emit(bytecode.Import, packageName)

		// If there was an alias created for this package, store it in the symbol table
		if aliasName != "" {
			c.b.Emit(bytecode.CreateAndStore, data.NewList(aliasName, packageDef))
		}

		// If this is a list, keep going until we run out of tokens. Otherwise, done.
		if !isList {
			break
		}

		// Eat the semicolon if present
		if c.t.IsNext(tokenizer.SemicolonToken) {
			continue
		}

		if c.t.Next() == tokenizer.EndOfListToken {
			break
		}
	}

	return err
}

// readPackageFile reads the text from a file into a string.
func (c *Compiler) readPackageFile(name string) (string, error) {
	s, err := c.directoryContents(name)
	if err == nil {
		return s, nil
	}

	ui.Log(ui.PackageLogger, "pkg.compiler.read.file", ui.A{
		"path": name})

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

			fn = filepath.Join(path, name+defs.EgoFilenameExtension)
			content, e2 = os.ReadFile(fn)

			// Nope, see if it's in the path relative to EGO path
			if e2 != nil {
				fn = filepath.Join(r, name+defs.EgoFilenameExtension)
				content, e2 = os.ReadFile(fn)
			}

			if e2 != nil {
				c.t.Advance(-1)

				return "", c.error(e2)
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
	return "@line 0; " + string(content), nil
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

	dirname := name
	if !strings.HasPrefix(dirname, path) {
		dirname = filepath.Join(path, name)

		ui.Log(ui.PackageLogger, "pkg.compiler.apply.path", ui.A{
			"path": dirname})
	}

	fi, err := os.ReadDir(dirname)
	if err != nil {
		return "", errors.New(err)
	}

	ui.Log(ui.PackageLogger, "pkg.compiler.dir.read", ui.A{
		"path": name})

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
