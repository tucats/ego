package datatypes

import (
	"sync"
)

func NewInterfaceType() *Type {
	t := &Type{
		name:      "interface{}",
		kind:      InterfaceKind,
		functions: make(map[string]interface{}),
		valueType: &InterfaceType,
	}

	return t
}

// InstanceOfType accepts a kind type indicator, and returns the zero-value
// model of that type. This only applies to base types.
func InstanceOfType(t Type) interface{} {
	switch t.kind {
	case StructKind:
		return NewStruct(t)

	case MapKind:
		return NewMap(*t.keyType, *t.valueType)

	case ArrayKind:
		return NewArray(t, 0)

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

		return NewStruct(*superType)
	}

	if t.kind == ArrayKind {
		result := NewArray(*t.valueType, 0)

		return result
	}

	if t.kind == MapKind {
		result := NewMap(*t.keyType, *t.valueType)

		return result
	}

	if t.kind == PointerKind {
		result := t.valueType.InstanceOf(nil)

		return &result
	}

	return InstanceOfType(t)
}
