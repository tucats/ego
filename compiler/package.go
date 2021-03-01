package compiler

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// compilePackage compiles a package statement.
func (c *Compiler) compilePackage() *errors.EgoError {
	name := c.t.Next()
	if !tokenizer.IsSymbol(name) {
		return c.newError(errors.InvalidPackageName, name)
	}

	name = strings.ToLower(name)

	if (c.PackageName != "") && (c.PackageName != name) {
		return c.newError(errors.PackageRedefinitionError)
	}

	c.PackageName = name

	c.b.Emit(bytecode.PushPackage, name)

	return nil
}

// compileImport handles the import statement.
func (c *Compiler) compileImport() *errors.EgoError {
	var err *errors.EgoError

	if c.blockDepth > 0 {
		return c.newError(errors.InvalidImportError)
	}

	if c.loops != nil {
		return c.newError(errors.InvalidImportError)
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

		fileName := c.t.Next()

		if _, ok := c.packages[fileName]; ok {
			ui.Debug(ui.CompilerLogger, "*** Already imported \"%s\", skipping...", fileName)
		} else {
			ui.Debug(ui.CompilerLogger, "*** Importing package \"%s\"", fileName)

			if isList && fileName == ")" {
				break
			}

			if len(fileName) > 2 && fileName[:1] == "\"" {
				fileName = fileName[1 : len(fileName)-1]
			}

			if c.loops != nil {
				return c.newError(errors.InvalidImportError)
			}

			// Get the package name from the given string. If this is
			// a file system name, remove the extension if present.
			packageName := filepath.Base(fileName)
			if filepath.Ext(packageName) != "" {
				packageName = packageName[:len(filepath.Ext(packageName))]
			}

			packageName = strings.ToLower(packageName)

			// If this is an import of a package already processed, no work to do.
			if _, found := c.s.Get(packageName); found {
				ui.Debug(ui.CompilerLogger, "+++ Previously imported \"%s\", skipping", packageName)

				continue
			}

			// If this is an import of the package we're currently importing, no work to do.
			if packageName == c.PackageName {
				continue
			}

			builtinsAdded := c.AddBuiltins(packageName)
			if builtinsAdded {
				ui.Debug(ui.CompilerLogger, "+++ Adding builtins for package "+packageName)
			} else {
				// The nil in the packages list just prevents this from being read again
				// if it was already processed once.
				ui.Debug(ui.CompilerLogger, "+++ No builtins for package "+packageName)
				c.packages[packageName] = nil
			}

			// Read the imported object as a file path
			text, err := c.readPackageFile(fileName)
			if !errors.Nil(err) {
				// If it wasn't found but we did add some builtins, good enough.
				// Skip past the filename that was rejected by c.Readfile()...
				if builtinsAdded {
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

			importCompiler := New("import " + fileName)
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

			// If after the import we ended with mismatched block markers, complain
			if importCompiler.blockDepth != 0 {
				return c.newError(errors.MissingEndOfBlockError, packageName)
			}

			// The import will have generate code that must be run to actually register
			// package contents.
			importSymbols := symbols.NewChildSymbolTable("import "+fileName, &symbols.RootSymbolTable)
			ctx := bytecode.NewContext(importSymbols, importCompiler.b)
			ctx.SetTracing(true)

			err = ctx.Run()
			if err != nil && !err.Is(errors.Stop) {
				break
			}

			if _, ok := c.packages[fileName]; ok {
				ui.Debug(ui.CompilerLogger, "+++ expected package not in dictionary: %s", fileName)
			}
		}

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
		content, e2 = ioutil.ReadFile(name + ".ego")
		if !errors.Nil(e2) {
			// Path name did not resolve. Get the Ego path and try
			// variations on that.
			r := os.Getenv("EGO_PATH")
			if r == "" {
				r = persistence.Get(defs.EgoPathSetting)
			}

			// Try to see if it's in the lib directory under EGO path
			fn = filepath.Join(r, "lib", name+".ego")
			content, e2 = ioutil.ReadFile(fn)

			// Nope, see if it's in the path relative to EGO path
			if e2 != nil {
				fn = filepath.Join(r, name+".ego")
				content, e2 = ioutil.ReadFile(fn)
			}

			if e2 != nil {
				c.t.Advance(-1)

				return "", c.newError(e2)
			}
		} else {
			fn = name + ".ego"
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

	r := os.Getenv("EGO_PATH")
	if r == "" {
		r = persistence.Get(EgoPathSetting)
	}

	r = filepath.Join(r, "lib")

	dirname := name
	if !strings.HasPrefix(dirname, r) {
		dirname = filepath.Join(r, name)

		ui.Debug(ui.CompilerLogger, "+++ Applying EGO_PATH, %s", dirname)
	}

	fi, err := ioutil.ReadDir(dirname)
	if !errors.Nil(err) {
		if _, ok := err.(*os.PathError); ok {
			ui.Debug(ui.CompilerLogger, "+++ No such directory")
		}

		return "", errors.New(err)
	}

	ui.Debug(ui.CompilerLogger, "+++ Directory read attempt for \"%s\"", name)

	if len(fi) == 0 {
		ui.Debug(ui.CompilerLogger, "+++ Directory is empty")
	} else {
		ui.Debug(ui.CompilerLogger, "+++ Reading package directory %s", dirname)
	}

	// For all the items that aren't directories themselves, and
	// for file names ending in ".ego", read them into the master
	// result string. Note that recursive directory reading is
	// not supported.
	for _, f := range fi {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".ego") {
			fname := filepath.Join(dirname, f.Name())

			t, err := c.readPackageFile(fname)
			if !errors.Nil(err) {
				return "", err
			}

			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}
