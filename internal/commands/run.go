package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"

	// This file imports Ego's own "io" and "errors" packages, so Go's standard
	// packages of the same names are given distinct names here.
	goErrors "errors"
	goIO "io"

	"github.com/tucats/ego/internal/builtins"
	"github.com/tucats/ego/internal/cli/app"
	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/compiler"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/debugger"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/runtime"
	"github.com/tucats/ego/internal/runtime/io"
	egoOS "github.com/tucats/ego/internal/runtime/os"
	"github.com/tucats/ego/internal/runtime/profile"
)

var (
	sourceType = "file "
)

// stdinSourceName is the name reported for a program that was piped in rather
// than read from a file, so that diagnostics have something to call it.
const stdinSourceName = "<stdin>"

// runSession gathers everything the interactive console and the one-shot
// program runner need to know about a single "ego run".
//
// These values used to be passed from function to function as a long list of
// parameters -- runLoop alone took thirteen of them, and runREPL took eleven
// of the same ones purely to hand them on. That made the call sites hard to
// read, and it hid real defects: several parameters were assigned before they
// were ever read, so what looked like an input was really a local variable,
// and one branch that tested such a parameter could never run at all.
//
// Collecting them in one place means each value is written once, where it is
// decided, and read wherever it is needed.
type runSession struct {
	// How the program reached us, and how it should be run.
	text        string // the program source, or the statement being typed
	mainName    string // what to call the source in diagnostics
	prompt      string // the interactive prompt string
	interactive bool   // input comes from a person at a console
	isProject   bool   // source was a directory of files, not one file
	extensions  bool   // Ego language extensions, such as "help", are enabled

	// entryPoint is the function to call once everything has compiled, and
	// entryPointGiven records whether the user named it themselves with
	// --entry-point rather than it being the default of "main". See
	// readPipedSource, where the difference decides whether a piped program
	// is run or merely compiled.
	entryPoint      string
	entryPointGiven bool

	// wasCommandLine is true when the whole program was supplied at once --
	// named on the command line, or piped in -- rather than typed a statement
	// at a time. It is what decides whether the run loop goes round again.
	wasCommandLine bool

	// Options that affect execution.
	debug       bool  // run under the debugger
	fullScope   bool  // make all enclosing scopes visible
	dumpSymbols bool  // print the symbol table after each statement
	sandbox     *bool // force sandbox mode on or off; nil means neither

	// The pieces that do the work.
	comp        *compiler.Compiler
	symbolTable *symbols.SymbolTable

	// lineNumber tracks which line of the session is being compiled, so that
	// diagnostics can name the line the user typed rather than counting from
	// the start of whichever fragment is being compiled. It counts every line
	// the user enters, including the continuation lines of an unfinished block
	// or string. See docs/issues/REPL-1.md.
	lineNumber int
}

// RunAction is the command handler for the ego CLI. It reads program text from
// either a file, directory, or stdin, and compiles and executes it. If the program
// was being read from the console, then the program will be executed in a REPL
// (Read-Eval-Print Loop) style, prompting the user for statements one at a time.
// If the program was read from a file, then the program will be executed and
// Ego will exit.
//
// RunAction is also the default verb for both grammars — if you invoke "ego" with
// a filename and no subcommand, it is treated as "ego run <filename>".
//
// Invoked by:
//
//	Traditional: ego run [<file>...] (also the default when no subcommand is given)
//	Verb:        ego run [<file>...] (also the default when no subcommand is given)
func RunAction(c *cli.Context) error {
	var err error

	// The session collects everything the run needs to know. It starts out
	// describing a program named on the command line, which is the common
	// case; reading from the console adjusts it below.
	session := &runSession{
		prompt:         consolePrompt(c.MainProgram),
		wasCommandLine: true,
		debug:          c.Boolean("debug"),
		extensions:     settings.GetBool(defs.ExtensionsEnabledSetting),
	}

	// Tell the compiler subsystem if we are debugging this code.
	compiler.DebugMode = session.debug

	// Start whichever kinds of profiling were asked for. The returned function
	// finishes them off, and "defer" makes sure it runs however this function
	// returns -- including the error returns below, which is what the old code
	// got wrong by calling os.Exit from inside the source loader.
	stopProfiling, err := startProfiling(c)
	if err != nil {
		return err
	}

	defer stopProfiling()

	// Everything that has to be in place before any Ego code can be compiled:
	// the runtime library, logging, and configuration defaults.
	if err := prepareRuntime(c); err != nil {
		return err
	}

	staticTypes := configureExecutionOptions(c, session)

	// Get the default entry point from the command line, if specified.
	// If not, use the default value of "main".
	entryPoint, entryPointGiven := c.String("entry-point")
	if entryPoint == "" {
		entryPoint = defs.Main
		entryPointGiven = false
	}

	session.entryPoint = entryPoint
	session.entryPointGiven = entryPointGiven

	// How many parameters were found on the command line?
	argc := c.ParameterCount()
	ui.Log(ui.CLILogger, "cli.parm.count", ui.A{
		"count": argc})

	// Load the program: from the file or directory named on the command line
	// if there was one, and from the console or a pipe if there was not.
	if argc > 0 {
		if session.text, session.isProject, session.mainName, err = loadSource(c, entryPoint); err != nil {
			return err
		}
	}

	// Initialize the DSN manager in case the program needs it.
	if err := dsns.Initialize(c); err != nil {
		return errors.New(err)
	}

	programArgs := programArguments(c, argc)

	if argc == 0 {
		if err = session.readSourceFromConsole(c); err != nil {
			return err
		}
	}

	// Set up the symbol table.
	session.symbolTable = initializeSymbols(c, session.mainName, programArgs, staticTypes, session.interactive)
	session.symbolTable.Root().SetAlways(defs.MainVariable, defs.Main)
	session.symbolTable.Root().SetAlways(defs.ExtensionsVariable, session.extensions)
	session.symbolTable.Root().SetAlways(defs.UserCodeRunningVariable, true)

	exitValue, err := session.run(c)

	// A non-zero exit value means the program reported failure. If there is
	// also a specific error, that is the more useful of the two, so it is kept
	// rather than replaced; the previous version of this code always discarded
	// it in favor of the generic "terminated with errors".
	if exitValue > 0 && err == nil {
		err = errors.ErrTerminatedWithErrors
	}

	return err
}

