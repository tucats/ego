package main

import (
	"errors"

	"github.com/tucats/gopackages/expressions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

// FunctionPi implements the pi() function
func FunctionPi(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.New("too many arguments to pi()")
	}
	return 3.1415926535, nil
}

// FunctionEval implements the pi() function
func FunctionEval(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("wrong number of arguments")
	}
	exprString := util.GetString(args[0])
	e := expressions.New(exprString)
	v, err := e.Eval(symbols)
	return v, err
}
