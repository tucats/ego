package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/compiler"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/tokenizer"
	"github.com/tucats/gopackages/util"
)

// Reserved symbol names used for configuration
const (
	ConfigDisassemble = "disassemble"
	ConfigTrace       = "trace"
)

// QuitCommand is the command that exits console input
const QuitCommand = "%quit"

// RunAction is the command handler for the ego CLI
func RunAction(c *cli.Context) error {

	programArgs := make([]interface{}, 0)
	mainName := "main program"
	prompt := c.MainProgram + "> "

	autoImport := persistence.GetBool("auto-import")
	if c.WasFound("auto-import") {
		autoImport = c.GetBool("auto-import")
	}

	text := ""
	wasCommandLine := true
	disassemble := c.GetBool("disassemble")
	if disassemble {
		ui.DebugMode = true
	}

	exitOnBlankLine := false
	v := persistence.Get("exit-on-blank")
	if v == "true" {
		exitOnBlankLine = true
	}

	argc := c.GetParameterCount()

	if argc > 0 {
		fname := c.GetParameter(0)

		// If the input file is "." then we read all of stdin
		if fname == "." {
			text = ""
			mainName = "<stdin>"
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				text = text + scanner.Text() + " "
			}
		} else {

			// Otherwise, use the parameter as a filename
			content, err := ioutil.ReadFile(fname)
			if err != nil {
				content, err = ioutil.ReadFile(fname + ".ego")
				if err != nil {
					return fmt.Errorf("unable to read file: %s", fname)
				}
			}
			mainName = fname
			// Convert []byte to string
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
			if persistence.Get("no-copyright") != "true" {
				banner = c.AppName + " " + c.Version + " " + c.Copyright
			}
			if exitOnBlankLine {
				fmt.Printf("%s\nEnter a blank line to exit\n", banner)
			} else {
				fmt.Printf("%s\n", banner)
			}
			text = readConsoleText(prompt)
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

	// Create an empty symbol table and store the program arguments.
	syms := symbols.NewSymbolTable(mainName)

	_ = syms.SetAlways("_args", programArgs)
	setConfig(syms, ConfigDisassemble, disassemble)
	setConfig(syms, ConfigTrace, c.GetBool("trace"))

	// Get a list of all the environment variables and make
	// a symbol map of their lower-case names
	if c.GetBool("environment") {
		list := os.Environ()
		for _, env := range list {
			pair := strings.SplitN(env, "=", 2)
			_ = syms.SetAlways(pair[0], pair[1])
		}
	}

	// Add local funcion(s) that extend the Ego function set. Note that
	// the gremlin open function is placed in a package (a map with special
	// values) so it is addressed as "gremlin.open()" in the Ego source
	_ = syms.SetAlways("eval", FunctionEval)
	_ = syms.SetAlways("table", FunctionTable)
	g := map[string]interface{}{
		"open":       FunctionGremlinOpen,
		"__readonly": true,
	}
	_ = syms.SetAlways("gremlin", g)

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
			if err != nil {
				content, err = ioutil.ReadFile(fname + ".ego")
				if err != nil {
					return fmt.Errorf("unable to read file: %s", fname)
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
				text = text + readConsoleText("...> ")
				t = tokenizer.New(text)
				continue
			}
			break
		}

		// Compile the token stream
		comp := compiler.New().WithNormalization(persistence.GetBool("case-normalized"))

		b, err := comp.Compile(t)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			exitValue = 1
		} else {

			if !builtinsAdded {
				// Add the builtin functions
				comp.AddBuiltins("")
				err := comp.AutoImport(autoImport)
				if err != nil {
					fmt.Printf("Unable to auto-import packages: " + err.Error())
				}
				comp.AddPackageToSymbols(syms)
				builtinsAdded = true
			}
			oldDebugMode := ui.DebugMode
			if getConfig(syms, ConfigDisassemble) {
				ui.DebugMode = true
				b.Disasm()
			}
			ui.DebugMode = oldDebugMode

			// Run the compiled code
			ctx := bytecode.NewContext(syms, b)
			oldDebugMode = ui.DebugMode
			ctx.Tracing = getConfig(syms, ConfigTrace)
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
			ui.DebugMode = oldDebugMode

			if err != nil {
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
		text = readConsoleText(prompt)
	}

	if exitValue > 0 {
		return errors.New("terminated with errors")
	}
	return nil
}

// ReaderInstance is the readline Instance used for console input
var consoleReader *readline.Instance

func readConsoleText(prompt string) string {

	mode := persistence.Get("use-readline")

	// If readline has been explicitly disabled for some reason,
	// do a more primitive input operation.
	// TODO this entire functionality could probably be moved
	// into ui.Prompt() at some point.
	if mode == "off" || mode == "false" {

		var b strings.Builder

		reading := true
		line := 1

		for reading {
			text := ui.Prompt(prompt)
			if len(text) == 0 {
				break
			}
			line = line + 1
			if text[len(text)-1:] == "\\" {
				text = text[:len(text)-1]
				prompt = fmt.Sprintf("ego[%d]> ", line)
			} else {
				reading = false
			}
			b.WriteString(text)
			b.WriteString("\n")
		}
		return b.String()
	}

	// Nope, let's use readline. IF we have never initialized
	// the reader, let's do so now.
	if consoleReader == nil {
		consoleReader, _ = readline.New(prompt)
	}

	if len(prompt) > 1 && prompt[:1] == "~" {
		b, _ := consoleReader.ReadPassword(prompt[1:])
		return string(b)
	}
	// Set the prompt string and do the read. We ignore errors.
	consoleReader.SetPrompt(prompt)
	result, _ := consoleReader.Readline()
	return result + "\n"

}

func setConfig(s *symbols.SymbolTable, name string, value bool) {
	v, found := s.Get("_config")
	if !found {
		m := map[string]interface{}{name: value}
		_ = s.SetAlways("_config", m)
	}
	if m, ok := v.(map[string]interface{}); ok {
		m[name] = value
	}
}

func getConfig(s *symbols.SymbolTable, name string) bool {

	f := false

	v, found := s.Get("_config")
	if found {
		if m, ok := v.(map[string]interface{}); ok {
			f, found := m[name]
			if found {
				return util.GetBool(f)
			}
		}
	}
	return f
}
