package bytecode

import (
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func callNativeFunction(c *Context, visibility bool, function builtins.NativeFunction, args []interface{}) error {
	var err error
	var parentTable *symbols.SymbolTable
	var result interface{}

	functionName := builtins.GetName(function)

	if visibility {
		parentTable = c.symbols
	} else {
		parentTable = c.symbols.FindNextScope()
	}

	funcSymbols := symbols.NewChildSymbolTable("builtin "+functionName, parentTable)

	if v, ok := c.popThis(); ok {
		funcSymbols.SetAlways(defs.ThisVariable, v)
	}

	result, err = function(funcSymbols, data.NewList(args...))

	if r, ok := result.(data.List); ok {
		_ = c.push(NewStackMarker("results"))
		for i := r.Len() - 1; i >= 0; i = i - 1 {
			_ = c.push(r.Get(i))
		}

		return nil
	}

	if err != nil {
		err = c.error(err).In(builtins.FindName(function))
	} else {
		err = c.push(result)
	}

	return err
}
