package builtins

import (
	"sync"

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
		t := v.Type()
		// Is it a struct that is defined by a type? If so, return that type.
		if t.Kind() != data.StructKind {
			return v.Type(), nil
		}

		return v, nil

	case data.Struct:
		return v.Type(), nil

	case nil:
		return data.NilType, nil

	case error:
		return data.ErrorType, nil

	case *sync.WaitGroup:
		return data.WaitGroupType, nil

	case **sync.WaitGroup:
		return data.PointerType(data.WaitGroupType), nil

	case *sync.Mutex:
		return data.MutexType, nil

	case **sync.Mutex:
		return data.PointerType(data.MutexType), nil

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
		tt := data.TypeOf(v)
		
		return tt, nil
	}
}
