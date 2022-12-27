package compiler

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// compilePackage compiles a package statement.
func (c *Compiler) compilePackage() *errors.EgoError {
	if c.t.AnyNext(";", tokenizer.EndOfTokens) {
		return c.newError(errors.ErrMissingPackageName)
	}

	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.ErrInvalidPackageName, name)
	}

	name = strings.ToLower(name)

	if (c.PackageName != "") && (c.PackageName != name) {
		return c.newError(errors.ErrPackageRedefinition)
	}

	c.PackageName = name

	c.b.Emit(bytecode.PushPackage, name)

	return nil
}

// compileImport handles the import statement.
func (c *Compiler) compileImport() *errors.EgoError {
	var err *errors.EgoError

	// Unless we are in test mode, constrain imports to
	// only occur at the top-level before blocks are
	// compiled (i.e. not inside a function, etc.)
	if !c.TestMode() {
		if c.blockDepth > 0 {
			return c.newError(errors.ErrInvalidImport)
		}

		if c.loops != nil {
			return c.newError(errors.ErrInvalidImport)
		}
	}

	isList := false
	if c.t.IsNext("(") {
		isList = true

		ui.Debug(ui.CompilerLogger, "*** Processing import list")
	}

	parsing := true
	for parsing {
		// Make sure that if this isn't the list format of an import, we only do this once.
		if !isList {
			parsing = false
		}

		var packageName string

		fileName := c.t.Next()
		if fileName[0:1] != "\"" {
			packageName = fileName
			fileName = c.t.Next()
		}

		if len(fileName) > 2 && fileName[:1] == "\"" {
			fileName = fileName[1 : len(fileName)-1]
			// Get the package name from the given string if it wasn't
			// explicitly given in the import statement.If this is
			// a file system name, remove the extension if present.
			if packageName == "" {
				packageName = filepath.Base(fileName)
				if filepath.Ext(packageName) != "" {
					packageName = packageName[:len(filepath.Ext(packageName))]
				}
			}
		}

		if isList && fileName == ")" {
			break
		}

		packageName = strings.ToLower(packageName)
		pkgData, _ := bytecode.GetPackage(packageName)

		wasBuiltin := pkgData.Builtins
		wasImported := pkgData.Imported

		ui.Debug(ui.CompilerLogger, "*** Importing package \"%s\"", fileName)

		// If this is an import of the package we're currently importing, no work to do.
		if packageName == c.PackageName {
			continue
		}

		if !pkgData.Builtins {
			pkgData.Builtins = c.AddBuiltins(packageName)

			ui.Debug(ui.CompilerLogger, "+++ Added builtins for package "+fileName)
		} else {
			ui.Debug(ui.CompilerLogger, "--- Builtins already initialized for package "+fileName)
		}

		if !pkgData.Builtins {
			// The nil in the packages list just prevents this from being read again
			// if it was already processed once.
			ui.Debug(ui.CompilerLogger, "+++ No builtins for package "+fileName)
			c.packages.Mutex.Lock()
			c.packages.Package[packageName] = datatypes.NewPackage(fileName)
			c.packages.Mutex.Unlock()
		}

		// Read the imported object as a file path if we haven't already done this
		// for this package.
		if !pkgData.Imported {
			text, err := c.readPackageFile(fileName)
			if !errors.Nil(err) {
				// If it wasn't found but we did add some builtins, good enough.
				// Skip past the filename that was rejected by c.Readfile()...
				if pkgData.Builtins {
					c.t.Advance(1)

					if !isList || c.t.IsNext(")") {
						break
					}

					continue
				}

				// Nope, import had no effect.
				return err
			}

			ui.Debug(ui.CompilerLogger, "+++ Adding source for package "+packageName)

			importCompiler := New("import " + fileName).SetRoot(c.RootTable).SetTestMode(c.testMode)
			importCompiler.b = bytecode.New("import " + fileName)
			importCompiler.t = tokenizer.New(text)
			importCompiler.PackageName = packageName

			for !importCompiler.t.AtEnd() {
				err := importCompiler.compileStatement()
				if !errors.Nil(err) {
					return err
				}
			}

			importCompiler.b.Emit(bytecode.PopPackage, packageName)

			// If we are disassembling, do it now for the imported definitions.
			if ui.LoggerIsActive(ui.ByteCodeLogger) {
				importCompiler.b.Disasm()
			}

			// If after the import we ended with mismatched block markers, complain
			if importCompiler.blockDepth != 0 {
				return c.newError(errors.ErrMissingEndOfBlock, packageName)
			}

			// The import will have generate code that must be run to actually register
			// package contents.
			importSymbols := symbols.NewChildSymbolTable("import "+fileName, c.RootTable)
			ctx := bytecode.NewContext(importSymbols, importCompiler.b)

			err = ctx.Run()
			if err != nil && !err.Is(errors.ErrStop) {
				break
			}

			pkgData.Imported = true

		} else {
			ui.Debug(ui.CompilerLogger, "--- Import of package \"%s\" already done", fileName)
		}

		// Rewrite the package if we've added stuff to it.
		if wasImported != pkgData.Imported || wasBuiltin != pkgData.Builtins {
			if ui.LoggerIsActive(ui.CompilerLogger) {
				ui.Debug(ui.CompilerLogger, "+++ updating package definition: %s", fileName)

				keys := pkgData.Keys()
				keyString := ""

				for idx, k := range keys {
					if idx > 0 {
						keyString = keyString + ","
					}

					keyString = keyString + k
				}

				ui.Debug(ui.CompilerLogger, "+++ package keys: %s", keyString)
			}

			err2 := symbols.RootSymbolTable.SetAlways(fileName, pkgData)
			if !errors.Nil(err2) {
				return err2
			}
		}

		// Now that the package is in the cache, add the instruction to the active
		// program to import that cached info at runtime.
		c.b.Emit(bytecode.Import, packageName)

		if !isList {
			break
		}

		if isList && c.t.Next() == ")" {
			break
		}
	}

	return err
}

