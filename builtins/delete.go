package builtins

import (
	"fmt"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Delete can be used three ways. To delete a member from a structure, to delete
// an element from an array by index number, or to delete a symbol entirely. The
// first form requires a string name, the second form requires an integer index,
// and the third form does not have a second parameter.
func Delete(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if _, ok := args.Get(0).(string); ok {
		if args.Len() != 1 {
			return nil, errors.ErrArgumentCount.In("delete")
		}
	} else {
		if args.Len() != 2 {
			return nil, errors.ErrArgumentCount.In("delete")
		}
	}

	switch v := args.Get(0).(type) {
	case string:
		if !extensions() {
			return nil, errors.ErrArgumentType.In("delete").Context("argument 1: string")
		}

		return nil, s.Delete(v, false)

	case *data.Map:
		_, err := v.Delete(args.Get(1))

		return v, err

	case *data.Array:
		i, err := data.Int(args.Get(1))
		if err != nil {
			return nil, err
		}

		err = v.Delete(i)

		return v, err

	default:
		return nil, errors.ErrInvalidType.In("delete").Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(v).String()))
	}
}
