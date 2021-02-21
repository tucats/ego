package commands

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/io"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Reserved symbol names used for configuration.
const (
	ConfigDisassemble = "disassemble"
	ConfigTrace       = "trace"
)

// QuitCommand is the command that exits console input.
const QuitCommand = "%quit"

// RunAction is the command handler for the ego CLI.
func RunAction(c *cli.Context) *errors.EgoError {
	if err := runtime.InitProfileDefaults(); !errors.Nil(err) {
		return err
	}

	programArgs := make([]interface{}, 0)
	mainName := "main"
	prompt := c.MainProgram + "> "
	debug := c.GetBool("debug")
	text := ""
	wasCommandLine := true
	fullScope := false

	autoImport := persistence.GetBool(defs.AutoImportSetting)
	if c.WasFound(defs.AutoImportSetting) {
		autoImport = c.GetBool(defs.AutoImportOption)
	}

	if c.WasFound(defs.FullSymbolScopeOption) {
		fullScope = c.GetBool(defs.FullSymbolScopeOption)
	}

	disassemble := c.GetBool(defs.DisassembleOption)
	if disassemble {
		ui.DebugMode = true

		ui.SetLogger(ui.ByteCodeLogger, true)
	}

	exitOnBlankLine := false

	v := persistence.Get(defs.ExitOnBlankSetting)
	if v == "true" {
		exitOnBlankLine = true
	}

	staticTypes := persistence.GetUsingList(defs.StaticTypesSetting, "dynamic", "static") == 2

	if c.WasFound(defs.StaticTypesOption) {
		staticTypes = c.GetBool(defs.StaticTypesOption)
	}

	interactive := false

	argc := c.GetParameterCount()
	if argc > 0 {
		fname := c.GetParameter(0)

		// If the input file is "." then we read all of stdin
		if fname == "." {
			text = ""
			mainName = "console"

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				text = text + scanner.Text() + " "
			}
		} else {
			// Otherwise, use the parameter as a filename
			content, err := ioutil.ReadFile(fname)
			if !errors.Nil(err) {
				content, err = ioutil.ReadFile(fname + ".ego")
				if !errors.Nil(err) {
					return errors.New(err).Context(fname)
				}
			}

			mainName = fname
			text = string(content)
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

			if persistence.Get(defs.NoCopyrightSetting) != "true" {
				banner = c.AppName + " " + c.Version + " " + c.Copyright
			}

			if exitOnBlankLine {
				fmt.Printf("%s\nEnter a blank line to exit\n", banner)
			} else {
				fmt.Printf("%s\n", banner)
			}

			text = io.ReadConsoleText(prompt)
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
	syms := initializeSymbols(c, mainName, programArgs, staticTypes, interactive, disassemble)

	exitValue := 0
	builtinsAdded := false

	for {
		// Handle special cases.
		if strings.TrimSpace(text) == QuitCommand {
			break
		}

		if exitOnBlankLine && len(strings.TrimSpace(text)) == 0 {
			break
		}

		if len(text) > 8 && text[:8] == "%include" {
			fname := strings.TrimSpace(text[8:])

			content, err := ioutil.ReadFile(fname)
			if !errors.Nil(err) {
				content, err = ioutil.ReadFile(fname + ".ego")
				if !errors.Nil(err) {
					return errors.New(err).Context(fname)
				}
			}
			// Convert []byte to string
			text = string(content)
		}
		// Tokenize the input
		t := tokenizer.New(text)

		// If not in command-line mode, see if there is an incomplete quote
		// in the last token, which means we want to prompt for more and
		// re-tokenize
		for !wasCommandLine && len(t.Tokens) > 0 {
			lastToken := t.Tokens[len(t.Tokens)-1]
			if lastToken[0:1] == "`" && lastToken[len(lastToken)-1:] != "`" {
				text = text + io.ReadConsoleText("...> ")
				t = tokenizer.New(text)

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
				text = text + io.ReadConsoleText("...> ")
				t = tokenizer.New(text)

				continue
			} else {
				break
			}
		}
		// If this is the exit command, turn off the debugger to prevent and endless loop
		if t != nil && len(t.Tokens) > 0 && t.Tokens[0] == "exit" {
			debug = false
		}

		// Compile the token stream. Allow the EXIT command only if  we are in "run" mode interactively
		comp := compiler.New("run").WithNormalization(persistence.GetBool(defs.CaseNormalizedSetting)).ExitEnabled(interactive)

		b, err := comp.Compile(mainName, t)
		if !errors.Nil(err) {
			fmt.Printf("Error: %s\n", err.Error())

			exitValue = 1
		} else {
			if !builtinsAdded {
				// Add the builtin functions
				comp.AddBuiltins("")

				err := comp.AutoImport(autoImport)
				if !errors.Nil(err) {
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
			ctx := bytecode.NewContext(syms, b).SetDebug(debug)
			oldDebugMode = ui.DebugMode

			ctx.Tracing = io.GetConfig(syms, ConfigTrace)
			if ctx.Tracing {
				ui.DebugMode = true
				ui.SetLogger(ui.DebugLogger, true)
			}

			// If we are doing source tracing of execution, we'll need to link the tokenzier
			// back to the execution context. If you don't need source tracing, you can use
			// the simpler CompileString() function which doesn't require a discrete tokenizer.
			if c.GetBool("source-tracing") || debug {
				ctx.SetTracing(true)
			}

			ctx.SetTokenizer(t)
			ctx.SetFullSymbolScope(fullScope)

			// If we run under control of the debugger, do that, else just run the context.
			if debug {
				err = debugger.Run(ctx)
			} else {
				err = ctx.Run()
			}

			if err.Is(errors.Stop) {
				err = nil
			}

			ui.DebugMode = oldDebugMode

			if !errors.Nil(err) {
				fmt.Printf("Error: %s\n", err.Error())

				exitValue = 2
			} else {
				exitValue = 0
			}
		}

		if c.GetBool("symbols") {
			fmt.Println(syms.Format(false))
		}

		if wasCommandLine {
			break
		}

		text = io.ReadConsoleText(prompt)
	}

	if exitValue > 0 {
		return errors.New(errors.TerminatedWithErrors)
	}

	return nil
}

func initializeSymbols(c *cli.Context, mainName string, programArgs []interface{}, staticTypes, interactive, disassemble bool) *symbols.SymbolTable {
	// Create an empty symbol table and store the program arguments.
	syms := symbols.NewSymbolTable("file " + mainName)

	_ = syms.SetAlways("__cli_args", programArgs)
	_ = syms.SetAlways("__static_data_types", staticTypes)

	if interactive {
		_ = syms.SetAlways("__exec_mode", "interactive")
	} else {
		_ = syms.SetAlways("__exec_mode", "run")
	}

	if c.GetBool("trace") {
		ui.SetLogger(ui.TraceLogger, true)
	}

	traceLogging := ui.ActiveLogger(ui.TraceLogger)

	io.SetConfig(syms, ConfigTrace, c.GetBool("trace") || traceLogging)
	io.SetConfig(syms, ConfigDisassemble, disassemble)

	// Get a list of all the environment variables and make
	// a symbol map of their lower-case names
	if c.GetBool(defs.EnvironmentOption) {
		list := os.Environ()
		for _, env := range list {
			pair := strings.SplitN(env, "=", 2)
			_ = syms.SetAlways(pair[0], pair[1])
		}
	}

	// Add local funcion(s) that extend the Ego function set.
	_ = syms.SetAlways("eval", runtime.Eval)
	_ = syms.SetAlways("prompt", runtime.Prompt)

	runtime.AddBuiltinPackages(syms)

	return syms
}
