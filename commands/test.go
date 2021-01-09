package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/io"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/compiler"
	"github.com/tucats/gopackages/debugger"
	"github.com/tucats/gopackages/functions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/tokenizer"
)

// TestAction is the command handler for the ego TEST command
func TestAction(c *cli.Context) error {

	var text string
	var err error

	if err := runtime.InitProfileDefaults(); err != nil {
		return err
	}

	// Create an empty symbol table and store the program arguments.
	syms := symbols.NewSymbolTable("Unit Tests")

	staticTypes := persistence.GetBool(defs.StaticTypesSetting)
	if c.WasFound("static-types") {
		staticTypes = c.GetBool("static-types")
	}

	// Add test-specific functions and values
	_ = syms.SetAlways("eval", runtime.Eval)
	_ = syms.SetAlways("table", runtime.Table)
	_ = syms.SetAlways("_mode", "test")
	_ = syms.SetAlways("__static_data_types", staticTypes)
	runtime.AddBuiltinPackages(syms)

	exitValue := 0
	builtinsAdded := false

	// Use the parameters from the parent context which are the command line
	// values after the verb. If there were no parameters, assume the default
	// of "tests" is expected, from the ego path if it is defined.
	locations := []string{}
	fileList := []string{}

	if len(c.Parent.Parameters) == 0 {
		path := persistence.Get(defs.EgoPathSetting)
		if path == "" {
			path = os.Getenv("EGO_PATH")
		}
		defaultName := "tests"
		if path != "" {
			defaultName = filepath.Join(path, "tests")
		}
		locations = []string{defaultName}
	} else {
		locations = append(locations, c.Parent.Parameters...)
	}

	// Now use the list of locations given to build an expanded list of files
	for _, fileOrPath := range locations {
		files, err := functions.ExpandPath(fileOrPath, ".ego")
		if err != nil {
			return fmt.Errorf("unable to read file or path: %s", fileOrPath)
		}
		fileList = append(fileList, files...)
	}

	for _, fileOrPath := range fileList {
		text, err = ReadFile(fileOrPath)
		if err != nil {
			return fmt.Errorf("unable to read file: %s", fileOrPath)
		}

		// Handle special cases.
		if strings.TrimSpace(text) == QuitCommand {
			break
		}

		// Tokenize the input
		t := tokenizer.New(text)

		// Compile the token stream
		comp := compiler.New()
		name := strings.ReplaceAll(fileOrPath, "/", "_")
		b, err := comp.Compile(name, t)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			exitValue = 1
		} else {

			if !builtinsAdded {
				// Add the builtin functions
				comp.AddBuiltins("")

				// Always autoimport
				err := comp.AutoImport(true)
				if err != nil {
					fmt.Printf("Unable to auto-import packages: " + err.Error())
				}
				comp.AddPackageToSymbols(syms)
				builtinsAdded = true
			}
			oldDebugMode := ui.DebugMode
			if io.GetConfig(syms, ConfigDisassemble) {
				ui.DebugMode = true
				b.Disasm()
			}
			ui.DebugMode = oldDebugMode

			// Run the compiled code
			ctx := bytecode.NewContext(syms, b)
			oldDebugMode = ui.DebugMode
			ctx.Tracing = io.GetConfig(syms, ConfigTrace)
			if ctx.Tracing {
				ui.DebugMode = true
			}

			// If we are doing source tracing of execution, we'll need to link the tokenzier
			// back to the execution context. If you don't need source tracing, you can use
			// the simpler CompileString() function which doesn't require a discrete tokenizer.
			if c.GetBool("source-tracing") {
				ctx.SetTokenizer(t)
			}

			err = ctx.Run()
			if err != nil && err.Error() == debugger.Stop.Error() {
				err = nil
			}
			ui.DebugMode = oldDebugMode

			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
				exitValue = 2
			}
		}

	}

	if exitValue > 0 {
		return errors.New(defs.TerminatedWithErrors)
	}
	return nil
}

// ReadDirectory reads all the files in a directory into a single string.
func ReadDirectory(name string) (string, error) {

	var b strings.Builder

	dirname := name
	fi, err := ioutil.ReadDir(dirname)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			ui.Debug(ui.DebugLogger, "+++ No such directory")
		}
		return "", err
	}

	ui.Debug(ui.DebugLogger, "+++ Directory read attempt for \"%s\"", name)
	if len(fi) == 0 {
		ui.Debug(ui.DebugLogger, "+++ Directory is empty")
	} else {
		ui.Debug(ui.DebugLogger, "+++ Reading test directory %s", dirname)
	}

	// For all the items that aren't directories themselves, and
	// for file names ending in ".ego", read them into the master
	// result string. Note that recursive directory reading is
	// not supported.
	for _, f := range fi {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".ego") {
			fname := filepath.Join(dirname, f.Name())
			t, err := ReadFile(fname)
			if err != nil {
				return "", err
			}
			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}

// ReadFile reads the text from a file into a string
func ReadFile(name string) (string, error) {

	s, err := ReadDirectory(name)
	if err == nil {
		return s, nil
	}
	ui.Debug("+++ Reading test file %s", name)
	// Not a directory, try to read the file
	content, err := ioutil.ReadFile(name)
	if err != nil {
		content, err = ioutil.ReadFile(name + ".ego")
		if err != nil {
			r := os.Getenv("EGO_PATH")
			fn := filepath.Join(r, "lib", name+".ego")
			content, err = ioutil.ReadFile(fn)
			if err != nil {
				return "", fmt.Errorf("unable to read test file: %s", err.Error())
			}
		}
	}

	// Convert []byte to string
	return string(content), nil
}
