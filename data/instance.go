package data

import (
	"sync"
)

// InstanceOfType accepts a kind type indicator, and returns the zero-value
// model of that type. This only applies to base types.
func InstanceOfType(t *Type) interface{} {
	switch t.kind {
	case StructKind:
		return NewStruct(t)

	case MapKind:
		return NewMap(t.keyType, t.valueType)

	case ArrayKind:
		return NewArray(t.valueType, 0)

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

// For a given type, create a "zero-instance" of that type. For builtin scalar
// types, it is the same as the InstanceOf() function. However, this can also
// generate structs, maps, arrays, and user type instances as well.
func (t Type) InstanceOf(superType *Type) interface{} {
	switch t.kind {
	case TypeKind:
		return t.valueType.InstanceOf(&t)

	case StructKind:
		if superType == nil {
			superType = StructType
		}

		return NewStruct(superType)

	case ArrayKind:
		return NewArray(t.valueType, 0)

	case MapKind:
		return NewMap(t.keyType, t.valueType)

	case PointerKind:
		return t.valueType.InstanceOf(nil)

	default:
		return InstanceOfType(&t)
	}
}
