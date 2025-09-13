package builtins

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// describeType implements the type() function.
func typeOf(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	switch v := args.Get(0).(type) {
	case *data.Map:
		return v.Type(), nil

	case *data.Array:
		return data.ArrayType(v.Type()), nil

	case *data.Struct:
		return v.Type(), nil

	case nil:
		return data.NilType, nil

	case error:
		return data.ErrorType, nil

	case *data.Channel:
		return data.ChanType, nil

	case *data.Type:
		return data.TypeDefinition("type", v), nil

	case *data.Package:
		return data.PackageType(v.Name), nil

	case *interface{}:
		return data.PointerType(data.InterfaceType), nil

	case func(s *symbols.SymbolTable, args data.List) (interface{}, error):
		return "<builtin>", nil

	case data.Function:
		return data.FunctionType(&v), nil

	default:
		// Check to see if this is a package type. If it is a pointer type,
		// strip off the "*" to get the type name.
		typeName := strings.TrimPrefix(reflect.TypeOf(v).String(), "*")

		if parts := strings.Split(typeName, "."); len(parts) == 2 {
			if pkgData, found := s.Get(parts[0]); found {
				if pkg, ok := pkgData.(*data.Package); ok {
					if t, found := pkg.Get(parts[1]); found {
						if theType, ok := t.(*data.Type); ok {
							return theType, nil
						}
					}
				}
			}
		}

		return data.TypeOf(v), nil
	}
}
