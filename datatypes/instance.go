package datatypes

import (
	"sync"
)

// InstanceOfType accepts a kind type indicator, and returns the zero-value
// model of that type. This only applies to base types.
func InstanceOfType(t Type) interface{} {
	switch t.kind {
	case StructKind:
		m := NewStruct(t)
		// @tomcole do we need the non-native version any more? OR should this
		// switch based on the native.structure setting?
		/*
			m := map[string]interface{}{}
			SetType(m, t)

			for k, v := range t.fields {
				m[k] = InstanceOfType(v)
			}
		*/
		return m

	case MapKind:
		m := NewMap(*t.keyType, *t.valueType)

		return m

	case ArrayKind:
		m := NewArray(t, 0)

		return m

	case TypeKind:
		return t.InstanceOf(nil)

	case MutexKind:
		return &sync.Mutex{}

	case WaitGroupKind:
		return &sync.WaitGroup{}

	case PointerKind:
		switch t.valueType.kind {
		case MutexKind:
			mt := &sync.Mutex{}

			return &mt

		case WaitGroupKind:
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
	if t.kind == TypeKind {
		return t.valueType.InstanceOf(&t)
	}

	if t.kind == StructKind {
		if superType == nil {
			superType = &StructType
		}

		if NativeStructures {
			return NewStruct(*superType)
		}

		result := map[string]interface{}{}

		SetMetadata(result, TypeMDKey, *superType)

		for fieldName, fieldType := range t.fields {
			result[fieldName] = fieldType.InstanceOf(nil)
		}

		return result
	}

	if t.kind == ArrayKind {
		result := NewArray(*t.valueType, 0)

		return result
	}

	if t.kind == MapKind {
		result := NewMap(*t.keyType, *t.valueType)

		return result
	}

	return InstanceOfType(t)
}
