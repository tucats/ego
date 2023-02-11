package util

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/symbols"
)

// Eval implements the eval() function which accepts a string representation of
// an expression and returns the expression result. This can also be used to convert
// string expressions of structs or arrays.
func Eval(symbols *symbols.SymbolTable, args data.List) (interface{}, error) {
	return expressions.Evaluate(data.String(args.Get(0)), symbols)
}