// startProfiling turns on whichever kinds of profiling the command line asked
// for, and returns the function that finishes them off.
//
// There are two independent kinds. Ego's own profiler measures how long each
// Ego statement takes. Go's pprof profiler samples the interpreter itself, and
// is a hidden option meant for working on Ego rather than on Ego programs.
//
// Returning a single cleanup function, rather than leaving the caller to
// remember several, is what makes it safe for any later step to fail: one
// deferred call finishes whatever was started.
func startProfiling(c *cli.Context) (func(), error) {
	stopFuncs := []func(){}

	stopAll := func() {
		for _, stop := range stopFuncs {
			stop()
		}
	}

	// --profile-file implies profiling should run even without --profile also
	// being given: naming a destination file but never collecting any data
	// into it would otherwise silently produce nothing.
	profileFile, hasProfileFile := c.String("profile-file")

	if hasProfileFile {
		// The file is the only destination the user asked for; don't also dump
		// the same data to the console at end-of-run or from any in-script
		// "@profile report"/"dump" call.
		bytecode.SuppressConsoleReport(true)
	}

	if c.Boolean("profile") || hasProfileFile {
		if err := bytecode.ProfileAction(bytecode.StartAction); err != nil {
			return stopAll, err
		}
	}

	if hasProfileFile {
		// Writing the report does not clear the collected data (see
		// WriteProfileReportFile's own comment), so main.go's end-of-run call
		// to PrintProfileReport still prints the same session's data.
		stopFuncs = append(stopFuncs, func() {
			if err := bytecode.WriteProfileReportFile(profileFile); err != nil {
				ui.Log(ui.AppLogger, "app.console.error", ui.A{
					"error": err})
			}
		})
	}

	// This is a hidden option, not used by an end user.
	if filename, found := c.String("pprof"); found {
		f, err := os.Create(filename)
		if err != nil {
			return stopAll, errors.New(err)
		}

		if err := pprof.StartCPUProfile(f); err != nil {
			return stopAll, errors.New(err)
		}

		stopFuncs = append(stopFuncs, pprof.StopCPUProfile)
	}

	return stopAll, nil
}

// prepareRuntime puts in place everything that has to exist before any Ego
// code can be compiled or run.
func prepareRuntime(c *cli.Context) error {
	// Set up the symbol table serialization default. By default this is false
	// for executing program statements from the console or from source. This can
	// be overridden by the EGO_SERIALIZE_SYMBOLTABLES environment variable.
	if flag := os.Getenv(defs.EgoSerializeSymbolTablesEnv); flag != "" {
		symbols.SerializeTableAccess = data.BoolOrFalse(flag)
	} else {
		symbols.SerializeTableAccess = false
	}

	// Set the Finding 17 Tier 2 global-reference cache default (see
	// docs/internals/GLOBALS.md). Defaults to true; only an explicit "false"
	// value should disable it, mirroring ego.compiler.constfold's own
	// unset-vs-explicit-false handling (compiler.go's New()).
	bytecode.GlobalCacheEnabled = true
	if v := settings.Get(defs.GlobalCacheSetting); v != "" {
		bytecode.GlobalCacheEnabled = settings.GetBool(defs.GlobalCacheSetting)
	}

	// Initialize the runtime library directory if needed.
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
	return profile.InitProfileDefaults(profile.RuntimeDefaults)
}

