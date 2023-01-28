package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/runtime/io"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

var sourceType = "file "

// RunAction is the command handler for the ego CLI.
func RunAction(c *cli.Context) error {
	var err error

	if logFile, found := c.String("log-file"); found {
		err := ui.OpenLogFile(logFile, false)
		if err != nil {
			return err
		}
	}

	if err := profile.InitProfileDefaults(); err != nil {
		return err
	}

	programArgs := make([]interface{}, 0)
	mainName := defs.Main
	prompt := strings.TrimSuffix(c.MainProgram, ".exe") + "> "
	debug := c.Boolean("debug")
	text := ""
	wasCommandLine := true
	fullScope := false
	lineNumber := 1
	isProject := false

	entryPoint, _ := c.String("entry-point")
	if entryPoint == "" {
		entryPoint = defs.Main
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
		ui.Active(ui.ByteCodeLogger, true)
	}

	if c.WasFound(defs.OptimizerOption) {
		optimize := "true"
		if !c.Boolean(defs.OptimizerOption) {
			optimize = "false"
		}

		settings.Set(defs.OptimizerSetting, optimize)
	}

	interactive := false

	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, defs.Strict, defs.Loose, defs.Dynamic) - 1
	if value, found := c.Keyword(defs.TypingOption); found {
		staticTypes = value
	}

	argc := c.ParameterCount()
	ui.Log(ui.CLILogger, "Initial parameter count is %d", argc)

	if argc > 0 {
		if c.WasFound("project") {
			projectPath := c.Parameter(0)

			ui.Log(ui.CLILogger, "Read project at %s", projectPath)

			files, err := os.ReadDir(projectPath)
			if err != nil {
				fmt.Printf("Unable to read project file, %v\n", err)
				os.Exit(2)
			}

			for _, file := range files {
				if file.IsDir() {
					continue
				}

				if filepath.Ext(file.Name()) == ".ego" {
					b, err := os.ReadFile(file.Name())
					if err == nil {
						ui.Log(ui.CompilerLogger, "Reading project file %s", file.Name())

						text = text + "\n@file " + strconv.Quote(filepath.Base(file.Name())) + "\n"
						text = text + "@line 1\n" + string(b)
					}
				}
			}

			if text == "" {
				fmt.Println("No source files found in project directory")
				os.Exit(2)
			}

			mainName, _ = filepath.Abs(projectPath)
			mainName = filepath.Base(mainName) + string(filepath.Separator)
			sourceType = "project "
			text = text + "\n@entrypoint " + entryPoint
			isProject = true
		} else {
			fileName := c.Parameter(0)

			ui.Log(ui.CLILogger, "Read source file %s", fileName)

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
				if content, e1 := os.ReadFile(fileName); e1 != nil {
					if content, e2 := os.ReadFile(fileName + defs.EgoFilenameExtension); e2 != nil {
						return errors.NewError(e1).Context(fileName)
					} else {
						text = string(content)
					}
				} else {
					text = string(content)
				}

				mainName = fileName
				text = text + "\n@entrypoint " + entryPoint
			}
		}
	}

	// Remaining command line arguments are stored
	ui.Log(ui.CLILogger, "Processing remaining arguments (%d)", argc)

	if argc > 1 {
		programArgs = make([]interface{}, argc-1)

		for n := 1; n < argc; n = n + 1 {
			programArgs[n-1] = c.Parameter(n)
		}

		ui.Log(ui.CLILogger, "Saving CLI parameters %v", programArgs)
	} else if argc == 0 {
		wasCommandLine = false

		ui.Log(ui.CLILogger, "No source given, reading from console")

		if !ui.IsConsolePipe() {
			ui.Log(ui.CLILogger, "Console is not a pipe")
			var banner string

			if settings.Get(defs.NoCopyrightSetting) != defs.True {
				banner = c.AppName + " " + c.Version + " " + c.Copyright
			}

			fmt.Printf("%s\n", banner)

			// If this is the first time through this loop, interactive is still
			// false, but we know we're going to use user input. So this first
			// time through, make the text just be an empty string. This will
			// force the run loop to compile the empty string, which will process
			// all the uuto-imports. In this way, the use of --log TRACE on the
			// command line will handle all the import processing BEFORE the
			// first prompt, so the tracing after the prompt is just for the
			// statement(s) typed in at the prompt.
			//
			// If we already know we're interaactive, this isn't the first time
			// through the loop, and we just prompt the user for statements.
			if !interactive {
				text = ""
			} else {
				text = io.ReadConsoleText(prompt)
			}

			interactive = true
		} else {
			ui.Log(ui.CLILogger, "Console is a pipe")

			wasCommandLine = true // It is a pipe, so no prompting for more!
			interactive = true
			text = ""
			mainName = "<stdin>"

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				text = text + scanner.Text() + " "
			}

			ui.Log(ui.CLILogger, "Source: %s", text)
		}
	}

	// Set up the symbol table.
	symbolTable := initializeSymbols(c, mainName, programArgs, staticTypes, interactive, disassemble)
	symbolTable.Root().SetAlways(defs.MainVariable, defs.Main)

	exitValue := 0

	for {
		// If we are processing interactive console commands, and help is enabled, and this is a
		// "help" command, handle that specially.
		if interactive && settings.GetBool(defs.ExtensionsEnabledSetting) && (strings.HasPrefix(text, "help\n") || strings.HasPrefix(text, "help ")) {
			keys := strings.Split(strings.ToLower(text), " ")

			help(keys)

			text = ""

			continue
		}

		// If we're interactive and not in debug mode, help out
		// by updating the line number in REPL mode.
		if interactive && !debug {
			text = fmt.Sprintf("@line %d;\n%s", lineNumber, text)
			lineNumber = lineNumber + strings.Count(text, "\n") - 1
		}

		// Tokenize the input
		t := tokenizer.New(text, true)

		// If not in command-line mode, see if there is an incomplete quote
		// in the last token, which means we want to prompt for more and
		// re-tokenize
		for !wasCommandLine && len(t.Tokens) > 0 {
			lastToken := t.Tokens[len(t.Tokens)-1]
			if lastToken.Spelling()[0:1] == "`" && lastToken.Spelling()[len(lastToken.Spelling())-1:] != "`" {
				text = text + io.ReadConsoleText("...> ")
				t = tokenizer.New(text, true)
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
				switch v.Spelling() {
				case "{", "(", "[":
					count++

				case "}", ")", "]":
					count--
				}
			}

			if count > 0 {
				text = text + io.ReadConsoleText("...> ")
				t = tokenizer.New(text, true)
				lineNumber++

				continue
			} else {
				break
			}
		}

		// If this is the exit command, turn off the debugger to prevent and endless loop
		if t != nil && len(t.Tokens) > 0 && t.Tokens[0].Spelling() == tokenizer.ExitToken.Spelling() {
			debug = false
		}

		// Compile the token stream. Allow the EXIT command only if we are in "run" mode interactively
		if comp == nil {
			comp = compiler.New("run").WithNormalization(settings.GetBool(defs.CaseNormalizedSetting)).ExitEnabled(interactive)

			// link to the global table so we pick up special builtins.
			comp.SetRoot(&symbols.RootSymbolTable)

			err := comp.AutoImport(autoImport, symbolTable)
			if err != nil {
				ui.WriteLog(ui.InternalLogger, "DEBUG: RunAction() auto-import error %v", err)

				return errors.ErrStop
			}

			comp.AddPackageToSymbols(symbolTable)
			comp.SetInteractive(interactive)
		}

		var b *bytecode.ByteCode

		b, err = comp.Compile(mainName, t)
		if err != nil {
			exitValue = 1
			msg := fmt.Sprintf("%s: %s\n", i18n.L("Error"), err.Error())

			os.Stderr.Write([]byte(msg))
		} else {
			if isProject && !comp.MainSeen() {
				return errors.ErrNoMainPackage
			}

			b.Disasm()

			// Run the compiled code from a new context, configured with the symbol table,
			// token stream, and scope/debug settings.
			ctx := bytecode.NewContext(symbolTable, b).
				SetDebug(debug).
				SetTokenizer(t).
				SetFullSymbolScope(fullScope)

			if ctx.Tracing() {
				ui.Active(ui.DebugLogger, true)
			}

			// If we run under control of the debugger, do that, else just run the context.
			if debug {
				err = debugger.Run(ctx)
			} else {
				err = ctx.Run()
			}

			if errors.Equals(err, errors.ErrStop) {
				err = nil
			}

			exitValue = 0

			if err != nil {
				if egoErr, ok := err.(*errors.Error); ok && !egoErr.Is(errors.ErrExit) {
					exitValue = 2
					msg := fmt.Sprintf("%s: %s\n", i18n.L("Error"), err.Error())

					os.Stderr.Write([]byte(msg))
				}

				// If it was an exit operation, we are done with the REPL loop
				if egoErr, ok := err.(*errors.Error); ok && egoErr.Is(errors.ErrExit) {
					break
				}
			}

			if c.Boolean("symbols") {
				fmt.Println(symbolTable.Format(false))
			}
		}

		// If this came as a program via the command line, do not do REPL loop.
		if wasCommandLine {
			break
		}

		text = io.ReadConsoleText(prompt)
	}

	if exitValue > 0 {
		return errors.ErrTerminatedWithErrors
	}

	return err
}

func initializeSymbols(c *cli.Context, mainName string, programArgs []interface{}, typeEnforcement int, interactive, disassemble bool) *symbols.SymbolTable {
	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable(sourceType + mainName).Shared(true)

	args := data.NewArrayFromArray(data.StringType, programArgs)
	symbolTable.SetAlways(defs.CLIArgumentListVariable, args)

	if typeEnforcement < defs.StrictTypeEnforcement || typeEnforcement > defs.NoTypeEnforcement {
		typeEnforcement = defs.NoTypeEnforcement
	}

	symbolTable.SetAlways(defs.TypeCheckingVariable, typeEnforcement)

	if interactive {
		symbolTable.SetAlways(defs.ModeVariable, "interactive")
	} else {
		symbolTable.SetAlways(defs.ModeVariable, "run")
	}

	if c.Boolean("trace") {
		ui.Active(ui.TraceLogger, true)
	}

	// Add the runtime packags and the builtins functions
	runtime.AddPackages(symbolTable)
	builtins.AddBuiltins(symbolTable)

	return symbolTable
}
