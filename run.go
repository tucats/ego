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
	"github.com/tucats/gopackages/functions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/tokenizer"
)

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
		fmt.Println("Enter expressions to evaluate. End with a blank line.")
		text = ui.Prompt("solve> ")
	}

	// Get a list of all the environment variables and make
	// a symbol map of their lower-case name
	symbols := symbols.NewSymbolTable(mainName)
	symbols.Set("_args", programArgs)
	if c.GetBool("environment") {
		list := os.Environ()
		for _, env := range list {
			pair := strings.SplitN(env, "=", 2)
			symbols.Set(strings.ToLower(pair[0]), pair[1])
		}
	}
	// Add the builtin functions
	functions.AddBuiltins(symbols)

	// Add local funcion(s)
	symbols.Set("pi", FunctionPi)

	exitValue := 0

	for len(strings.TrimSpace(text)) > 0 {

		// Handle special cases.
		if text == "%quit" {
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
		b, err := compiler.Compile(t)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			exitValue = 1
		} else {

			if disassemble {
				b.Disasm()
			}
			// Run the compiled code
			ctx := bytecode.NewContext(symbols, b)

			oldDebugMode := ui.DebugMode
			ctx.Tracing = c.GetBool("trace")
			if ctx.Tracing {
				ui.DebugMode = true
			}
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
			fmt.Println(symbols.Format())

		}
		if wasCommandLine {
			break
		}
		text = ui.Prompt("solve> ")
	}

	if exitValue > 0 {
		return errors.New("terminated with errors")
	}
	return nil
}
