package reflect

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// deepCopy implements the reflect.deepCopy function.
func deepCopy(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	depth := MaxDeepCopyDepth
	if len(args) > 1 {
		depth = data.Int(args[1])
	}

	return recursiveCopy(args[0], depth), nil
}

// DeepCopy makes a deep copy of an Ego data type. It should be called with the
// maximum nesting depth permitted (i.e. array index->array->array...). Because
// it calls itself recursively, this is used to determine when to give up and
// stop traversing nested data. The default is MaxDeepCopyDepth.
func recursiveCopy(source interface{}, depth int) interface{} {
	if depth < 0 {
		return nil
	}

	switch v := source.(type) {
	case bool:
		return v

	case byte:
		return v

	case int32:
		return v

	case int:
		return v

	case int64:
		return v

	case string:
		return v

	case float32:
		return v

	case float64:
		return v

	case *data.Struct:
		return v.Copy()

	case *data.Array:
		r := data.NewArray(v.Type(), v.Len())

		for i := 0; i < v.Len(); i++ {
			vv, _ := v.Get(i)
			vv = recursiveCopy(vv, depth-1)
			_ = v.Set(i, vv)
		}

		return r

	case *data.Map:
		r := data.NewMap(v.KeyType(), v.ElementType())

		for _, k := range v.Keys() {
			d, _, _ := v.Get(k)
			_, _ = r.Set(k, recursiveCopy(d, depth-1))
		}

		return r

	case *data.Package:
		r := data.Package{}
		keys := v.Keys()

		for _, k := range keys {
			d, _ := v.Get(k)
			r.Set(k, recursiveCopy(d, depth-1))
		}

		return &r

	default:
		return v
	}
}