// configureExecutionOptions applies the command line options that change how
// code is compiled and run, and returns the type enforcement level chosen.
func configureExecutionOptions(c *cli.Context, session *runSession) int {
	// Get the allocation factor for symbols from the configuration. If it
	// was specified on the command line, override it.
	configureSymbolAllocations(c)

	// If the user specified that full symbol scopes are to be used, override
	// the default value of false.
	if c.WasFound(defs.FullSymbolScopeOption) {
		session.fullScope = c.Boolean(defs.FullSymbolScopeOption)
	}

	// If the user specified the "disassemble" option, turn on the disassembler.
	if c.Boolean(defs.DisassembleOption) {
		ui.Active(ui.ByteCodeLogger, true)
	}

	// Override the default value of the optimizer setting if the user specified
	// it on the command line.
	configureOptimizer(c)

	// Override the default value of the case normalization setting if the user
	// specified it on the command line. We require the value to be one of the
	// permitted types of "strict", "relaxed", or "dynamic".
	return configureTypeCompliance(c)
}

// programArguments collects the command line parameters that follow the
// program name, which the running Ego program sees as its own arguments.
func programArguments(c *cli.Context, argc int) []any {
	if argc < 2 {
		return make([]any, 0)
	}

	programArgs := make([]any, argc-1)
	for n := 1; n < argc; n++ {
		programArgs[n-1] = c.Parameter(n)
	}

	ui.Log(ui.CLILogger, "cli.parm.saving", ui.A{
		"parms": programArgs})

	return programArgs
}

// readSourceFromConsole works out where an "ego run" with no file named on the
// command line should get its program from, and records the answer in the
// session.
//
// There are two possibilities. If the console is a terminal, the user is going
// to type statements, and the session is interactive. If it is a pipe, the
// whole program is already waiting to be read, and it is run in one piece just
// as a named file would be.
//
// This used to take six parameters and return four values, but four of the six
// were written before they were ever read: they were really local variables
// wearing a parameter's clothes. Writing the results into the session says
// plainly that this function decides these values rather than adjusts them.
func (s *runSession) readSourceFromConsole(c *cli.Context) error {
	s.wasCommandLine = false

	ui.Log(ui.CLILogger, "cli.no.source", nil)

	if ui.IsConsolePipe() {
		return s.readPipedSource()
	}

	ui.Log(ui.CLILogger, "cli.not.pipe", nil)

	// Print the version and copyright banner ahead of the first prompt, unless
	// the user has asked for it to be suppressed.
	if settings.Get(defs.NoCopyrightSetting) != defs.True {
		fmt.Println(c.AppName + " " + c.Version + " " + c.Copyright)
	}

	// Start with empty text rather than prompting for a statement here. The run
	// loop compiles that empty string first, which is what processes all the
	// automatic imports. Doing it before the first prompt means "--log TRACE"
	// shows the import work on its own, and everything traced after the prompt
	// belongs to what the user typed. The run loop does the prompting from
	// then on.
	//
	// This used to be a two-way test on "interactive", whose other branch
	// prompted for input immediately. That branch could never run: this is
	// called exactly once, from a point where interactive is always still
	// false, and its comment describing "this isn't the first time through the
	// loop" referred to a loop that does not exist.
	s.text = ""
	s.interactive = true

	settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")

	return nil
}

// readPipedSource reads a whole program from a pipe on the standard input.
func (s *runSession) readPipedSource() error {
	ui.Log(ui.CLILogger, "cli.pipe", nil)

	// It arrives all at once, so there is nothing to prompt for -- the run loop
	// compiles it and stops, exactly as it does for a named file.
	s.wasCommandLine = true
	s.interactive = true
	s.mainName = stdinSourceName

	text, err := readAllStdin()
	if err != nil {
		return errors.New(err)
	}

	s.text = s.entryPointForPipedSource(text)

	ui.Log(ui.CLILogger, "cli.source", ui.A{
		"text": s.text})

	return nil
}

// entryPointForPipedSource appends the directive that calls a piped program's
// entry point, when the program looks like one.
//
// This is the difference between the two things a pipe can carry. A few loose
// statements -- "echo 'fmt.Println(1+2)' | ego run" -- are meant to be
// executed as they stand, exactly as if they had been typed at the console,
// and there is nothing to call. A complete program, on the other hand,
// declares its work inside a function and would otherwise be compiled and then
// simply discarded, producing no output and a successful exit status, which is
// a thoroughly confusing thing for a program to do.
//
// The two are told apart by looking for the entry point function in the source.
// That is done with the tokenizer rather than by searching the text, so that
// the words "func main(" appearing inside a comment or a string are not
// mistaken for a declaration.
//
// When the user names an entry point themselves with --entry-point, the
// directive is emitted whether or not the function was found. They have said
// plainly that they want it called, so if it is missing they are better served
// by an error saying so than by silence.
//
// Note that this applies only to piped input. Statements typed at the console
// are executed one at a time as they are entered; there is no point at which
// the program is complete and something should be called.
func (s *runSession) entryPointForPipedSource(text string) string {
	if !s.entryPointGiven && !declaresFunction(text, s.entryPoint) {
		return text
	}

	ui.Log(ui.CLILogger, "cli.entrypoint", ui.A{
		"name": s.entryPoint})

	return text + "\n@entrypoint " + s.entryPoint
}

