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
	"github.com/tucats/gopackages/tokenizer"
)

// RunAction is the command handler for the solve CLI
func RunAction(c *cli.Context) error {

	text := ""
	wasCommandLine := true
	debug := ui.DebugMode
	args := c.Parameters

	if c.GetParameterCount() > 0 {
		fname := c.GetParameter(0)
		content, err := ioutil.ReadFile(fname)
		if err != nil {
			return fmt.Errorf("unable to read file: %s", fname)
		}

		// Convert []byte to string
		text = string(content)
	} else if len(args) == 0 {
		wasCommandLine = false
		fmt.Println("Enter expressions to evaluate. End with a blank line.")
		text = ui.Prompt("solve> ")
	} else {
		var buffer strings.Builder
		for _, v := range args {
			buffer.WriteString(v)
			buffer.WriteRune(' ')
		}
		text = buffer.String()
	}

	// Get a list of all the environment variables and make
	// a symbol map of their lower-case name
	symbols := bytecode.NewSymbolTable("environment variables")
	list := os.Environ()
	for _, env := range list {
		pair := strings.SplitN(env, "=", 2)
		symbols.Set(strings.ToLower(pair[0]), pair[1])
	}

	// Add local funcion(s)
	symbols.Set("pi()", FunctionPi)

	exitValue := 0

	for len(strings.TrimSpace(text)) > 0 {

		// Tokenize the input
		t := tokenizer.New(text)

		// Compile the token stream
		b, err := compiler.Compile(t)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			exitValue = 1
		} else {

			if debug {
				b.Disasm()
			}
			// Run the compiled code
			c := bytecode.NewContext(symbols, b)
			err = c.Run()

			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
				exitValue = 2
			}
			exitValue = 0
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
