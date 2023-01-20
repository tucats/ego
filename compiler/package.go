package compiler

import (
	"io/ioutil"
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

// compilePackage compiles a package statement.
func (c *Compiler) compilePackage() error {
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingPackageName)
	}

	name := c.t.Next()
	if !name.IsIdentifier() {
		return c.error(errors.ErrInvalidPackageName, name)
	}

	name = c.normalizeToken(name)

	if (c.activePackageName != "") && (c.activePackageName != name.Spelling()) {
		return c.error(errors.ErrPackageRedefinition)
	}

	c.activePackageName = name.Spelling()

	c.b.Emit(bytecode.PushPackage, name)

	return nil
}

// compileImport handles the import statement.
func (c *Compiler) compileImport() error {
	var err error

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

		ui.Log(ui.CompilerLogger, "*** Processing import list")
	}

	parsing := true
	for parsing {
		// Make sure that if this isn't the list format of an import, we only do this once.
		if !isList {
			parsing = false
		}

		var packageName string

		fileName := c.t.Next()
		if fileName == tokenizer.EndOfListToken {
			break
		}

		for isList && (fileName == tokenizer.SemicolonToken) {
			fileName = c.t.Next()
		}

		if !fileName.IsString() {
			packageName = fileName.Spelling()
			fileName = c.t.Next()
		}

		filePath := fileName.Spelling()
		// Get the package name from the given string if it wasn't
		// explicitly given in the import statement.If this is
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
		packageDef, _ := bytecode.GetPackage(packageName)

		wasBuiltin := packageDef.Builtins()
		wasImported := packageDef.HasImportedSource()

		ui.Log(ui.CompilerLogger, "*** Importing package \"%s\"", fileName)

		// If this is an import of the package we're currently importing, no work to do.
		if packageName == c.activePackageName {
			continue
		}

		if !packageDef.Builtins() {
			packageDef.SetBuiltins(c.AddBuiltins(packageName))

			ui.Log(ui.CompilerLogger, "+++ Added builtins for package "+fileName.Spelling())
		} else {
			ui.Log(ui.CompilerLogger, "--- Builtins already initialized for package "+fileName.Spelling())
		}

		if !packageDef.Builtins() {
			// The nil in the packages list just prevents this from being read again
			// if it was already processed once.
			ui.Log(ui.CompilerLogger, "+++ No builtins for package "+fileName.Spelling())
			c.packageMutex.Lock()
			c.packages[packageName] = data.NewPackage(fileName.Spelling())
			c.packageMutex.Unlock()
		}

		// Read the imported object as a file path if we haven't already done this
		// for this package.
		if !packageDef.HasImportedSource() {
			text, err := c.readPackageFile(fileName.Spelling())
			if err != nil {
				// If it wasn't found but we did add some builtins, good enough.
				// Skip past the filename that was rejected by c.Readfile()...
				if packageDef.Builtins() {
					c.t.Advance(1)

					if !isList || c.t.IsNext(tokenizer.EndOfListToken) {
						break
					}

					continue
				}

				// Nope, import had no effect.
				return err
			}

			ui.Log(ui.CompilerLogger, "+++ Adding source for package "+packageName)

			importCompiler := New("import " + filePath).SetRoot(c.rootTable).SetTestMode(c.flags.testMode)
			importCompiler.b = bytecode.New("import " + filePath)
			importCompiler.t = tokenizer.New(text, true)
			importCompiler.activePackageName = packageName

			for !importCompiler.t.AtEnd() {
				err := importCompiler.compileStatement()
				if err != nil {
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
			importSymbols := symbols.NewChildSymbolTable("import "+fileName.Spelling(), c.rootTable)
			ctx := bytecode.NewContext(importSymbols, importCompiler.b)

			if err = ctx.Run(); !errors.Equals(err, errors.ErrStop) {
				break
			}

			packageDef.SetImported(true)
		} else {
			ui.Log(ui.CompilerLogger, "--- Import of package \"%s\" already done", fileName)
		}

		// Rewrite the package if we've added stuff to it.
		if wasImported != packageDef.HasImportedSource() || wasBuiltin != packageDef.Builtins() {
			if ui.IsActive(ui.CompilerLogger) {
				ui.Log(ui.CompilerLogger, "+++ updating package definition: %s", fileName)

				keys := packageDef.Keys()
				keyString := ""

				for idx, k := range keys {
					if idx > 0 {
						keyString = keyString + ","
					}

					keyString = keyString + k
				}

				ui.Log(ui.CompilerLogger, "+++ package keys: %s", keyString)
			}

			symbols.RootSymbolTable.SetAlways(filePath, packageDef)
		}

		// Now that the package is in the cache, add the instruction to the active
		// program to import that cached info at runtime.
		c.b.Emit(bytecode.Import, packageName)

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

	ui.Log(ui.CompilerLogger, "+++ Reading package file %s", name)

	// Not a directory, try to read the file
	fn := name

	content, e2 := ioutil.ReadFile(fn)
	if e2 != nil {
		content, e2 = ioutil.ReadFile(name + defs.EgoFilenameExtension)
		if e2 != nil {
			// Path name did not resolve. Get the Ego path and try
			// variations on that.
			r := os.Getenv(defs.EgoPathEnv)
			if r == "" {
				r = settings.Get(defs.EgoPathSetting)
			}

			// Try to see if it's in the lib directory under EGO path
			fn = filepath.Join(r, defs.LibPathName, name+defs.EgoFilenameExtension)
			content, e2 = ioutil.ReadFile(fn)

			// Nope, see if it's in the path relative to EGO path
			if e2 != nil {
				fn = filepath.Join(r, name+defs.EgoFilenameExtension)
				content, e2 = ioutil.ReadFile(fn)
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

	// Convert []byte to string
	return string(content), nil
}

// directoryContents reads all the files in a directory into a single string.
func (c *Compiler) directoryContents(name string) (string, error) {
	var b strings.Builder

	r := os.Getenv(defs.EgoPathEnv)
	if r == "" {
		r = settings.Get(defs.EgoPathSetting)
	}

	r = filepath.Join(r, defs.LibPathName)

	dirname := name
	if !strings.HasPrefix(dirname, r) {
		dirname = filepath.Join(r, name)

		ui.Log(ui.CompilerLogger, "+++ Applying EGO_PATH, %s", dirname)
	}

	fi, err := ioutil.ReadDir(dirname)
	if err != nil {
		return "", errors.NewError(err)
	}

	ui.Log(ui.CompilerLogger, "+++ Directory read attempt for \"%s\"", name)

	if len(fi) == 0 {
		ui.Log(ui.CompilerLogger, "+++ Directory is empty")
	} else {
		ui.Log(ui.CompilerLogger, "+++ Reading package directory %s", dirname)
	}

	// For all the items that aren't directories themselves, and
	// for file names ending in defs.EgoExtension, read them into the master
	// result string. Note that recursive directory reading is
	// not supported.
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
