package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// TestAction is the command handler for the ego TEST command.
func TestAction(c *cli.Context) *errors.EgoError {
	var text string

	var err *errors.EgoError

	if err := runtime.InitProfileDefaults(); !errors.Nil(err) {
		return err
	}

	// Set extensions to be enabled for this run. Also, sandboxing file system
	// operations will break tests, so disable that.
	settings.SetDefault(defs.ExtensionsEnabledSetting, defs.True)
	settings.SetDefault(defs.SandboxPathSetting, "")

	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable("Unit Tests")
	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, "dynamic", "static") == 2

	if c.WasFound("static-types") {
		staticTypes = c.Boolean("static-types")
	}

	// Add test-specific functions and values
	_ = symbolTable.SetAlways("eval", runtime.Eval)
	_ = symbolTable.SetAlways("table", runtime.Table)
	_ = symbolTable.SetAlways("__exec_mode", "test")
	_ = symbolTable.SetAlways("__static_data_types", staticTypes)

	runtime.AddBuiltinPackages(symbolTable)

	exitValue := 0
	builtinsAdded := false

	// Use the parameters from the parent context which are the command line
	// values after the verb. If there were no parameters, assume the default
	// of "tests" is expected, from the ego path if it is defined.
	locations := []string{}
	fileList := []string{}

	if len(c.Parent.Parameters) == 0 {
		path := settings.Get(defs.EgoPathSetting)
		if path == "" {
			path = os.Getenv(defs.EgoPathEnv)
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
		files, err := functions.ExpandPath(fileOrPath, defs.EgoFilenameExtension)
		if !errors.Nil(err) {
			return err
		}

		fileList = append(fileList, files...)
	}

	for _, fileOrPath := range fileList {
		text, err = ReadFile(fileOrPath)
		if !errors.Nil(err) {
			return err
		}

		// Tokenize the input
		t := tokenizer.New(text)

		// If it doesn't start with "@", "test" it's not a test,
		// but a support file, and we skip it.
		if len(t.Tokens) < 2 || t.Peek(1) != tokenizer.DirectiveToken || t.Peek(2) != tokenizer.TestToken {
			continue
		}

		// Compile the token stream, using a compilier running in "test" mode.
		name := filepath.Base(fileOrPath)
		comp := compiler.New(name).SetTestMode(true)

		// We set this to "interaactive" mode so tests can include program
		// text without needing a main
		comp.SetInteractive(true)

		b, err := comp.Compile(name, t)
		if !errors.Nil(err) {
			exitValue = 1
			msg := fmt.Sprintf("%s: %v\n", i18n.L("Error"), err.Error())

			os.Stderr.Write([]byte(msg))
		} else {
			if !builtinsAdded {
				// Add the builtin functions
				comp.AddStandard(symbolTable)

				// Always autoimport
				err := comp.AutoImport(true, symbolTable)
				if !errors.Nil(err) {
					msg := fmt.Sprintf("%s\n", i18n.E("auto.import", map[string]interface{}{
						"err": err.Error(),
					}))

					os.Stderr.Write([]byte(msg))
				}

				//comp.AddPackageToSymbols(symbolTable)

				builtinsAdded = true
			}

			if ui.LoggerIsActive(ui.ByteCodeLogger) {
				b.Disasm()
			}

			// Run the compiled code
			ctx := bytecode.NewContext(symbolTable, b)

			ctx.EnableConsoleOutput(false)
			if c.Boolean("trace") {
				ui.SetLogger(ui.TraceLogger, true)
			}

			// If we are doing source tracing of execution, we'll need to link the tokenizer
			// back to the execution context. If you don't need source tracing, you can use
			// the simpler CompileString() function which doesn't require a discrete tokenizer.
			if c.Boolean("source-tracing") {
				ctx.SetTokenizer(t)
			}

			err = ctx.Run()
			if err.Is(errors.ErrStop) {
				err = nil
			}

			if !errors.Nil(err) {
				exitValue = 2
				msg := fmt.Sprintf("%s: %v\n", i18n.L("Error"), err.Error())

				os.Stderr.Write([]byte(msg))
			}
		}
	}

	if exitValue > 0 {
		return errors.New(errors.ErrTerminatedWithErrors)
	}

	return nil
}

// ReadDirectory reads all the files in a directory into a single string.
func ReadDirectory(name string) (string, *errors.EgoError) {
	var b strings.Builder

	dirname := name

	fi, err := ioutil.ReadDir(dirname)
	if !errors.Nil(err) {
		if _, ok := err.(*os.PathError); ok {
			ui.Debug(ui.DebugLogger, "+++ No such directory")
		}

		return "", errors.New(err)
	}

	ui.Debug(ui.DebugLogger, "+++ Directory read attempt for \"%s\"", name)

	if len(fi) == 0 {
		ui.Debug(ui.DebugLogger, "+++ Directory is empty")
	} else {
		ui.Debug(ui.DebugLogger, "+++ Reading test directory %s", dirname)
	}

	// For all the items that aren't directories themselves, and
	// for file names ending in defs.EgoExtension, read them into the master
	// result string. Note that recursive directory reading is
	// not supported.
	for _, f := range fi {
		if !f.IsDir() && strings.HasSuffix(f.Name(), defs.EgoFilenameExtension) {
			fileName := filepath.Join(dirname, f.Name())

			t, err := ReadFile(fileName)
			if !errors.Nil(err) {
				return "", err
			}

			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}

// ReadFile reads the text from a file into a string.
func ReadFile(name string) (string, *errors.EgoError) {
	s, err := ReadDirectory(name)
	if errors.Nil(err) {
		return s, nil
	}

	ui.Debug(ui.TraceLogger, "+++ Reading test file %s", name)

	// Not a directory, try to read the file
	content, e2 := ioutil.ReadFile(name)
	if e2 != nil {
		content, e2 = ioutil.ReadFile(name + defs.EgoFilenameExtension)
		if e2 != nil {
			r := os.Getenv(defs.EgoPathEnv)
			fn := filepath.Join(r, defs.LibPathName, name+defs.EgoFilenameExtension)

			content, e2 = ioutil.ReadFile(fn)
			if e2 != nil {
				return "", errors.New(e2)
			}
		}
	}

	// Convert []byte to string
	return string(content), nil
}
