package commands

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// RunAction is the command handler for the ego CLI.
func RunAction(c *cli.Context) *errors.EgoError {
	if logFile, found := c.String("log"); found {
		err := ui.OpenLogFile(logFile, false)
		if !errors.Nil(err) {
			return err
		}
	}

	if err := runtime.InitProfileDefaults(); !errors.Nil(err) {
		return err
	}

	programArgs := make([]interface{}, 0)
	mainName := "main"
	prompt := c.MainProgram + "> "
	debug := c.Boolean("debug")
	text := ""
	wasCommandLine := true
	fullScope := false
	lineNumber := 1

	entryPoint, _ := c.String("entry-point")
	if entryPoint == "" {
		entryPoint = "main"
	}

	var comp *compiler.Compiler

	// Get the allocation factor for symbols from the configuration.
	symAllocFactor := settings.GetInt(defs.SymbolTableAllocationSetting)
	if symAllocFactor > 0 {
		symbols.SymbolAllocationSize = symAllocFactor
	}

	// If it was specified on the command line, override it.
	if c.WasFound(defs.SymbolTableSizeOption) {
		symbols.SymbolAllocationSize, _ = c.Integer(defs.SymbolTableSizeOption)
	}

	// Ensure that the value isn't too small
	if symbols.SymbolAllocationSize < symbols.MinSymbolAllocationSize {
		symbols.SymbolAllocationSize = symbols.MinSymbolAllocationSize
	}

	autoImport := settings.GetBool(defs.AutoImportSetting)
	if c.WasFound(defs.AutoImportSetting) {
		autoImport = c.Boolean(defs.AutoImportOption)
	}

	if c.WasFound(defs.FullSymbolScopeOption) {
		fullScope = c.Boolean(defs.FullSymbolScopeOption)
	}

	disassemble := c.Boolean(defs.DisassembleOption)
	if disassemble {
		ui.SetLogger(ui.ByteCodeLogger, true)
	}

	interactive := false

	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, "dynamic", "static") == 2
	if c.WasFound(defs.StaticTypesOption) {
		staticTypes = c.Boolean(defs.StaticTypesOption)
	}

	argc := c.GetParameterCount()
	if argc > 0 {
		fileName := c.GetParameter(0)

		// If the input file is "." then we read all of stdin
		if fileName == "." {
			text = ""
			mainName = "console"

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				text = text + scanner.Text() + " "
			}
		} else {
			// Otherwise, use the parameter as a filename
			content, err := ioutil.ReadFile(fileName)
			if !errors.Nil(err) {
				content, err = ioutil.ReadFile(fileName + defs.EgoFilenameExtension)
				if !errors.Nil(err) {
					return errors.New(err).Context(fileName)
				}
			}

			mainName = fileName
			text = string(content) + "\n@main " + entryPoint
		}
		// Remaining command line arguments are stored
		if argc > 1 {
			programArgs = make([]interface{}, argc-1)

			for n := 1; n < argc; n = n + 1 {
				programArgs[n-1] = c.GetParameter(n)
			}
		}
	} else if argc == 0 {
		wasCommandLine = false

		if !ui.IsConsolePipe() {
			var banner string

			if settings.Get(defs.NoCopyrightSetting) != defs.True {
				banner = c.AppName + " " + c.Version + " " + c.Copyright
			}

			fmt.Printf("%s\n", banner)

			// If this is the first time through this loop, interactive is still
			// false, but we know we're going to use user input. So this first
			// time through, make the text just be an empty string. This will
			// force the run loop to compile the empty string, which will process
			// all the uuto-imports. In this way, the use of -d TRACE on the
			// command line will handle all the import processing BEFORE the
			// first prompt, so the tracing after the prompt is just for the
			// statement(s) typed in at the prompt.
			//
			// If we already know we're interaactive, this isn't the first time
			// through the loop, and we just prompt the user for statements.
			if !interactive {
				text = ""
			} else {
				text = runtime.ReadConsoleText(prompt)
			}

			interactive = true
		} else {
			wasCommandLine = true // It is a pipe, so no prompting for more!
			text = ""
			mainName = "<stdin>"

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				text = text + scanner.Text() + " "
			}
		}
	}

	// Set up the symbol table.
	symbolTable := initializeSymbols(c, mainName, programArgs, staticTypes, interactive, disassemble)
	_ = symbolTable.Root().SetAlways("__main", "main")

	exitValue := 0

	for {
		if len(text) > 8 && text[:8] == "%include" {
			fileName := strings.TrimSpace(text[8:])

			content, err := ioutil.ReadFile(fileName)
			if !errors.Nil(err) {
				content, err = ioutil.ReadFile(fileName + defs.EgoFilenameExtension)
				if !errors.Nil(err) {
					return errors.New(err).Context(fileName)
				}
			}
			// Convert []byte to string
			text = string(content)
		}

		// If we are processing interactive console commands, and help is aenabled, and this is a
		// "help" command, handle that specially.
		if interactive && settings.GetBool(defs.ExtensionsEnabledSetting) && len(text) >= 4 && (text == "help\n" || text[:5] == "help ") {
			text = ""
			keys := strings.Split(strings.ToLower(text), " ")

			help(keys)

			continue
		}

		// If we're interactive and not in debug mode, help out
		// by updating the line number in REPL mode.
		if interactive && !debug {
			text = fmt.Sprintf("@line %d;\n%s", lineNumber, text)
			lineNumber = lineNumber + strings.Count(text, "\n") - 1
		}

		// Tokenize the input
		t := tokenizer.New(text)

		// If not in command-line mode, see if there is an incomplete quote
		// in the last token, which means we want to prompt for more and
		// re-tokenize
		for !wasCommandLine && len(t.Tokens) > 0 {
			lastToken := t.Tokens[len(t.Tokens)-1]
			if lastToken[0:1] == "`" && lastToken[len(lastToken)-1:] != "`" {
				text = text + runtime.ReadConsoleText("...> ")
				t = tokenizer.New(text)
				lineNumber++

				continue
			}

			break
		}

		// Also, make sure we have a balanced count for {}, (), and [] if we're in interactive
		// mode.
		for interactive && len(t.Tokens) > 0 {
			count := 0

			for _, v := range t.Tokens {
				switch v {
				case "{", "(", "[":
					count++

				case "}", ")", "]":
					count--
				}
			}

			if count > 0 {
				text = text + runtime.ReadConsoleText("...> ")
				t = tokenizer.New(text)
				lineNumber++

				continue
			} else {
				break
			}
		}

		// If this is the exit command, turn off the debugger to prevent and endless loop
		if t != nil && len(t.Tokens) > 0 && t.Tokens[0] == "exit" {
			debug = false
		}

		// Compile the token stream. Allow the EXIT command only if we are in "run" mode interactively
		if comp == nil {
			comp = compiler.New("run").WithNormalization(settings.GetBool(defs.CaseNormalizedSetting)).ExitEnabled(interactive)

			// link to the global table so we pick up special builtins.
			comp.SetRoot(&symbols.RootSymbolTable)

			err := comp.AutoImport(autoImport)
			if !errors.Nil(err) {
				panic(fmt.Sprintf("Unable to auto-import packages: " + err.Error()))
			}

			comp.AddPackageToSymbols(&symbols.RootSymbolTable)
			comp.SetInteractive(interactive)
		}

		b, err := comp.Compile(mainName, t)
		if !errors.Nil(err) {
			exitValue = 1
			msg := fmt.Sprintf("Error: %s\n", err.Error())

			os.Stderr.Write([]byte(msg))
		} else {
			if ui.LoggerIsActive(ui.ByteCodeLogger) {
				b.Disasm()
			}

			// Run the compiled code from a new context, configured with the symbol table,
			// token stream, and scope/debug settings.
			ctx := bytecode.NewContext(symbolTable, b).
				SetDebug(debug).
				SetTokenizer(t).
				SetFullSymbolScope(fullScope)

			if ctx.Tracing() {
				ui.SetLogger(ui.DebugLogger, true)
			}

			// If we run under control of the debugger, do that, else just run the context.
			if debug {
				err = debugger.Run(ctx)
			} else {
				err = ctx.Run()
			}

			if err.Is(errors.ErrStop) {
				err = nil
			}

			if !errors.Nil(err) {
				exitValue = 2
				msg := fmt.Sprintf("Error: %s\n", err.Error())

				os.Stderr.Write([]byte(msg))
			} else {
				exitValue = 0
			}

			if c.Boolean("symbols") {
				fmt.Println(symbolTable.Format(false))
			}
		}

		if wasCommandLine {
			break
		}

		text = runtime.ReadConsoleText(prompt)
	}

	if exitValue > 0 {
		return errors.New(errors.ErrTerminatedWithErrors)
	}

	return nil
}

func initializeSymbols(c *cli.Context, mainName string, programArgs []interface{}, staticTypes, interactive, disassemble bool) *symbols.SymbolTable {
	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable("file " + mainName)

	args := datatypes.NewArrayFromArray(datatypes.StringType, programArgs)
	_ = symbolTable.SetAlways("__cli_args", args)
	_ = symbolTable.SetAlways("__static_data_types", staticTypes)

	if interactive {
		_ = symbolTable.SetAlways("__exec_mode", "interactive")
	} else {
		_ = symbolTable.SetAlways("__exec_mode", "run")
	}

	if c.Boolean("trace") {
		ui.SetLogger(ui.TraceLogger, true)
	}

	// Add the runtime builtins and the function library builtins
	runtime.AddBuiltinPackages(symbolTable)
	functions.AddBuiltins(symbolTable)

	return symbolTable
}
