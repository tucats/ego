package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/bytecode"
	"github.com/tucats/gopackages/expressions"
	"github.com/tucats/gopackages/tokenizer"
	"github.com/tucats/gopackages/util"
)

func main() {

	text := ""
	wasCommandLine := true
	debug := false

	args := os.Args[1:]
	if len(args) > 0 {
		if args[0] == "-d" {
			debug = true
			args = args[1:]
		}
	}

	if len(args) == 0 {
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

		// Peek ahead to see if this is an assignment
		symbolName := ""
		if t.Peek(2) == ":=" {
			symbolName = t.Next()
			t.Advance(1)
		}

		// Parse an expression with the remaining tokens
		e := expressions.NewWithTokenizer(t)
		if debug {
			ui.DebugMode = true
			e.Disasm()
		}
		v, err := e.Eval(symbols)

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			exitValue = 1
		} else {

			if symbolName > "" {
				symbols.Set(symbolName, v)
			} else {
				fmt.Printf("%s\n", util.Format(v))
			}
		}

		if wasCommandLine {
			break
		}
		text = ui.Prompt("solve> ")
	}

	os.Exit(exitValue)
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
