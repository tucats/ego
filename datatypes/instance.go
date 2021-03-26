package datatypes

import (
	"sync"
)

// InstanceOfKind accepts a kind type indicator, and returns the zero-value
// model of that type. This only applies to base types.
func InstanceOfKind(kind Type) interface{} {
	// Waitgroups and mutexes (and pointers to them) must be uniquely created
	// to satisfy Go requirements for unique instances for any value.
	switch kind.Kind {
	case userKind:
		return kind.InstanceOf(nil)

	case mutexKind:
		return &sync.Mutex{}

	case waitGroupKind:
		return &sync.WaitGroup{}

	case pointerKind:
		switch kind.ValueType.Kind {
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
	if t.Kind == userKind {
		return t.ValueType.InstanceOf(&t)
	}

	if t.Kind == structKind {
		result := map[string]interface{}{}

		if superType == nil {
			superType = &StructType
		}

		SetMetadata(result, TypeMDKey, *superType)

		for fieldName, fieldType := range t.Fields {
			result[fieldName] = fieldType.InstanceOf(nil)
		}

		return result
	}

	if t.Kind == arrayKind {
		result := NewArray(*t.ValueType, 0)

		return result
	}

	if t.Kind == mapKind {
		result := NewMap(*t.KeyType, *t.ValueType)

		return result
	}

	return InstanceOfKind(t)
}
