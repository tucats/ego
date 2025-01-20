package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/runtime/io"
	"github.com/tucats/ego/runtime/profile"
	rutil "github.com/tucats/ego/runtime/util"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// TestAction is the command handler for the ego TEST command.
func TestAction(c *cli.Context) error {
	var (
		text string
		err  error
	)

	if err := profile.InitProfileDefaults(profile.AllDefaults); err != nil {
		return err
	}

	// Set any options needed in the symbol table for test execution. For example,
	// language extensions are enabled and file sandboxing is disabled.
	settings.SetDefault(defs.ExtensionsEnabledSetting, defs.True)
	settings.SetDefault(defs.SandboxPathSetting, "")
	symbols.RootSymbolTable.SetAlways(defs.ExtensionsVariable, true)

	// Create the global symbol table for running tests.
	symbolTable := configureTestSymbolTable(c)

	exitValue := 0
	builtinsAdded := false

	// Turn on the deep scope setting. This meeans that functions do not have
	// isolated symbol tables, but instead the entire runtime stack of symbols
	// is always visible to all units. This aids in making tests more modular
	// without breaking some assumptions about symbol values and what scope
	// they are found in during testing.
	settings.SetDefault(defs.RuntimeDeepScopeSetting, "true")

	// Use the parameters from the parent context which are the command line
	// values after the verb. If there were no parameters, assume the default
	// of "tests" is expected, from the ego path if it is defined.
	fileList, err := getTestFileList(c)
	if err != nil {
		return err
	}

	for _, fileOrPath := range fileList {
		text, err = readTestFile(fileOrPath)
		if err != nil {
			return err
		}

		// Tokenize the input
		t := tokenizer.New(text, true)

		// Skip any blank lines (just have an end-of-line semicolon added). If the next
		// token info doesn't start with "@", "test" it's not a test, but a support file,
		// and we skip it.
		for t.IsNext(tokenizer.SemicolonToken) {
		}

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
		if err != nil {
			exitValue = 1
			msg := fmt.Sprintf("%s: %v\n", i18n.L("Error"), err.Error())

			os.Stderr.Write([]byte(msg))
		} else {
			if !builtinsAdded {
				// Add the builtin functions
				compiler.AddStandard(symbolTable)

				// Always autoimport
				err := comp.AutoImport(true, symbolTable)
				if err != nil {
					msg := fmt.Sprintf("%s\n", i18n.E("auto.import", map[string]interface{}{
						"err": err.Error(),
					}))

					os.Stderr.Write([]byte(msg))
				}

				//comp.AddPackageToSymbols(symbolTable)

				builtinsAdded = true
			}

			b.Disasm()

			// Run the compiled code
			ctx := bytecode.NewContext(symbolTable, b)

			ctx.EnableConsoleOutput(false)

			if c.Boolean("trace") {
				ui.Active(ui.TraceLogger, true)
			}

			err = ctx.Run()
			if errors.Equals(err, errors.ErrStop) {
				err = nil
			}

			if err != nil {
				exitValue = 2
				msg := fmt.Sprintf("%s: %v\n", i18n.L("Error"), err.Error())

				os.Stderr.Write([]byte(msg))
			}
		}
	}

	if exitValue > 0 {
		return errors.ErrTerminatedWithErrors
	}

	return nil
}

func configureTestSymbolTable(c *cli.Context) *symbols.SymbolTable {
	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable("Unit Tests").Shared(true)
	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, defs.Strict, defs.Relaxed, defs.Dynamic) - 1

	if c.WasFound(defs.TypingOption) {
		typeOption, _ := c.Keyword(defs.TypingOption)
		if typeOption < defs.StrictTypeEnforcement {
			typeOption = defs.NoTypeEnforcement
		}

		staticTypes = typeOption
	}

	if c.WasFound(defs.OptimizerOption) {
		optimize := "true"
		if !c.Boolean(defs.OptimizerOption) {
			optimize = "false"
		}

		settings.Set(defs.OptimizerSetting, optimize)
	}

	// Add test-specific functions and values
	symbolTable.SetAlways("eval", rutil.Eval)
	symbolTable.SetAlways(defs.ModeVariable, "test")
	symbolTable.SetAlways(defs.TypeCheckingVariable, staticTypes)

	runtime.AddPackages(symbolTable)

	return symbolTable
}

// getTestFileList reads all the files specified implicitly or explicitly on the
// command line and generates a list of file paths containing all the tests to be
// run.
func getTestFileList(c *cli.Context) ([]string, error) {
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
		files, err := io.ExpandPath(fileOrPath, defs.EgoFilenameExtension)
		if err != nil {
			return nil, err
		}

		fileList = append(fileList, files...)
	}

	sort.Strings(fileList)

	return fileList, nil
}

// readTestDirectory reads all the files in a directory into a single string.
func readTestDirectory(name string) (string, error) {
	var b strings.Builder

	dirname := name

	fileInfos, err := os.ReadDir(dirname)
	if err != nil {
		return "", errors.New(err)
	}

	// Alphebetize the names
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].Name() < fileInfos[j].Name()
	})

	// For all the items that aren't directories themselves, and
	// for file names ending in defs.EgoExtension, read them into the master
	// result string. Note that recursive directory reading is
	// not supported.
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), defs.EgoFilenameExtension) {
			fileName := filepath.Join(dirname, fileInfo.Name())

			t, err := readTestFile(fileName)
			if err != nil {
				return "", err
			}

			b.WriteString(t)
		}
	}

	return b.String(), nil
}

// readTestFile reads the text from a file into a string.
func readTestFile(name string) (string, error) {
	if s, err := readTestDirectory(name); err == nil {
		return s, nil
	}

	ui.Log(ui.TraceLogger, "trace.test.file",
		"path", name)

	// Not a directory, try to read the file
	content, err := os.ReadFile(name)
	if err != nil {
		content, err = os.ReadFile(name + defs.EgoFilenameExtension)
		if err != nil {
			path := ""
			if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
				path = libpath
			} else {
				path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
			}

			fn := filepath.Join(path, name+defs.EgoFilenameExtension)

			if content, err = os.ReadFile(fn); err != nil {
				return "", errors.New(err)
			}
		}
	}

	// Convert []byte to string and add a trailing @pass directive. If there is
	// already one in the file, this is ignored.
	text := string(content) + "\n@pass\n"

	return text, nil
}
