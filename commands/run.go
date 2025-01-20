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
	"github.com/tucats/ego/profiling"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/runtime/io"
	egoOS "github.com/tucats/ego/runtime/os"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

var (
	sourceType = "file "
)

// RunAction is the command handler for the ego CLI. It reads program text from
// either a file, directory, or stdin, and compiles and executes it. If the program
// was being read from the console, then the program will be executed in a REPL
// style. If the program was read from a file, then the program will be executed
// and Ego will exit.
func RunAction(c *cli.Context) error {
	var (
		err            error
		programArgs    = make([]interface{}, 0)
		prompt         = strings.TrimSuffix(c.MainProgram, ".exe") + "> "
		lineNumber     = 1
		wasCommandLine = true
		fullScope      = false
		interactive    = false
		debug          = c.Boolean("debug")
		text           string
		mainName       string
		isProject      bool
		extensions     = settings.GetBool(defs.ExtensionsEnabledSetting)
	)

	// Tell the compiler subsystem if we are debugging this code.
	compiler.DebugMode = debug

	// If we are doing profiling, start the native profiler.
	if c.Boolean("profiling") {
		err = profiling.Profile(profiling.StartAction)
		if err != nil {
			return err
		}
	}

	// Do we enable pprof profiing for development work? This is a hidden option not used
	// by an end-user.
	if filename, found := c.String("pprof"); found {
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
	if err := profile.InitProfileDefaults(profile.RuntimeDefaults); err != nil {
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
	configureSymbolAllocations(c)

	// Get the auto-import setting from the configuration. If it was specified
	// on th ecommand line, override the default.
	autoImport := configureAutoImport(c)

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
	configureOptimizer(c)

	// Override the default value of the case normalization setting if the user
	// specified it on the command line. We require the value to be one of the
	// permitted types of "strict", "relaxed", or "dynamic".
	staticTypes := configureTypeCompliance(c)

	// How many parameters were found on the command line?
	argc := c.ParameterCount()
	ui.Log(ui.CLILogger, "cli.parm.count",
		"count", argc)

	// Load the initial source text from the specified file, directory, or stdin.
	if argc > 0 {
		text, isProject, mainName, err = loadSource(c, entryPoint)
		if err != nil {
			return err
		}
	}

	// Remaining command line arguments are stored
	if argc > 1 {
		programArgs = make([]interface{}, argc-1)

		for n := 1; n < argc; n = n + 1 {
			programArgs[n-1] = c.Parameter(n)
		}

		ui.Log(ui.CLILogger, "cli.parm.saving",
			"parms", programArgs)
	} else if argc == 0 {
		// There were no loose arguments on the command line, so no source was given
		// yet.
		wasCommandLine, interactive, text, mainName = readSourceFromConsoleOrPipe(wasCommandLine, c, interactive, text, prompt, mainName)
	}

	// Set up the symbol table.
	symbolTable := initializeSymbols(c, mainName, programArgs, staticTypes, interactive, autoImport)
	symbolTable.Root().SetAlways(defs.MainVariable, defs.Main)
	symbolTable.Root().SetAlways(defs.ExtensionsVariable, extensions)
	symbolTable.Root().SetAlways(defs.UserCodeRunningVariable, true)

	exitValue, err := runREPL(interactive, extensions, text, debug, lineNumber, wasCommandLine, mainName, isProject, symbolTable, fullScope, c, prompt)

	if exitValue > 0 {
		return errors.ErrTerminatedWithErrors
	}

	return err
}

// When in REPL mode, read the next source statement(s) from the default input. This can be the
// console, or stdin if it's a pipe. The source is returned, along with flags indicating whether
// the input implies the session should be consider interactive.
func readSourceFromConsoleOrPipe(wasCommandLine bool, c *cli.Context, interactive bool, text string, prompt string, mainName string) (bool, bool, string, string) {
	wasCommandLine = false

	ui.Log(ui.CLILogger, "cli.no.source")

	// If the input is not from a pipe, then we are interactive. If it is from a
	// pipe then the pipe is drained from the input source text.
	if !ui.IsConsolePipe() {
		var banner string

		ui.Log(ui.CLILogger, "cli.not.pipe")

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
		ui.Log(ui.CLILogger, "cli.pipe")

		wasCommandLine = true // It is a pipe, so no prompting for more!
		interactive = true
		text = ""
		mainName = "<stdin>"

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text = text + scanner.Text() + " "
		}

		ui.Log(ui.CLILogger, "cli.source",
			"text", text)
	}

	return wasCommandLine, interactive, text, mainName
}

// Get the command lin options for the --type setting. If not present, the default vlaue is taken from the
// configuration profile.
func configureTypeCompliance(c *cli.Context) int {
	staticTypes := settings.GetUsingList(defs.StaticTypesSetting, defs.Strict, defs.Relaxed, defs.Dynamic) - 1
	if value, found := c.Keyword(defs.TypingOption); found {
		staticTypes = value
	}

	return staticTypes
}

// Configure the optimizer setting from the command line.
func configureOptimizer(c *cli.Context) {
	if c.WasFound(defs.OptimizerOption) {
		optimize := "true"
		if !c.Boolean(defs.OptimizerOption) {
			optimize = "false"
		}

		settings.SetDefault(defs.OptimizerSetting, optimize)
	}
}

// Configure automatic import of well-known packages from the command line option.
func configureAutoImport(c *cli.Context) bool {
	autoImport := settings.GetBool(defs.AutoImportSetting)
	if c.WasFound(defs.AutoImportOption) {
		autoImport = c.Boolean(defs.AutoImportOption)
		settings.SetDefault(defs.AutoImportSetting, strconv.FormatBool(autoImport))
	}

	return autoImport
}

// Get the symbol table allocation factor from the command line, or the configu file if not present
// on the command line.
func configureSymbolAllocations(c *cli.Context) {
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
}

func loadSource(c *cli.Context, entryPoint string) (string, bool, string, error) {
	var (
		mainName  string
		isProject bool
		text      string
	)

	if c.WasFound("project") {
		projectPath := c.Parameter(0)

		ui.Log(ui.CLILogger, "cli.project",
			"path", projectPath)

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
					ui.Log(ui.CompilerLogger, "cli.project.file",
						"path", file.Name())

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

		ui.Log(ui.CLILogger, "cli.source.file",
			"path", fileName)

		if fileName == "." {
			text = ""
			mainName = "console"

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				text = text + scanner.Text() + " "
			}
		} else {
			if content, e1 := os.ReadFile(fileName); e1 != nil {
				if content, e2 := os.ReadFile(fileName + defs.EgoFilenameExtension); e2 != nil {
					return "", false, "", errors.New(e1).Context(fileName)
				} else {
					text = string(content)
				}
			} else {
				text = string(content)
			}

			if strings.HasPrefix(text, "#!") {
				if i := strings.Index(text, "\n"); i > 0 {
					text = text[i+1:]
				}
			}

			mainName = fileName
			text = text + "\n@entrypoint " + entryPoint
		}
	}

	return text, isProject, mainName, nil
}

