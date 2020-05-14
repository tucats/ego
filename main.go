package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/app"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/compiler"
	"github.com/tucats/gopackages/tokenizer"
	"github.com/tucats/gopackages/util"
)

// SolveGrammar handles the command line options
var SolveGrammar = []cli.Option{
	cli.Option{
		LongName:    "file",
		Description: "The name of a file to execute",
		OptionType:  cli.StringType,
	},
	cli.Option{
		LongName:    "disassemble",
		ShortName:   "d",
		Description: "Display a disassembly of the bytecode before execution",
		OptionType:  cli.BooleanType,
	},
}

func main() {
	app := app.New("solve: execute code in the solve language")
	app.SetVersion(1, 0, 0)
	app.SetCopyright("(C) Copyright Tom Cole 2020")

	err := app.Parse(SolveGrammar, os.Args, SolveAction)

	// If something went wrong, report it to the user and force an exit
	// status from the error, else a default General error.
	if err != nil {
		fmt.Printf("Error: %v\n", err.Error())
		if e2, ok := err.(cli.ExitError); ok {
			os.Exit(e2.ExitStatus)
		}
		os.Exit(1)
	}
}

// SolveAction is the command handler for the solve CLI
func SolveAction(c *cli.Context) error {

	text := ""
	wasCommandLine := true
	debug := ui.DebugMode
	args := c.Parameters

	if c.WasFound("file") {
		fname, _ := c.GetString("file")
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
	symbols.Set("pi()", pi)
	symbols.Set("sum()", sum)

	exitValue := 0

	for len(strings.TrimSpace(text)) > 0 {

		// Tokenize the input
		t := tokenizer.New(text)

		// Compile the token stream
		b, err := compiler.Compile(t)
		if err != nil {
			fmt.Printf("Error: compile, %v\n", err)
			exitValue = 1
		} else {

			if debug {
				b.Disasm()
			}
			// Run the compiled code
			c := bytecode.NewContext(symbols, b)
			err = c.Run()

			if err != nil {
				fmt.Printf("Error: execute, %v\n", err)
			}
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

func pi(args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.New("too many arguments to pi()")
	}
	return 3.1415926535, nil
}

func sum(args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("insufficient arguments to sum()")
	}
	base := args[0]
	for _, addend := range args[1:] {
		addend = util.Coerce(addend, base)
		switch addend.(type) {
		case int:
			base = base.(int) + addend.(int)
		case float64:
			base = base.(float64) + addend.(float64)
		case string:
			base = base.(string) + addend.(string)

		case bool:
			base = base.(bool) || addend.(bool)
		}
	}
	return base, nil
}