// declaresFunction reports whether the source declares a function with the
// given name, by looking for the three tokens "func", the name, and an opening
// parenthesis, one after another.
//
// Using the tokenizer is what makes this trustworthy. It discards comments
// entirely and returns a whole string literal as a single token, so neither
//
//	// func main() is not written yet
//	message := "call func main() to start"
//
// is mistaken for a declaration. Nor is a function whose name merely begins
// with the same letters, such as "mainLoop", because the name is compared as a
// complete token rather than as a prefix of the text.
func declaresFunction(text string, name string) bool {
	t := tokenizer.New(text, true)

	for i := 0; i+2 < len(t.Tokens); i++ {
		if t.Tokens[i].Spelling() == "func" &&
			t.Tokens[i+1].Spelling() == name &&
			t.Tokens[i+2].Spelling() == "(" {
			return true
		}
	}

	return false
}

// readAllStdin reads everything available on the standard input and returns it
// as one string, with line endings normalized.
//
// This deliberately does not use bufio.Scanner, which is the obvious tool for
// reading input line by line and was what this code used to do. Scanner
// refuses to return a line longer than 64KB, reporting "token too long". The
// old code printed that message and then carried on with the text it had
// managed to read, so a script containing one very long line -- a large
// embedded string or a generated file, say -- was silently cut in half and the
// first half was executed, with a successful exit status. Reading the whole
// stream in one call has no such limit, and the error is returned rather than
// printed and ignored.
//
// Normalizing line endings preserves a property the scanner provided for free:
// bufio.ScanLines strips a carriage return from the end of each line, so a
// script written on Windows and piped in worked. io.ReadAll does no such
// thing, so the conversion is done here instead.
func readAllStdin() (string, error) {
	b, err := goIO.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}

	text := normalizeLineEndings(string(b))

	// Guarantee the text ends with a line ending. The compiler treats the
	// input as a sequence of complete lines, and input that ends without a
	// final newline -- which is easy to produce with "printf" or an editor
	// that does not add one -- would otherwise present its last line
	// differently from every other line.
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	return text, nil
}

// normalizeLineEndings rewrites Windows and classic Mac line endings as the
// line feeds the rest of the code expects. See splitLines in help.go, which
// exists for the same reason and explains the three conventions.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")

	return strings.ReplaceAll(text, "\r", "\n")
}

