package builtins

import (
	"github.com/tucats/ego/data"
)

// MaxDeepCopyDepth specifies the maximum depth that a recursive
// copy will go before failing. Setting this too small will
// prevent complex structures from copying correctly. Setting it
// too large can result in excessive memory consumption.
const MaxDeepCopyDepth = 100

// DeepCopy makes a deep copy of an Ego data type. It should be called with the
// maximum nesting depth permitted (i.e. array index->array->array...). Because
// it calls itself recursively, this is used to determine when to give up and
// stop traversing nested data. The default is MaxDeepCopyDepth.
func DeepCopy(source any, depth int) any {
	if depth < 0 {
		return nil
	}

	switch v := source.(type) {
	case bool:
		return v

	case byte:
		return v

	case int8:
		return v

	case int16:
		return v

	case uint16:
		return v
		
	case uint32:
		return v

	case uint:
		return v

	case uint64:
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
		// BUILTIN-COPY-2 fix: v.Copy() is a shallow copy — any field whose value is
		// itself a pointer (e.g. a nested *data.Array or *data.Map) would share
		// storage between the original and the copy.  We now do a proper recursive
		// deep copy by iterating every field and applying DeepCopy to each value.
		r := v.Copy() // creates a new struct with the same type and field layout

		// FieldNames(false) returns the names of all public fields.
		// We deep-copy each field value so that nested composites are independent.
		for _, fieldName := range v.FieldNames(false) {
			fv, _ := v.Get(fieldName)
			_ = r.Set(fieldName, DeepCopy(fv, depth-1))
		}

		return r

	case *data.Array:
		r := data.NewArray(v.Type(), v.Len())

		for i := 0; i < v.Len(); i++ {
			vv, _ := v.Get(i)
			vv = DeepCopy(vv, depth-1)
			// BUILTIN-COPY-1 fix: write the deep-copied element into RESULT r,
			// not back into SOURCE v.  The original code had v.Set(i, vv) here,
			// which populated the source and left r perpetually empty.
			_ = r.Set(i, vv)
		}

		return r

	case *data.Map:
		r := data.NewMap(v.KeyType(), v.ElementType())

		for _, k := range v.Keys() {
			d, _, _ := v.Get(k)
			_, _ = r.Set(k, DeepCopy(d, depth-1))
		}

		return r

	case *data.Package:
		r := data.Package{}
		keys := v.Keys()

		for _, k := range keys {
			d, _ := v.Get(k)
			r.Set(k, DeepCopy(d, depth-1))
		}

		return &r

	default:
		return v
	}
}
