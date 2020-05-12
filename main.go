package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/expressions"
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
	symbols := map[string]interface{}{}
	list := os.Environ()
	for _, env := range list {
		pair := strings.SplitN(env, "=", 2)
		symbols[strings.ToLower(pair[0])] = pair[1]
	}

	// Add local funcion(s)
	symbols["pi()"] = pi
	symbols["sum()"] = sum

	for len(strings.TrimSpace(text)) > 0 {
		// Make an expression handler and evaluate the expression,
		// using the environment symbols already loaded.
		e := expressions.New(text)
		if debug {
			ui.DebugMode = true
			e.Disasm()
		}
		v, err := e.Eval(symbols)

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", util.Format(v))
		if wasCommandLine {
			break
		}
		text = ui.Prompt("solve> ")
	}

	os.Exit(0)
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
