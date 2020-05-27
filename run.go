package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/compiler"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/tokenizer"
)

// QuitCommand is the command that exits console input
const QuitCommand = "%quit"

// RunAction is the command handler for the solve CLI
func RunAction(c *cli.Context) error {

	programArgs := make([]interface{}, 0)
	mainName := "main program"
	text := ""
	wasCommandLine := true
	disassemble := c.GetBool("disassemble")
	if disassemble {
		ui.DebugMode = true
	}

	argc := c.GetParameterCount()

	if argc > 0 {
		fname := c.GetParameter(0)
		content, err := ioutil.ReadFile(fname)
		if err != nil {
			content, err = ioutil.ReadFile(fname + ".solve")
			if err != nil {
				return fmt.Errorf("unable to read file: %s", fname)
			}
		}
		mainName = fname
		// Convert []byte to string
		text = string(content)

		// Remaining command line arguments are stored
		if argc > 1 {
			programArgs = make([]interface{}, argc-1)
			for n := 1; n < argc; n = n + 1 {
				programArgs[n-1] = c.GetParameter(n)
			}
		}
	} else if argc == 0 {
		wasCommandLine = false
		fmt.Printf("Enter %s to exit\n", QuitCommand)
		text = readConsoleText()
	}

	// Create an empty symbol table and store the program arguments.
	syms := symbols.NewSymbolTable(mainName)
	syms.SetAlways("_args", programArgs)

	// Get a list of all the environment variables and make
	// a symbol map of their lower-case names
	if c.GetBool("environment") {
		list := os.Environ()
		for _, env := range list {
			pair := strings.SplitN(env, "=", 2)
			syms.SetAlways(strings.ToLower(pair[0]), pair[1])
		}
	}

	// Add local funcion(s)
	syms.SetAlways("pi", FunctionPi)

	exitValue := 0

	for true {

		// Handle special cases.
		if strings.TrimSpace(text) == QuitCommand {
			break
		}

		if len(text) > 8 && text[:8] == "%include" {
			fname := strings.TrimSpace(text[8:])
			content, err := ioutil.ReadFile(fname)
			if err != nil {
				content, err = ioutil.ReadFile(fname + ".solve")
				if err != nil {
					return fmt.Errorf("unable to read file: %s", fname)
				}
			}
			mainName = fname
			// Convert []byte to string
			text = string(content)
		}
		// Tokenize the input
		t := tokenizer.New(text)

		// Compile the token stream
		comp := compiler.New()
		b, err := comp.Compile(t)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			exitValue = 1
		} else {

			// Add the builtin functions
			comp.AddBuiltins("")
			comp.AddPackageToSymbols(syms)
			if disassemble {
				b.Disasm()
			}
			// Run the compiled code
			ctx := bytecode.NewContext(syms, b)

			oldDebugMode := ui.DebugMode
			ctx.Tracing = c.GetBool("trace")
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
			}
			exitValue = 0
		}

		if c.GetBool("symbols") {
			fmt.Println(syms.Format(false))

		}
		if wasCommandLine {
			break
		}
		text = readConsoleText()
	}

	if exitValue > 0 {
		return errors.New("terminated with errors")
	}
	return nil
}

func readConsoleText() string {

	var b strings.Builder

	prompt := "solve> "
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
			prompt = fmt.Sprintf("solve[%d]> ", line)
		} else {
			reading = false
		}
		b.WriteString(text)
		b.WriteString("\n")
	}
	return b.String()
}