// Get the command lin options for the --type setting. If not present, the default value
// is taken from the configuration profile.
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
		optimize := 1
		if v, ok := c.Integer(defs.OptimizerOption); ok {
			optimize = v
		}

		settings.SetDefault(defs.OptimizerSetting, strconv.Itoa(optimize))
	}

	// If the optimier level is at least 3, also explicitly enable local variable
	// "register" tracking.
	if settings.GetInt(defs.OptimizerSetting) > 2 {
		settings.SetDefault(defs.RegistersSetting, "true")
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

// Get the symbol table allocation factor from the command line, or the config file if not present
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

// loadSource reads the program text named on the command line. It returns the
// text, whether it came from a project directory, and a name to call the
// source in diagnostics.
func loadSource(c *cli.Context, entryPoint string) (string, bool, string, error) {
	// A parameter of "." used to mean "read the program from standard input".
	// That never actually worked -- the source was read but nothing was ever
	// run, because unlike the named-file case no entry point was appended --
	// and it collides with what "." means to --project, and with what
	// "go run ." means to a Go programmer. It is now a deprecated spelling of
	// "--project .", which is what it was always meant to do.
	if !c.WasFound("project") && c.Parameter(0) == "." {
		ui.Say("msg.run.dot.deprecated")

		return loadProject(".", entryPoint)
	}

	if c.WasFound("project") {
		return loadProject(c.Parameter(0), entryPoint)
	}

	fileName := c.Parameter(0)

	ui.Log(ui.CLILogger, "cli.source.file", ui.A{
		"path": fileName})

	return loadFile(fileName, entryPoint)
}

// loadProject reads every Ego source file in a directory and joins them into a
// single unit of text to compile.
//
// The "@file" and "@line" directives inserted ahead of each file tell the
// compiler where that piece of the joined text originally came from, so a
// diagnostic can name the file it belongs to.
func loadProject(projectPath string, entryPoint string) (string, bool, string, error) {
	var text string

	ui.Log(ui.CLILogger, "cli.project", ui.A{
		"path": projectPath})

	files, err := os.ReadDir(projectPath)
	if err != nil {
		// This used to print a message and call os.Exit(2). Exiting here skips
		// every deferred cleanup RunAction registered -- most visibly
		// "defer pprof.StopCPUProfile()", so "ego run --pprof out --project
		// baddir" left a zero-byte, unreadable profile behind. Returning the
		// error lets the normal exit path run.
		return "", false, "", errors.New(err).Context(projectPath)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		sourceFile := filepath.Join(projectPath, file.Name())
		if filepath.Ext(sourceFile) != defs.EgoFilenameExtension {
			continue
		}

		b, err := os.ReadFile(sourceFile)
		if err != nil {
			return "", false, "", errors.New(err).Context(sourceFile)
		}

		ui.Log(ui.CompilerLogger, "cli.project.file", ui.A{
			"path": file.Name()})

		text = text + "\n@file " + strconv.Quote(filepath.Base(file.Name())) + "\n"
		text = text + "@line 1\n" + string(b)
	}

	if text == "" {
		// As above: an error rather than os.Exit(2), so that cleanup runs.
		return "", false, "", errors.ErrNoSourceFiles.Context(projectPath)
	}

	// Name the source after the directory it came from, with a trailing
	// separator so it reads as a directory rather than a file.
	mainName, _ := filepath.Abs(projectPath)
	mainName = filepath.Base(mainName) + string(filepath.Separator)

	sourceType = "project "
	text = text + "\n@entrypoint " + entryPoint

	return text, true, mainName, nil
}

// removeShebang blanks out the "#!" interpreter line that makes a script
// directly executable, so the compiler does not have to make sense of it.
//
// The line is emptied rather than removed. Deleting it would move every
// remaining line of the file up by one, so a program with a shebang reported
// its errors one line earlier than the identical program without one, and
// neither matched what the user counted in their editor. Leaving an empty line
// behind costs nothing and keeps every later line where it belongs. See
// docs/issues/REPL-1.md.
func removeShebang(text string) string {
	if !strings.HasPrefix(text, "#!") {
		return text
	}

	// A file consisting only of a shebang line, with no line ending at all,
	// still has to lose the "#!" text; there is simply no rest of the file.
	i := strings.Index(text, "\n")
	if i < 0 {
		return ""
	}

	return text[i:]
}

// loadFile reads a single Ego source file.
//
// If the name as given does not exist, the standard ".ego" extension is added
// and the read is tried again, so that "ego run hello" finds "hello.ego".
func loadFile(fileName string, entryPoint string) (string, bool, string, error) {
	content, err := os.ReadFile(fileName)
	if err != nil {
		withExtension := fileName + defs.EgoFilenameExtension

		// Note which error is reported when neither name works. The error from
		// the *original* name is the useful one, because that is the name the
		// user actually typed; reporting that "hello.ego" does not exist when
		// they asked for "hello" is more confusing than helpful. But if the
		// file with the extension does exist and simply could not be read --
		// a permissions problem, say -- that error is the real explanation and
		// is reported instead. The previous version of this code always
		// reported the first error, so an unreadable "hello.ego" was described
		// as "hello does not exist".
		alternate, altErr := os.ReadFile(withExtension)
		if altErr != nil {
			if !os.IsNotExist(altErr) {
				return "", false, "", errors.New(altErr).Context(withExtension)
			}

			return "", false, "", errors.New(err).Context(fileName)
		}

		content = alternate
	}

	text := string(content)

	text = removeShebang(text)

	// The entry point directive is what actually causes the program's main
	// function to be called once everything has been compiled.
	text = text + "\n@entrypoint " + entryPoint

	return text, false, fileName, nil
}

// run creates a compiler for this session and executes the program, either
// once or, in interactive mode, a statement at a time until the user is done.
//
// It returns the shell exit status the run produced.
func (s *runSession) run(c *cli.Context) (int, error) {
	s.comp = compiler.New("run").
		SetNormalization(settings.GetBool(defs.CaseNormalizedSetting)).
		SetExitEnabled(!s.wasCommandLine && s.interactive).
		SetDebuggerActive(s.debug).
		SetRoot(&symbols.RootSymbolTable).
		SetInteractive(s.interactive)

	// Make the standard packages available. Importing them all costs startup
	// time, so a user who does not want that gets only the handful that the
	// runtime itself depends on, plus a hook to add the rest on demand.
	if configureAutoImport(c) {
		ui.Log(ui.InfoLogger, "runtime.autoimport.all", nil)

		_ = s.comp.AutoImport(true, s.symbolTable)
	} else {
		ui.Log(ui.InfoLogger, "runtime.autoimport.min", nil)

		s.symbolTable.SetAlways("os", egoOS.OsPackage)
		s.symbolTable.SetAlways("profile", profile.ProfilePackage)
		symbols.RootSymbolTable.SetAlways("__AddPackages", runtime.AddPackage)
	}

	s.dumpSymbols = c.Boolean("symbols")

	// --sandbox=true|false lets a caller (typically automated testing) force
	// sandbox mode on or off for this run, the same restricted mode a
	// server-hosted dashboard "run" session executes untrusted code under
	// (see bytecode.Context.Sandboxed). Nil means the flag was not given, and
	// a plain "ego run" is not sandboxed by default.
	if c.WasFound("sandbox") {
		flag := c.Boolean("sandbox")
		s.sandbox = &flag
	}

	exitValue, err := s.runLoop()
	if err == nil {
		_, err = s.comp.Close()
	}

	return exitValue, err
}

// runLoop compiles and executes the session's text, and in interactive mode
// goes back for the next statement until the user finishes.
//
// Each pass round the loop does four things: deal with anything that is a
// console command rather than Ego source, gather a complete statement, compile
// and run it, and read the next one. Each of those is a function of its own
// below, which is what keeps this loop readable.
func (s *runSession) runLoop() (int, error) {
	var (
		exitValue int
		err       error
	)

	s.lineNumber = 1

	// In interactive mode with nothing to run yet, ask for the first statement.
	if !s.wasCommandLine && s.interactive && strings.TrimSpace(s.text) == "" {
		if err := s.readNextStatement(); err != nil {
			return endInteractiveSession(err)
		}
	}

	for {
		// "help" is a console command rather than Ego source, so it never
		// reaches the compiler.
		if s.handleHelpCommand() {
			continue
		}

		t := s.tokenizeCompleteStatement()

		var done bool

		exitValue, done, err = s.compileAndRun(t)
		if done {
			return exitValue, err
		}

		// A program supplied all at once runs exactly once.
		if s.wasCommandLine {
			return exitValue, err
		}

		if readErr := s.readNextStatement(); readErr != nil {
			return endInteractiveSession(readErr)
		}

		settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")
	}
}

// handleHelpCommand deals with the input if it is a "help" command, and
// reports whether it did.
//
// Only the one line the command occupies is consumed; anything that followed
// it stays in the session text and goes on to be compiled as normal. That
// matters when the input is a pipe, because the whole script was read in one
// piece. See helpCommand in help.go.
func (s *runSession) handleHelpCommand() bool {
	keys, rest, found := helpCommand(s.text)
	if !found || !s.interactive || !s.extensions {
		return false
	}

	help(keys)

	s.text = rest

	return true
}

// tokenizeCompleteStatement turns the session's text into a token stream,
// prompting for more input if what has been typed so far is not yet a complete
// statement.
func (s *runSession) tokenizeCompleteStatement() *tokenizer.Tokenizer {
	// Tell the compiler which line of the session this text came from, so that
	// diagnostics can name the line the user typed rather than counting from
	// the start of this one fragment.
	//
	// The directive goes on a line of its own, so the statement itself always
	// starts on physical line 2 of what the compiler sees; the compiler's
	// lineDirective subtracts that two again. The counter then moves on by the
	// number of lines this fragment occupies, ready for the next statement.
	if s.interactive && !s.debug {
		s.text = fmt.Sprintf("@line %d;\n%s", s.lineNumber, s.text)

		sourceLineCount := strings.Count(s.text, "\n") - 1
		s.lineNumber += sourceLineCount
	}

	t := tokenizer.New(s.text, true)

	// A raw string or a block that the user has not finished typing means the
	// statement is incomplete; both of these prompt for the rest of it. Each
	// hands back the text it gathered and the line number it reached, so that
	// the continuation lines are counted rather than skipped over.
	t, s.text, s.lineNumber = inputUntilQuotesBalance(s.wasCommandLine, t, s.text, s.lineNumber)
	t, s.text, s.lineNumber = inputUntilBlocksBalance(s.interactive, t, s.text, s.lineNumber)

	// "exit" must not be run under the debugger, which would stop on it and
	// wait for a command, leaving no way to leave. The second test catches the
	// same word after the "@line" directive above has been prepended.
	if s.isExitStatement(t) {
		s.debug = false
	}

	return t
}

// isExitStatement reports whether the token stream is the console's "exit"
// command, with or without the "@line" directive prepended to it.
func (s *runSession) isExitStatement(t *tokenizer.Tokenizer) bool {
	if t == nil || len(t.Tokens) == 0 {
		return false
	}

	if t.Tokens[0].Spelling() == tokenizer.ExitToken.Spelling() {
		return true
	}

	return len(t.Tokens) > 4 &&
		t.Tokens[0].Spelling() == "@" &&
		t.Tokens[1].Spelling() == "line" &&
		t.Tokens[3].Spelling() == ";" &&
		t.Tokens[4].Spelling() == "exit"
}

// compileAndRun compiles one statement or program and executes it.
//
// The middle return value is true when the run loop should stop altogether,
// either because the program asked to exit or because it cannot be run at all.
func (s *runSession) compileAndRun(t *tokenizer.Tokenizer) (int, bool, error) {
	label := "console"
	if s.mainName != "" {
		label = "main '" + s.mainName + "'"
	}

	s.comp.Fragment(true)

	b, err := s.comp.Compile(label, t)
	if !errors.Nil(err) {
		// A compilation error is reported and, in interactive mode, the user
		// simply gets another prompt to try again.
		os.Stderr.Write([]byte(fmt.Sprintf("%s: %s\n", i18n.L("Error"), err.Error())))

		return 1, false, nil
	}

	// A project has to have a main package; there is nothing to call otherwise.
	if s.isProject && !s.comp.MainSeen() {
		return 0, true, errors.ErrNoMainPackage
	}

	// An empty statement compiles to nothing, and there is nothing to run.
	if b == nil {
		return 0, false, nil
	}

	err = runCompiledCode(b, t, s.symbolTable, s.debug, s.fullScope, s.sandbox)

	exitValue, endRunLoop := getExitStatusFromError(err)
	if endRunLoop {
		return exitValue, true, nil
	}

	if s.dumpSymbols {
		fmt.Println(s.symbolTable.Format(false))
	}

	// Who reports a runtime error depends on who is going to see it next.
	//
	// The interactive console has nobody to hand it to: the run loop goes
	// straight back round for the next statement, and the error would simply
	// be overwritten. So the console prints it here and reports success, the
	// same as it already does for a compilation error just above.
	//
	// Everything else -- a named file, a project, a piped program -- hands the
	// error back so that main.go prints it once on the way out. The old code
	// printed it *and* handed it back, so a runtime error in a script was
	// reported to the user twice, word for word. See docs/issues/REPL-1.md.
	if err != nil && !s.wasCommandLine {
		os.Stderr.Write([]byte(fmt.Sprintf("%s: %s\n", i18n.L("Error"), err.Error())))

		return exitValue, false, nil
	}

	return exitValue, false, err
}

// readNextStatement prompts for the next line of input and stores it in the
// session. The error it returns means the user has finished, not that
// something went wrong.
func (s *runSession) readNextStatement() error {
	text, err := io.ReadConsoleText(s.prompt)
	if err != nil {
		return err
	}

	s.text = text

	return nil
}

// endInteractiveSession decides how the interactive console should finish when
// the user stops supplying input, and returns the exit status and error the
// run loop should hand back.
//
// Both ways of stopping -- Ctrl-D, and Ctrl-C at the prompt -- end the session,
// which is what typing "exit" does too, so neither is a failure. A newline is
// printed first because the keystroke left the cursor sitting after the prompt,
// and without it the shell's next prompt would be appended to Ego's.
func endInteractiveSession(readErr error) (int, error) {
	fmt.Println()

	if goErrors.Is(readErr, io.ErrInterrupted) {
		ui.Log(ui.CLILogger, "cli.console.interrupt", nil)
	} else {
		ui.Log(ui.CLILogger, "cli.console.eof", nil)
	}

	return 0, nil
}

// consolePrompt builds the interactive prompt string from the program's own
// name, so that a renamed executable prompts with the name the user invoked.
//
// The extension is trimmed without regard to case because Windows reports
// program names in whatever case the file system recorded, and a prompt of
// "EGO.EXE> " helps nobody.
func consolePrompt(programName string) string {
	if len(programName) >= 4 && strings.EqualFold(programName[len(programName)-4:], ".exe") {
		programName = programName[:len(programName)-4]
	}

	return programName + "> "
}

// getExitStatusFromError works out what the error a program finished with
// means for the run: the exit status it implies, and whether the run loop
// should stop altogether.
//
// A request to exit is not a failure -- it is how an Ego program ends itself
// deliberately -- so it stops the loop with a status of zero. Anything else is
// a failure worth a non-zero status.
//
// This used to write the error to stderr as well. Reporting is now left to the
// caller, which is the only place that knows whether the error is also being
// handed back to someone else who will report it. See docs/issues/REPL-1.md.
func getExitStatusFromError(err error) (int, bool) {
	exitValue := 0

	if err != nil {
		// If it was an exit operation, we are done with the REPL loop
		if egoErr, ok := err.(*errors.Error); ok {
			if egoErr.Is(errors.ErrExit) {
				return exitValue, true
			}

			exitValue = 2
		}
	}

	return exitValue, false
}

// inputUntilBlocksBalance reads from the text file and tokenizes it. If the number of opening and closing blocks, braces, or
// brackets is not balanced, it prompts the user for more input until the blocks are balanced.
//
// Like inputUntilQuotesBalance, this returns the text it accumulated and the
// line number it reached, not just the tokenizer. It used to return only the
// tokenizer, keeping the extra lines to itself; the caller's line counter
// therefore stayed where the block started, and the statement typed after a
// three-line block was reported as being two lines earlier than it was. See
// docs/issues/REPL-1.md.
func inputUntilBlocksBalance(interactive bool, t *tokenizer.Tokenizer, text string, lineNumber int) (*tokenizer.Tokenizer, string, int) {
	for interactive && len(t.Tokens) > 0 {
		var (
			count        int
			continuation bool
		)

		if t.Tokens[len(t.Tokens)-1].Is(tokenizer.DotToken) {
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

		if !continuation {
			break
		}

		// Ask for the next line of the block. If the input ends instead --
		// Ctrl-D, or Ctrl-C -- stop asking. Previously the end of the input
		// came back as an empty line, so the brace count never changed and
		// this loop prompted forever with no way out but to kill the process.
		more, readErr := io.ReadConsoleText("...> ")
		if readErr != nil {
			break
		}

		text = text + more
		t = tokenizer.New(text, true)
		lineNumber++

		settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")
	}

	return t, text, lineNumber
}

// Run the compiled code from the most recent compilation in a new context, with debugging support as needed.
func runCompiledCode(b *bytecode.ByteCode, t *tokenizer.Tokenizer, symbolTable *symbols.SymbolTable, debug bool, fullScope bool, sandbox *bool) error {
	var err error

	// Clean up the unused parts of the tokenizer resources.
	t.Close()

	// If there is no code, no foul...
	if b == nil {
		ui.Log(ui.InternalLogger, "runtime.empty.bytecode", nil)

		return nil
	}

	// Disassemble the bytecode if requested.
	b.Disasm(false)

	// Run the compiled code from a new context, configured with the symbol table,
	// token stream, and scope/debug settings.
	ctx := bytecode.NewContext(symbolTable, b).
		SetDebug(debug).
		SetTokenizer(t).
		SetFullSymbolScope(fullScope)

	// --sandbox=true|false (see runREPL) lets a caller force sandbox mode on
	// or off for this run; nil (the flag was not given) leaves a plain
	// "ego run" unsandboxed, as always.
	if sandbox != nil {
		ctx.Sandboxed(*sandbox)
	}

	// If we run under control of the debugger, use the debugger to run the program
	// so it can handle breakpoints, stepping, etc. Otherwise, just run the program
	// directly.
	if debug {
		err = debugger.Run(ctx)
	} else {
		err = ctx.Run()
	}

	// Credit whatever statement was executing when the program stopped with
	// its elapsed time so far. Without this, the very last statement the
	// whole program executes (one that doesn't return into a caller, so
	// callFramePop's own flush never fires for it) would simply have its
	// pending time discarded. A no-op when profiling isn't active.
	ctx.FlushProfileTimer()

	// If the program ended with the "stop" error, it means the bytecode stream ended
	// normally, so we don't want to report an error.
	if errors.Equals(err, errors.ErrStop) {
		err = nil
	}

	return err
}

// inputUntilQuotesBalance reads from the text file and tokenizes it. If the number of opening and closing quotes is not balanced,
// it prompts the user for more input until the quotes are balanced.
func inputUntilQuotesBalance(wasCommandLine bool, t *tokenizer.Tokenizer, text string, lineNumber int) (*tokenizer.Tokenizer, string, int) {
	for !wasCommandLine && len(t.Tokens) > 0 {
		lastToken := t.Tokens[len(t.Tokens)-1]
		spelling := lastToken.Spelling()

		// A raw string that opened with a backtick but has not closed with one
		// is still being typed. The length check guards the slicing below
		// against a token with no text at all.
		if len(spelling) < 1 || spelling[0:1] != "`" || spelling[len(spelling)-1:] == "`" {
			break
		}

		// As in inputUntilBlocksBalance, the end of the input has to stop the
		// loop; otherwise an unterminated string would prompt forever.
		more, readErr := io.ReadConsoleText("...> ")
		if readErr != nil {
			break
		}

		text = text + more
		t = tokenizer.New(text, true)
		lineNumber++

		settings.SetDefault(defs.AllowFunctionRedefinitionSetting, "true")
	}

	return t, text, lineNumber
}

// initializeSymbols initializes the symbol table with the provided main name, program arguments, type enforcement, etc.
// based on the command line options specified.
func initializeSymbols(c *cli.Context, mainName string, programArgs []any, typeEnforcement int, interactive bool) *symbols.SymbolTable {
	// Create an empty symbol table and store the program arguments.
	var name string

	if mainName == "" {
		name = "console globals"
	} else {
		name = sourceType + mainName
	}

	symbolTable := symbols.NewSymbolTable(name).Shared(true)
	symbolTable.SetGlobalSingleton()

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

	builtins.AddBuiltins(symbolTable.Root())

	return symbolTable
}
