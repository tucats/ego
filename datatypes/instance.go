package datatypes

import (
	"sync"
)

// InstanceOfKind accepts a kind type indicator, and returns the zero-value
// model of that type. This only applies to base types.
func InstanceOfKind(kind Type) interface{} {
	// Waitgroups and mutexes (and pointers to them) must be uniquely created
	// to satisfy Go requirements for unique instances for any value.
	switch kind.kind {
	case typeKind:
		return kind.InstanceOf(nil)

	case mutexKind:
		return &sync.Mutex{}

	case waitGroupKind:
		return &sync.WaitGroup{}

	case pointerKind:
		switch kind.valueType.kind {
		case mutexKind:
			mt := &sync.Mutex{}

			return &mt

		case waitGroupKind:
			wg := &sync.WaitGroup{}

			return &wg
		}

	default:
		for _, typeDef := range TypeDeclarations {
			if typeDef.Kind.IsType(kind) {
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

	return InstanceOfKind(t)
}
