package data

import (
	"sync"

	"github.com/tucats/ego/app-cli/ui"
)

// InstanceOfType accepts a type object, and returns the zero-value
// model of that type. This only applies to base types.
func InstanceOfType(t *Type) interface{} {
	if t == nil {
		ui.Log(ui.InternalLogger, "Attempt to create instance of nil type")

		return nil
	}

	switch t.kind {
	case InterfaceKind:
		return Wrap(nil)

	case StructKind:
		return NewStruct(t)

	case MapKind:
		return NewMap(t.keyType, t.valueType)

	case ArrayKind:
		return NewArray(t.valueType, 0)

	case TypeKind:
		return t.InstanceOf(nil)

	case PointerKind:
		switch t.valueType.kind {
		case MutexKind:
			mt := &sync.Mutex{}

			return &mt

		case WaitGroupKind:
			wg := &sync.WaitGroup{}

			return &wg

		default:
			vx := t.valueType.InstanceOf(nil)

			return &vx
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
func (t *Type) InstanceOf(superType *Type) interface{} {
	if t == nil {
		ui.Log(ui.InternalLogger, "Attempt to create instance of nil type")

		return nil
	}

	if superType == nil {
		superType = t
	}

	// Is it a native type with a constructor supplied? IF so, use that.
	if t.nativeName != "" {
		if t.newFunction != nil {
			return t.New()
		}
	}

	// Otherise, build an Ego value based on the type.
	switch t.kind {
	case TypeKind:
		if t.valueType == nil {
			return TypeType
		}

		return t.valueType.InstanceOf(t)

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
		return InstanceOfType(t)
	}
}

// NewType creates a new type object with the given name, kind, and base type.
func NewType(name string, kind int, types ...*Type) *Type {
	t := &Type{
		name: name,
		kind: kind,
	}

	if len(types) > 0 {
		t.valueType = types[0]
	}

	if len(types) > 1 {
		t.keyType = types[1]
	}

	return t
}
