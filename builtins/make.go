package builtins

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// Make implements the make() function. The first argument must be a model of the
// array type (using the Go native version), and the second argument is the size.
func Make(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	kind := args.Get(0)
	size := data.Int(args.Get(1))

	// if it's an Ego type, get the model for the type.
	if v, ok := kind.(*data.Type); ok {
		kind = data.InstanceOfType(v)
	} else if egoArray, ok := kind.(*data.Array); ok {
		return egoArray.Make(size), nil
	}

	array := make([]interface{}, size)

	if v, ok := kind.([]interface{}); ok {
		if len(v) > 0 {
			kind = v[0]
		}
	}

	// If the model is a type we know about, let's go ahead and populate the array
	// with specific values.
	switch v := kind.(type) {
	case *data.Channel:
		return data.NewChannel(size), nil

	case *data.Array:
		return v.Make(size), nil

	case []int, int:
		for i := range array {
			array[i] = 0
		}

	case []bool, bool:
		for i := range array {
			array[i] = false
		}

	case []string, string:
		for i := range array {
			array[i] = ""
		}

	case []float64, float64:
		for i := range array {
			array[i] = 0.0
		}
	}

	return array, nil
}
