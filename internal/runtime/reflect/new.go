package reflect

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// instanceOf implements the reflect.InstanceOf() function. It creates a new
// "zero value" instance of the type of its argument.
//
// The argument can be:
//   - an actual type value (e.g. int, string, or a user-defined type -- see
//     reflect.Type()), in which case the zero value of that type is returned.
//   - an example value of any kind (scalar, struct, array, or map), in which
//     case the zero value of THAT value's type is returned -- not a copy of
//     the value itself. For a struct this means a fresh zero-value instance
//     of the same type; for an array or map, an empty one of the same
//     element/key types.
//   - a channel, which has no zero-value concept of its own and is returned
//     unchanged.
//
// Earlier versions of this function also accepted a string type name (e.g.
// "int") and a raw Go reflect.Kind integer constant. Both were removed: the
// string form is now redundant now that types are first-class Ego values
// (reflect.Type(x) or a bare type name like "int" already produce one), and
// the Kind-integer form was never intentional -- passing a plain Ego int
// value collided with it, since Ego ints and reflect.Kind constants are both
// just Go ints, so e.g. reflect.InstanceOf(3) misinterpreted the value 3 as
// reflect.Kind(3) (Int8) and returned byte(0) instead of an int zero value.
func instanceOf(s *symbols.SymbolTable, args data.List) (any, error) {
	arg := args.Get(0)

	if typeValue, ok := arg.(*data.Type); ok {
		return data.InstanceOfType(typeValue), nil
	}

	if ch, ok := arg.(*data.Channel); ok {
		return ch, nil
	}

	return data.InstanceOfType(data.TypeOf(arg)), nil
}
