package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tucats/gopackages/expressions"
)

func main() {

	// Build the expression string from arguments

	var text strings.Builder

	if len(os.Args) == 1 {
		fmt.Printf("usage: solve <expression>\n")
		os.Exit(0)
	}
	for _, v := range os.Args[1:] {
		text.WriteString(v)
		text.WriteRune(' ')
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

	// Make an expression handler and evaluate the expression,
	// using the environment symbols already loaded.
	e := expressions.New(text.String())
	v, err := e.Eval(symbols)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%v\n", v)
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
		addend = expressions.Coerce(addend, base)
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
