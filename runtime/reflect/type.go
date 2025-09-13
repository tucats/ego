package reflect

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
)

const (
	builtinLabel = "builtin"
	funcLabel    = "func"
)

// describeType implements the type() function.
func describeType(s *symbols.SymbolTable, args data.List) (any, error) {
	switch v := args.Get(0).(type) {
	case *data.Map:
		return v.Type(), nil

	case *data.Array:
		return data.ArrayType(v.Type()), nil

	case *data.Struct:
		t := v.Type()
		// Is it a struct that is defined by a type? If so, return that type.
		if t.Kind() != data.StructKind {
			return v.Type(), nil
		}

		return t, nil

	case data.Struct:
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

	case *any:
		return data.PointerType(data.InterfaceType), nil

	case func(s *symbols.SymbolTable, args data.List) (any, error):
		return "<" + builtinLabel + ">", nil

	case data.Function:
		return data.FunctionType(&v), nil

	default:
		// If the type can be derived from a package type, do it now.
		typeName := reflect.TypeOf(v).String()
		if parts := strings.Split(typeName, "."); len(parts) == 2 {
			pkg := parts[0]
			typeName := parts[1]

			if pkg == "time" && typeName == "Date" {
				typeName = "Time"
			}

			if pkgData := packages.GetByName(pkg); pkgData != nil {
				s := symbols.GetPackageSymbolTable(pkgData)
				if t, found := s.Get(typeName); found {
					if theType, ok := t.(*data.Type); ok {
						return theType, nil
					}
				}
			}
		}

		// Nope, assume it's one of our types.
		tt := data.TypeOf(v)

		return tt, nil
	}
}
