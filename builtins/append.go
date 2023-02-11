package builtins

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Append implements the builtin append() function, which concatenates all the items
// together as an array. The first argument is flattened into the result, and then each
// additional argument is added to the array as-is.
func Append(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	result := make([]interface{}, 0)
	kind := data.InterfaceType

	for i, j := range args.Elements() {
		if array, ok := j.(*data.Array); ok && i == 0 {
			if !kind.IsInterface() {
				if err := array.Validate(kind); err != nil {
					return nil, err
				}
			}

			result = append(result, array.BaseArray()...)

			if kind.IsInterface() {
				kind = array.Type()
			}
		} else if array, ok := j.([]interface{}); ok && i == 0 {
			result = append(result, array...)
		} else {
			if !kind.IsInterface() && !data.TypeOf(j).IsType(kind) {
				return nil, errors.ErrWrongArrayValueType.In("append")
			}
			result = append(result, j)
		}
	}

	return data.NewArrayFromInterfaces(kind, result...), nil
}
