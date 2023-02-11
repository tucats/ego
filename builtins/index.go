package builtins

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Index implements the index() function.
func Index(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if !extensions() {
		return nil, errors.ErrExtension.Context("index")
	}

	switch arg := args[0].(type) {
	case *data.Array:
		for i := 0; i < arg.Len(); i++ {
			vv, _ := arg.Get(i)
			if reflect.DeepEqual(vv, args[1]) {
				return i, nil
			}
		}

		return -1, nil

	case []interface{}:
		return nil, errors.ErrInvalidType.Context("[]interface{}")

	case *data.Map:
		_, found, err := arg.Get(args[1])

		return found, err

	default:
		v := data.String(args[0])
		p := data.String(args[1])

		return strings.Index(v, p) + 1, nil
	}
}