// runREPL creates a compiler and enters a runloop to process input text until an error, an exit status, or the text is exhausted.
func runREPL(interactive bool, extensions bool, text string, debug bool, lineNumber int, wasCommandLine bool, mainName string, isProject bool, symbolTable *symbols.SymbolTable, fullScope bool, c *cli.Context, prompt string) (int, error) {
	comp := compiler.New("run").
		SetNormalization(settings.GetBool(defs.CaseNormalizedSetting)).
		SetExitEnabled(interactive).
		SetDebuggerActive(debug).
		SetRoot(&symbols.RootSymbolTable).
		SetInteractive(interactive)

	if settings.GetBool(defs.AutoImportSetting) {
		_ = comp.AutoImport(true, &symbols.RootSymbolTable)
	} else {
		symbols.RootSymbolTable.SetAlways("__AddPackages", runtime.AddPackage)
	}

	dumpSymbols := c.Boolean("symbols")

	return runLoop(dumpSymbols, lineNumber, interactive, extensions, text, debug, wasCommandLine, mainName, comp, isProject, symbolTable, fullScope, prompt)
}

// runLoop reads input text from the console or input source and repeatedly compiles and executes it, until the input source is exhausted, or an error occurs.
func runLoop(dumpSymbols bool, lineNumber int, interactive bool, extensions bool, text string, debug bool, wasCommandLine bool, mainName string, comp *compiler.Compiler, isProject bool, symbolTable *symbols.SymbolTable, fullScope bool, prompt string) (int, error) {
	var (
		b          *bytecode.ByteCode
		err        error
		exitValue  int
		endRunLoop bool
	)

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
		t, text, lineNumber = inputUntilQuotesBalance(wasCommandLine, t, text, lineNumber)

		// Also, make sure we have a balanced count for {}, (), and [] if we're in interactive
		// mode. If we're unbalanced, this will continue to prompt for more text until the
		// input is balanced, and returns an revised tokenizer. This handles cases where a
		// function definition starts with "{" and ends the line, etc.
		t = inputUntilBlocksBalance(interactive, t, text, lineNumber)

		// If this is the exit command, turn off the debugger to prevent an endless loop
		if t != nil && len(t.Tokens) > 0 && t.Tokens[0].Spelling() == tokenizer.ExitToken.Spelling() {
			debug = false
		}

		// Compile the token stream we have accumulated, using the entrypoint name provided by
		// the user (or defaulting to "main").
		label := "console"
		if mainName != "" {
			label = "main '" + mainName + "'"
		}

		b, err = comp.Compile(label, t)
		if err != nil {
			exitValue = 1
			msg := fmt.Sprintf("%s: %s\n", i18n.L("Error"), err.Error())

			os.Stderr.Write([]byte(msg))
		} else {
			// If this is a project, and there is no main package, we can't run it. Bail out.
			if isProject && !comp.MainSeen() {
				return 0, errors.ErrNoMainPackage
			}

			// Let's run the code we've compiled.
			err = runCompiledCode(b, t, symbolTable, debug, fullScope)

			exitValue, endRunLoop = getExitStatusFromError(err)
			if endRunLoop {
				break
			}

			if dumpSymbols {
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

	return exitValue, err
}

// For a given error, determine if the error state contains a request to exit, in which case the shell exit
// status code is extracted. Also determines if the compiled code requested the end of the run loop, in which
// case the second parameter is returned as true which breaks out of the REPL run loop.
func getExitStatusFromError(err error) (int, bool) {
	exitValue := 0

	if err != nil {
		// If it was an exit operation, we are done with the REPL loop
		if egoErr, ok := err.(*errors.Error); ok {
			if egoErr.Is(errors.ErrExit) {
				return exitValue, true
			}

			exitValue = 2
			msg := fmt.Sprintf("%s: %s\n", i18n.L("Error"), err.Error())

			os.Stderr.Write([]byte(msg))
		}
	}

	return exitValue, false
}

// inputuntilblocksBalance reads from the text file and tokenizes it. If the number of opening and closing blocks, braces, or
// brackets is not balanced, it prompts the user for more input until the blocks are balanced.
func inputUntilBlocksBalance(interactive bool, t *tokenizer.Tokenizer, text string, lineNumber int) *tokenizer.Tokenizer {
	for interactive && len(t.Tokens) > 0 {
		var (
			count        int
			continuation bool
		)

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

		if continuation {
			text = text + io.ReadConsoleText("...> ")
			t = tokenizer.New(text, true)
			lineNumber++

			settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")

			continue
		} else {
			break
		}
	}

	return t
}

// Run the compiled code from the most recent compilation in a new context, with debugging support as needed.
func runCompiledCode(b *bytecode.ByteCode, t *tokenizer.Tokenizer, symbolTable *symbols.SymbolTable, debug bool, fullScope bool) error {
	var err error

	// Clean up the unused parts of the tokenizer resources.
	t.Close()

	// Disassemble the bytecode if requested.
	b.Disasm()

	// Run the compiled code from a new context, configured with the symbol table,
	// token stream, and scope/debug settings.
	ctx := bytecode.NewContext(symbolTable, b).
		SetDebug(debug).
		SetTokenizer(t).
		SetFullSymbolScope(fullScope)

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

	return err
}

// inputuntilquotesbalance reads from the text file and tokenizes it. If the number of opening and closing quotes is not balanced,
// it prompts the user for more input until the quotes are balanced.
func inputUntilQuotesBalance(wasCommandLine bool, t *tokenizer.Tokenizer, text string, lineNumber int) (*tokenizer.Tokenizer, string, int) {
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

	return t, text, lineNumber
}

// initializeSymbols initializes the symbol table with the provided main name, program arguments, type enforcement, etc.
// based on the command line options specified.
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
		ui.Log(ui.InfoLogger, "runtime.autoimport.all")
		runtime.AddPackages(symbolTable)
	} else {
		ui.Log(ui.InfoLogger, "runtime.autoimport.min")
		egoOS.MinimalInitialize(symbolTable)
	}

	builtins.AddBuiltins(symbolTable.Root())

	return symbolTable
}
