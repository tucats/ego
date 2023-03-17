package builtins

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Append implements the builtin append() function, which concatenates all the items
// together as an array. The first argument is flattened into the result, and then each
// additional argument is added to the array as-is.
func Append(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	result := make([]interface{}, 0)
	kind := data.InterfaceType

	typeChecking := defs.StrictTypeEnforcement
	if v, found := s.Get(defs.TypeCheckingVariable); found {
		typeChecking = data.Int(v)
	}

	for i, j := range args.Elements() {
		// If this is the first item, and it's an array, process it specially.
		if array, ok := j.(*data.Array); ok && i == 0 {
			if !kind.IsInterface() {
				if err := array.Validate(kind); err != nil {
					return nil, err
				}
			}

			// Flatten the base array (also  []interface{} ) and append to the
			// result directly. If we didn't yet know the type, let's assume it's
			// the type of the array items.
			result = append(result, array.BaseArray()...)

			if kind.IsInterface() {
				kind = array.Type()
			}
		} else if array, ok := j.([]interface{}); ok && i == 0 {
			// We also allow being passed a simple []interface{} in which case it is
			// dumped into the array without further checking.
			result = append(result, array...)
		} else {
			// Verify that the item is compatible with the array type. We only do this if
			// we are in relaxed or strict mode.
			if typeChecking < defs.NoTypeEnforcement && !kind.IsInterface() && !data.TypeOf(j).IsType(kind) {
				// Mismatched type. Do we complain, or fix it? If we can determine the state of
				// the type checking system (stored in the symbol table) and it is an integer
				// value greater than zero, then we are allowed to coerce the value to the
				// appropriate type.
				if typeChecking > defs.StrictTypeEnforcement {
					j = data.Coerce(j, data.InstanceOfType(kind))
				} else {
					// Nope, we are in strict type checking mode, so complain and be done.
					return nil, errors.ErrWrongArrayValueType.In("append")
				}
			}
			result = append(result, j)
		}
	}

	// Manufacture a new array from the result set, and return it as the result.
	return data.NewArrayFromInterfaces(kind, result...), nil
}
