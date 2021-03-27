package datatypes

import (
	"sync"
)

// InstanceOfType accepts a kind type indicator, and returns the zero-value
// model of that type. This only applies to base types.
func InstanceOfType(t Type) interface{} {
	switch t.kind {
	case structKind:
		m := map[string]interface{}{}
		SetType(m, t)

		for k, v := range t.fields {
			m[k] = InstanceOfType(v)
		}

		return m

	case mapKind:
		m := NewMap(*t.keyType, *t.valueType)

		return m

	case arrayKind:
		m := NewArray(t, 0)

		return m

	case typeKind:
		return t.InstanceOf(nil)

	case mutexKind:
		return &sync.Mutex{}

	case waitGroupKind:
		return &sync.WaitGroup{}

	case pointerKind:
		switch t.valueType.kind {
		case mutexKind:
			mt := &sync.Mutex{}

			return &mt

		case waitGroupKind:
			wg := &sync.WaitGroup{}

			return &wg
		}

	default:
		// Base types can read the "zero value" from the declarations table.
		for _, typeDef := range TypeDeclarations {
			if typeDef.Kind.IsType(t) {
				return typeDef.Model
			}
		}
	}

	return nil
}

func (t Type) InstanceOf(superType *Type) interface{} {
	if t.kind == typeKind {
		return t.valueType.InstanceOf(&t)
	}

	if t.kind == structKind {
		result := map[string]interface{}{}

		if superType == nil {
			superType = &StructType
		}

		SetMetadata(result, TypeMDKey, *superType)

		for fieldName, fieldType := range t.fields {
			result[fieldName] = fieldType.InstanceOf(nil)
		}

		return result
	}

	if t.kind == arrayKind {
		result := NewArray(*t.valueType, 0)

		return result
	}

	if t.kind == mapKind {
		result := NewMap(*t.keyType, *t.valueType)

		return result
	}

	return InstanceOfType(t)
}
