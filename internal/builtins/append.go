package builtins

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// Append implements the builtin append() function, which concatenates all the items
// together as an array. The first argument is flattened into the result, and then each
// additional argument is added to the array as-is.
func Append(s *symbols.SymbolTable, args data.List) (any, error) {
	var err error

	result := make([]any, 0)
	kind := data.InterfaceType

	// Determine if we are doing strict type checking or not. If the symbol table
	// contains a value for the type checking variable, use that. Otherwise, use
	// the default value.
	typeChecking := defs.StrictTypeEnforcement
	if v, found := s.Get(defs.TypeCheckingVariable); found {
		typeChecking, _ = data.Int(v)
	}

	// scan the arguments. If the first argument is an array, we will use its type
	// to define the target array type. Otherwise, we will use the type of the first
	// item in the array.
	for i, j := range args.Elements() {
		if array, ok := j.(*data.Array); ok && i == 0 {
			if !kind.IsInterface() {
				if err := array.Validate(kind); err != nil {
					return nil, err
				}
			}

			// Flatten the base array (also  []any ) and append to the
			// result directly. If we didn't yet know the type, let's assume it's
			// the type of the array items.
			result = append(result, array.BaseArray()...)

			if kind.IsInterface() {
				kind = array.Type()
			}
		} else if array, ok := j.([]any); ok && i == 0 {
			// We also allow being passed a simple []any in which case it is
			// dumped into the array without further checking.
			result = append(result, array...)

			// BUILTIN-APPEND-1 fix: when the first argument is a raw []any slice
			// the original code left `kind` as data.InterfaceType, so the returned
			// array was always typed as []interface{} regardless of the element
			// values.  We now inspect the first element and set `kind` to its
			// concrete type only when ALL elements share that type — mimicking
			// the behaviour of the *data.Array branch above.  If the slice is
			// empty, or elements have mixed types, kind stays as InterfaceType
			// (which is the correct representation for a heterogeneous array).
			if len(array) > 0 && kind.IsInterface() {
				candidate := data.TypeOf(array[0])
				uniform := true

				for _, elem := range array[1:] {
					if !data.TypeOf(elem).IsType(candidate) {
						uniform = false

						break
					}
				}

				if uniform {
					kind = candidate
				}
			}
		} else {
			// Always enforce the declared element type of a typed array, regardless of
			// the type-checking mode. Previously, dynamic mode (NoTypeEnforcement)
			// skipped this check entirely, allowing values of any type to be silently
			// appended — violating the array's type contract.
			//
			// The type-checking mode still governs *how* a mismatch is handled:
			//   strict (0): reject immediately with ErrWrongArrayValueType
			//   relaxed (1): attempt coercion; error if coercion fails
			//   dynamic (2): same as relaxed — coerce if possible, error if not
			//
			// Interface-typed arrays ([]interface{}) accept any element and are
			// intentionally excluded from this check.
			if !kind.IsInterface() && !data.TypeOf(j).IsType(kind) {
				if typeChecking > defs.StrictTypeEnforcement {
					// Relaxed or dynamic: attempt coercion to the target element type.
					if j, err = data.Coerce(j, data.InstanceOfType(kind)); err != nil {
						return nil, errors.New(err).In("append")
					}
				} else {
					// Strict: reject the mismatched value without attempting coercion.
					return nil, errors.ErrWrongArrayValueType.In("append")
				}
			}

			result = append(result, j)
		}
	}

	// Manufacture a new array from the result set, and return it as the result.
	return data.NewArrayFromInterfaces(kind, result...), nil
}
