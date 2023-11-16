package reflect

import (
	"reflect"
	"strings"
	"sync"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

const funcLabel = "func"

// describeType implements the type() function.
func describeType(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	switch v := args.Get(0).(type) {
	case *data.Map:
		return v.TypeString(), nil

	case *data.Array:
		return v.TypeString(), nil

	case *data.Struct:
		return v.TypeString(), nil

	case data.Struct:
		return v.TypeString(), nil

	case nil:
		return "nil", nil

	case error:
		return "error", nil

	case *sync.WaitGroup:
		return "sync.Waitgroup", nil

	case **sync.WaitGroup:
		return "*sync.WaitGroup", nil

	case **sync.Mutex:
		return "*sync.Mutex", nil

	case *sync.Mutex:
		return "sync.Mutex", nil

	case *data.Channel:
		return "chan", nil

	case *data.Type:
		typeName := v.String()

		space := strings.Index(typeName, " ")
		if space > 0 {
			typeName = typeName[space+1:]
		}

		return "type " + typeName, nil

	case *data.Package:
		t := data.TypeOf(v)

		if t.IsTypeDefinition() {
			return t.Name(), nil
		}

		return t.String(), nil

	case *interface{}:
		tt := data.TypeOfPointer(v)

		return "*" + tt.String(), nil

	case func(s *symbols.SymbolTable, args data.List) (interface{}, error):
		return "<builtin>", nil

	case data.Function:
		if v.Declaration == nil {
			return funcLabel, nil
		}

		return funcLabel + " " + v.Declaration.Name, nil

	default:
		tt := data.TypeOf(v)
		if tt.IsUndefined() {
			vv := reflect.ValueOf(v)
			if vv.Kind() == reflect.Func {
				return "builtin", nil
			}

			if vv.Kind() == reflect.Ptr {
				ts := vv.String()
				if ts == defs.ByteCodeReflectionTypeString {
					return funcLabel, nil
				}

				return "ptr " + ts, nil
			}

			return "unknown", nil
		}

		vv := reflect.ValueOf(v)
		if vv.Kind() == reflect.Ptr {
			ts := vv.String()
			if ts == defs.ByteCodeReflectionTypeString {
				return funcLabel, nil
			}

			return "ptr " + ts, nil
		}

		return tt.String(), nil
	}
}
