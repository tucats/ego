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
			ui.Debug(ui.CompilerLogger, "+++ No builtins for package "+packageName)
		}

		// Save some state
		savedPackageName := c.PackageName
		savedTokenizer := c.t
		savedBlockDepth := c.blockDepth
		savedStatementCount := c.statementCount
		savedSourceFile := c.SourceFile

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
		// Set up the new compiler settings
		c.statementCount = 0
		c.t = tokenizer.New(text)
		c.PackageName = packageName

		for !c.t.AtEnd() {
			err := c.compileStatement()
			if !errors.Nil(err) {
				return err
			}
		}

		c.b.Emit(bytecode.PopPackage)

		// If after the import we ended with mismatched block markers, complain
		if c.blockDepth != savedBlockDepth {
			return c.newError(errors.MissingEndOfBlockError, packageName)
		}

		// Reset the compiler back to the token stream we were working on
		c.t = savedTokenizer
		c.PackageName = savedPackageName
		c.SourceFile = savedSourceFile
		c.statementCount = savedStatementCount

		if !isList {
			break
		}

		if isList && c.t.Next() == ")" {
			break
		}
	}

	return nil
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
