package bytecode

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func callRuntimeFunction(c *Context, function func(*symbols.SymbolTable, data.List) (interface{}, error), savedDefinition *data.Function, fullScope bool, args []interface{}) error {
	var (
		parentTable *symbols.SymbolTable
		err         error
		result      interface{}
	)

	definition := builtins.FindFunction(function)
	name := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	name = strings.Replace(name, "github.com/tucats/ego/", "", 1)

	if definition == nil && savedDefinition != nil && savedDefinition.Declaration != nil {
		definition = synthesizeDefinition(definition, name, savedDefinition)
	}

	if definition != nil {
		fullScope = fullScope || definition.FullScope

		if definition.Declaration != nil {
			if !definition.Declaration.Variadic && definition.Declaration.ArgCount[0] == 0 && definition.Declaration.ArgCount[1] == 0 {
				definition.MinArgCount = len(definition.Declaration.Parameters)
				definition.MaxArgCount = len(definition.Declaration.Parameters)
			}
		}

		if len(args) < definition.MinArgCount || len(args) > definition.MaxArgCount {
			name := builtins.FindName(function)

			return errors.ErrArgumentCount.Context(len(args)).In(name)
		}
	}

	if fullScope {
		parentTable = c.symbols
	} else {
		parentTable = c.symbols.FindNextScope()
	}

	functionSymbols := symbols.NewChildSymbolTable("builtin "+name, parentTable)

	if v, ok := c.popThis(); ok {
		functionSymbols.SetAlways(defs.ThisVariable, v)
	}

	result, err = function(functionSymbols, data.NewList(args...))

	if results, ok := result.(data.List); ok {
		if stackErr := c.push(NewStackMarker("results")); stackErr != nil {
			return stackErr
		}

		for i := results.Len() - 1; i >= 0; i = i - 1 {
			if stackErr := c.push(results.Get(i)); stackErr != nil {
				return stackErr
			}
		}

		return nil
	}

	if definition != nil {
		if ok, err := functionReturnedValueAndError(definition, c, err, result); ok {
			return err
		}
	}

	if err != nil {
		err = c.error(err)
	} else {
		err = c.push(result)
	}

	return err
}

func functionReturnedValueAndError(definition *builtins.FunctionDefinition, c *Context, err error, result interface{}) (bool, error) {
	if definition.HasErrReturn {
		_ = c.push(NewStackMarker("results"))
		_ = c.push(err)
		_ = c.push(result)

		return true, err
	}

	if definition.Declaration != nil {
		if len(definition.Declaration.Returns) == 1 {
			returnType := definition.Declaration.Returns[0]
			if returnType != nil {
				if returnType.Kind() == data.ErrorKind {
					if err == nil {
						_ = c.push(result)
					}

					return true, err
				}
			}
		}
	}

	return false, nil
}

// Synthesize a definition object using the saved object definition. Correct min and max arg counts approriately.
func synthesizeDefinition(definition *builtins.FunctionDefinition, name string, savedDefinition *data.Function) *builtins.FunctionDefinition {
	definition = &builtins.FunctionDefinition{
		Name:        name,
		Declaration: savedDefinition.Declaration,
	}

	if !savedDefinition.Declaration.Variadic {
		if savedDefinition.Declaration.ArgCount[0] == 0 && savedDefinition.Declaration.ArgCount[1] == 0 {
			definition.MinArgCount = len(definition.Declaration.Parameters)
			definition.MaxArgCount = len(definition.Declaration.Parameters)
		} else {
			definition.MinArgCount = savedDefinition.Declaration.ArgCount[0]
			definition.MaxArgCount = savedDefinition.Declaration.ArgCount[1]
		}
	} else {
		definition.MinArgCount = len(savedDefinition.Declaration.Parameters) - 1
		definition.MaxArgCount = 99999
	}

	return definition
}