// readPackageFile reads the text from a file into a string.
func (c *Compiler) readPackageFile(name string) (string, *errors.EgoError) {
	s, err := c.directoryContents(name)
	if errors.Nil(err) {
		return s, nil
	}

	ui.Debug(ui.CompilerLogger, "+++ Reading package file %s", name)

	// Not a directory, try to read the file
	fn := name

	content, e2 := ioutil.ReadFile(fn)
	if e2 != nil {
		content, e2 = ioutil.ReadFile(name + defs.EgoFilenameExtension)
		if !errors.Nil(e2) {
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

				return "", c.newError(e2)
			}
		} else {
			fn = name + defs.EgoFilenameExtension
		}
	}

	if e2 == nil {
		c.SourceFile = fn
	}

	// Convert []byte to string
	return string(content), nil
}

// directoryContents reads all the files in a directory into a single string.
func (c *Compiler) directoryContents(name string) (string, *errors.EgoError) {
	var b strings.Builder

	r := os.Getenv(defs.EgoPathEnv)
	if r == "" {
		r = settings.Get(EgoPathSetting)
	}

	r = filepath.Join(r, defs.LibPathName)

	dirname := name
	if !strings.HasPrefix(dirname, r) {
		dirname = filepath.Join(r, name)

		ui.Debug(ui.CompilerLogger, "+++ Applying EGO_PATH, %s", dirname)
	}

	fi, err := ioutil.ReadDir(dirname)
	if !errors.Nil(err) {
		return "", errors.New(err)
	}

	ui.Debug(ui.CompilerLogger, "+++ Directory read attempt for \"%s\"", name)

	if len(fi) == 0 {
		ui.Debug(ui.CompilerLogger, "+++ Directory is empty")
	} else {
		ui.Debug(ui.CompilerLogger, "+++ Reading package directory %s", dirname)
	}

	// For all the items that aren't directories themselves, and
	// for file names ending in defs.EgoExtension, read them into the master
	// result string. Note that recursive directory reading is
	// not supported.
	for _, f := range fi {
		if !f.IsDir() && strings.HasSuffix(f.Name(), defs.EgoFilenameExtension) {
			fileName := filepath.Join(dirname, f.Name())

			t, err := c.readPackageFile(fileName)
			if !errors.Nil(err) {
				return "", err
			}

			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}
