package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/app"
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
	egoOS "github.com/tucats/ego/runtime/os"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

var sourceType = "file "

// RunAction is the command handler for the ego CLI. It reads program text from
// either a file, directory, or stdin, and compiles and executes it. If the program
// was being read from the console, then the program will be executed in a REPL
// style. If the program was read from a file, then the program will be executed
// and Ego will exit.
func RunAction(c *cli.Context) error {
	var (
		err            error
		comp           *compiler.Compiler
		b              *bytecode.ByteCode
		programArgs    = make([]interface{}, 0)
		mainName       = defs.Main
		prompt         = strings.TrimSuffix(c.MainProgram, ".exe") + "> "
		debug          = c.Boolean("debug")
		text           = ""
		lineNumber     = 1
		wasCommandLine = true
		fullScope      = false
		isProject      = false
		interactive    = false
		extensions     = settings.GetBool(defs.ExtensionsEnabledSetting)
	)

	if filename, found := c.String("profile"); found {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			return err
		}

		defer pprof.StopCPUProfile()
	}

	// Initialize the runtime library if needed.
	if err := app.LibraryInit(); err != nil {
		return err
	}

	// If the user specified a log file, open it now.
	if logFile, found := c.String("log-file"); found {
		if err := ui.OpenLogFile(logFile, false); err != nil {
			return err
		}
	}

	// Initialize the profile default values if not already set.
	if err := profile.InitProfileDefaults(); err != nil {
		return err
	}

	// Get the default entry point from the command line, if specified.
	// If not, use the default value of "main".
	entryPoint, _ := c.String("entry-point")
	if entryPoint == "" {
		entryPoint = defs.Main
	}

	// Get the allocation factor for symbols from the configuration. If it
	// was specified on the command line, override it.
	symAllocFactor := settings.GetInt(defs.SymbolTableAllocationSetting)
	if symAllocFactor > 0 {
		symbols.SymbolAllocationSize = symAllocFactor
	}

	if c.WasFound(defs.SymbolTableSizeOption) {
		symbols.SymbolAllocationSize, _ = c.Integer(defs.SymbolTableSizeOption)
	}

	// Ensure that the allocation value isn't too small, by ensuring it is at least
	// the value of the minimum allocation size.
	if symbols.SymbolAllocationSize < symbols.MinSymbolAllocationSize {
		symbols.SymbolAllocationSize = symbols.MinSymbolAllocationSize
	}

	// Get the auto-import setting from the configuration. If it was specified
	// on th ecommand line, override the default.
	autoImport := settings.GetBool(defs.AutoImportSetting)
	if c.WasFound(defs.AutoImportOption) {
		autoImport = c.Boolean(defs.AutoImportOption)
		settings.SetDefault(defs.AutoImportSetting, strconv.FormatBool(autoImport))
	}

	// If the user specified that full symbol scopes are to be used, override
	// the default value of false.
	if c.WasFound(defs.FullSymbolScopeOption) {
		fullScope = c.Boolean(defs.FullSymbolScopeOption)
	}

	// If the user specified the "disassemble" option, turn on the disassembler.
	disassemble := c.Boolean(defs.DisassembleOption)
	if disassemble {
		ui.Active(ui.ByteCodeLogger, true)
	}

	// Override the default value of the optimizer setting if the user specified
	// it on the command line.
	if c.WasFound(defs.OptimizerOption) {
		optimize := "true"
		if !c.Boolean(defs.OptimizerOption) {
			optimize = "false"
		}

		settings.SetDefault(defs.OptimizerSetting, optimize)
	}

	// Override the default value of the case normalization setting if the user
	// specified it on the command line. We require the value to be one of the
	// permitted types of "strict", "relaxed", or "dynamic".
	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, defs.Strict, defs.Relaxed, defs.Dynamic) - 1
	if value, found := c.Keyword(defs.TypingOption); found {
		staticTypes = value
	}

	// How many parameters were found on the command line?
	argc := c.ParameterCount()
	ui.Log(ui.CLILogger, "Initial parameter count is %d", argc)

	// If there is at least one parameter, and the "--project" option was specified,
	// it means the first parameter is a directory, and we should read all of the
	// source files in that directory and compile them as a project.
	if argc > 0 {
		if c.WasFound("project") {
			projectPath := c.Parameter(0)

			ui.Log(ui.CLILogger, "Read project at %s", projectPath)

			// Make a list of the files in the directory.
			files, err := os.ReadDir(projectPath)
			if err != nil {
				fmt.Printf("Unable to read project file, %v\n", err)
				os.Exit(2)
			}

			// Read each file in the directory and add them to the source text. Each file
			// is predicated by a @file directive.
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
			// There was no "--project" so the first parameter must be the singular
			// source file we read into the text buffer.
			fileName := c.Parameter(0)

			ui.Log(ui.CLILogger, "Read source file %s", fileName)

			// If the input file is "." then we read from stdin
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
						return errors.New(e1).Context(fileName)
					} else {
						text = string(content)
					}
				} else {
					text = string(content)
				}

				// Special case -- if we are invoked using the interpreter directive, there will be a "she-bang"
				// at the start of the input file. If so, find the next line break and delete up to the line
				// break. This will remove the indicator and the path to Ego from the source text.
				if strings.HasPrefix(text, "#!") {
					if i := strings.Index(text, "\n"); i > 0 {
						text = text[i+1:]
					}
				}

				mainName = fileName
				text = text + "\n@entrypoint " + entryPoint
			}
		}
	}

	// Remaining command line arguments are stored
	if argc > 1 {
		programArgs = make([]interface{}, argc-1)

		for n := 1; n < argc; n = n + 1 {
			programArgs[n-1] = c.Parameter(n)
		}

		ui.Log(ui.CLILogger, "Saving program parameters %v", programArgs)
	} else if argc == 0 {
		// There were no loose arguments on the command line, so no source was given
		// yet.
		wasCommandLine = false

		ui.Log(ui.CLILogger, "No source given, reading from console")

		// If the input is not from a pipe, then we are interactive. If it is from a
		// pipe then the pipe is drained from the input source text.
		if !ui.IsConsolePipe() {
			var banner string

			ui.Log(ui.CLILogger, "Console is not a pipe")

			// Because we're going to prompt for input, see if we are supposed to put out the
			// extended banner with version and copyright information.
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
			// If we already know we're interactive, this isn't the first time
			// through the loop, and we just prompt the user for statements.
			if !interactive {
				text = ""
			} else {
				text = io.ReadConsoleText(prompt)
			}

			interactive = true

			settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")
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
	symbolTable := initializeSymbols(c, mainName, programArgs, staticTypes, interactive, autoImport)
	symbolTable.Root().SetAlways(defs.MainVariable, defs.Main)
	symbolTable.Root().SetAlways(defs.ExtensionsVariable, extensions)
	symbolTable.Root().SetAlways(defs.UserCodeRunningVariable, true)

	exitValue := 0

	for {
		// If we are processing interactive console commands, and help is enabled, and this is a
		// "help" command, handle that specially.
		if interactive && extensions && (strings.HasPrefix(text, "help\n") || strings.HasPrefix(text, "help ")) {
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

				settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")

				continue
			}

			break
		}

		// Also, make sure we have a balanced count for {}, (), and [] if we're in interactive
		// mode.
		for interactive && len(t.Tokens) > 0 {
			var (
				count        int
				continuation bool
			)

			// If the last token is a "." then we must prompt to read more input. Also, if we
			// have unbalanced braces, parens, or brackets, we need to prompt for more input.
			if t.Tokens[len(t.Tokens)-1] == tokenizer.DotToken {
				continuation = true
			} else {
				for _, v := range t.Tokens {
					switch v.Spelling() {
					case "{", "(", "[":
						count++

					case "}", ")", "]":
						count--
					}
				}

				if count > 0 {
					continuation = true
				}
			}

			// If the token stream was unbalanced or incomplete, prompt for more text and append
			// it to the existing text. Then re-tokenize.
			if continuation {
				text = text + io.ReadConsoleText("...> ")
				t = tokenizer.New(text, true)
				lineNumber++

				settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")

				continue
			} else {
				// No continuation needed, so we're done with this loop.
				break
			}
		}

		// If this is the exit command, turn off the debugger to prevent and endless loop
		if t != nil && len(t.Tokens) > 0 && t.Tokens[0].Spelling() == tokenizer.ExitToken.Spelling() {
			debug = false
		}

		// Compile the token stream. Allow the EXIT command only if we are in interactive
		// "run" mode by setting the interactive attribute in the compiler.
		if comp == nil {
			comp = compiler.New("run").SetNormalization(settings.GetBool(defs.CaseNormalizedSetting)).SetExitEnabled(interactive)

			comp.SetRoot(&symbols.RootSymbolTable)
			comp.SetInteractive(interactive)

			if settings.GetBool(defs.AutoImportSetting) {
				_ = comp.AutoImport(true, &symbols.RootSymbolTable)
			} else {
				symbols.RootSymbolTable.SetAlways("__AddPackages", runtime.AddPackage)
			}
		}

		// Compile the token stream we have accumulated, using the entrypoint name provided by
		// the user (or defaulting to "main").
		b, err = comp.Compile(mainName, t)
		if err != nil {
			exitValue = 1
			msg := fmt.Sprintf("%s: %s\n", i18n.L("Error"), err.Error())

			os.Stderr.Write([]byte(msg))
		} else {
			// If this is a project, and there is no main package, we can't run it. Bail out.
			if isProject && !comp.MainSeen() {
				return errors.ErrNoMainPackage
			}

			// Disassemble the bytecode if requested.
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

			// If we run under control of the debugger, use the debugger to run the program
			// so it can handle breakpoints, stepping, etc. Otherwise, just run the program
			// directly.
			if debug {
				err = debugger.Run(ctx)
			} else {
				err = ctx.Run()
			}

			// If the program ended with the "stop" error, it means the bytecode stream ended
			// normally, so we don't want to report an error.
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

		settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")
	}

	if exitValue > 0 {
		return errors.ErrTerminatedWithErrors
	}

	return err
}

func initializeSymbols(c *cli.Context, mainName string, programArgs []interface{}, typeEnforcement int, interactive bool, autoImport bool) *symbols.SymbolTable {
	// Create an empty symbol table and store the program arguments.
	symbolTable := symbols.NewSymbolTable(sourceType + mainName).Shared(true)

	args := data.NewArrayFromInterfaces(data.StringType, programArgs...)
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
	if autoImport {
		ui.Log(ui.InfoLogger, "Auto-importing all default runtime packages")
		runtime.AddPackages(symbolTable)
	} else {
		ui.Log(ui.InfoLogger, "Auto-importing minimum packages")
		egoOS.MinimalInitialize(symbolTable)
	}

	builtins.AddBuiltins(symbolTable.Root())

	return symbolTable
}
