package data

import (
	"sync"

	"github.com/tucats/ego/internal/cli/ui"
)

// InstanceOfType returns the zero value for the given type.  A "zero value" is
// the default initial state — 0 for numbers, "" for strings, false for bools,
// an empty struct/map/array for complex types, etc.
//
// The function is the bridge between a *Type description and an actual Go
// value.  Callers use it when they need to allocate a fresh, empty instance
// of a type without knowing its concrete kind at compile time.
//
// The distinction between InstanceOfType (this function) and the method
// (t *Type).InstanceOf() below is subtle:
//   - InstanceOfType handles the base scalar types by scanning the
//     TypeDeclarations table, and handles a few special complex types (pointer
//     to WaitGroup/Mutex, interface, etc.) inline.
//   - (t *Type).InstanceOf() handles user-defined types (TypeKind),
//     native types with constructors (nativeName != ""), and everything else
//     by delegating back to InstanceOfType for scalars.
func InstanceOfType(t *Type) any {
	if t == nil {
		ui.Log(ui.InternalLogger, "runtime.nil.type", nil)

		return nil
	}

	switch t.kind {
	case InterfaceKind:
		// An interface zero value is a nil wrapped inside an Interface struct
		// so that the runtime always has a typed object to work with.
		return Wrap(nil)

	case StructKind:
		// Allocate a new Ego struct using the type definition, which sets up
		// the field names, types, and initial values all in one call.
		return NewStruct(t)

	case MapKind:
		// Return a nil-state map, matching Go's zero value for "var m map[K]V".
		// The key and value types are preserved in the *Map wrapper so that
		// type introspection works correctly before the map is initialized.
		// See data.NewNilMap for the nil-state design rationale.
		return NewNilMap(t.keyType, t.valueType)

	case ArrayKind:
		// A zero-length array of the element type.  Callers can append or
		// resize it after construction.
		return NewArray(t.valueType, 0)

	case TypeKind:
		// TypeKind means this is a user-defined type wrapper (e.g.
		// "type MyInt int").  Ask the type itself to produce an instance of
		// its underlying base type.
		return t.InstanceOf(nil)

	case PointerKind:
		// Two special cases for Go-native synchronization primitives:
		// sync.Mutex and sync.WaitGroup are created here so the Ego sync
		// package can return properly initialized values without needing a
		// custom constructor.
		switch t.valueType.kind {
		case MutexKind:
			mt := &sync.Mutex{}

			return &mt

		case WaitGroupKind:
			wg := &sync.WaitGroup{}

			return &wg

		default:
			// For any other pointer type, create an instance of the pointed-to
			// type and return its address as an any-boxed pointer.
			vx := t.valueType.InstanceOf(nil)

			return &vx
		}

	default:
		// Fix (PERFORMANCE.md Finding 2): for scalar base types (int,
		// float64, string, bool, etc.) dispatch directly on the already-known
		// t.kind instead of linearly scanning the ~39-entry TypeDeclarations
		// table and calling (*Type).IsType() - a general-purpose, recursive
		// type-compatibility check meant for comparing arbitrary structs,
		// arrays, and maps - on each entry until one happens to match.
		//
		// This matters because InstanceOfType is called from data.Coerce on
		// essentially every scalar arithmetic operation, comparison, and
		// typed assignment Ego performs in its default dynamic-typing mode,
		// so this scan used to run millions of times per second in any
		// arithmetic-heavy program. Profiling a tight loop attributed 62.86%
		// of InstanceOfType's own cost to IsType() calls made by this scan.
		//
		// Every case below returns the exact same package-level Model
		// variable (declared in declarations.go) that the OLD scan would
		// eventually have found in TypeDeclarations for that Kind - this is
		// purely a dispatch-strategy change, not a behavior change. Kinds
		// that reach this default branch but have no entry in
		// TypeDeclarations today (e.g. NilKind, PackageKind, FunctionKind)
		// already fell out of the old scan with no match and got nil; they
		// fall through this switch's own default case to the same fallback
		// scan below, so behavior for every possible Kind - matched or not -
		// is unchanged.
		switch t.kind {
		case BoolKind:
			return boolModel
		case ByteKind:
			return byteModel
		case Int8Kind:
			return int8Model
		case Int16Kind:
			return int16Model
		case UInt16Kind:
			return uint16Model
		case Int32Kind:
			return int32Model
		case UInt32Kind:
			return uint32Model
		case IntKind:
			return intModel
		case UIntKind:
			return uintModel
		case Int64Kind:
			return int64Model
		case UInt64Kind:
			return uint64Model
		case Float32Kind:
			return float32Model
		case Float64Kind:
			return float64Model
		case StringKind:
			return stringModel
		case ChanKind:
			return chanModel
		case ErrorKind:
			return errorModel
		}

		// Fallback for any Kind not handled by the fast-path switch above.
		// Nothing reachable through normal Ego program execution is expected
		// to land here (every scalar Kind that can appear as a Coerce/
		// InstanceOfType target is listed above), but this preserves the
		// old, fully general lookup behavior exactly for anything that
		// isn't - a deliberately defensive safety net, not a hot path.
		for _, typeDef := range TypeDeclarations {
			if typeDef.Kind.IsType(t) {
				return typeDef.Model
			}
		}
	}

	return nil
}

// InstanceOf is a method on *Type that creates a zero-value instance of this
// type.  It is similar to InstanceOfType but also handles:
//   - native types with registered constructor functions (nativeName != "")
//   - user type wrappers (TypeKind) that carry a supertype for struct
//     construction
//
// superType is the outermost named type to use when allocating a struct.
// For example, if you have "type Point struct{x,y int}" and call
// InstanceOf on that type, superType is the Point type and is passed to
// NewStruct so the struct carries the right type name.  Passing nil means
// "use self as the super type".
func (t *Type) InstanceOf(superType *Type) any {
	if t == nil {
		ui.Log(ui.InternalLogger, "runtime.nil.type", nil)

		return nil
	}

	if superType == nil {
		superType = t
	}

	// If this type has a registered Go-native constructor, call it.
	// This is set by packages like "time" or "uuid" that wrap native types.
	if t.nativeName != "" {
		if t.newFunction != nil {
			return t.New()
		}
	}

	switch t.kind {
	case TypeKind:
		// TypeKind means this is a "type" declaration.  Recurse into the
		// underlying base type.  If there is no base type, return the type
		// object itself as a placeholder.
		if t.valueType == nil {
			return TypeType
		}

		return t.valueType.InstanceOf(t)

	case StructKind:
		// Use superType (the named user type) so the allocated struct records
		// the right type name and receiver functions.
		if superType == nil {
			superType = StructType
		}

		return NewStruct(superType)

	case ArrayKind:
		return NewArray(t.valueType, 0)

	case MapKind:
		// Return a nil-state map (see InstanceOfType MapKind above for rationale).
		return NewNilMap(t.keyType, t.valueType)

	case PointerKind:
		// For a pointer type, create an instance of the pointed-to type.
		// Note: this does NOT return a pointer to the instance; the caller is
		// expected to box the result in a pointer as needed.
		return t.valueType.InstanceOf(nil)

	default:
		// If superType is a named user type distinct from this scalar base
		// (e.g. "type buzz int32" instantiating its Int32Kind valueType),
		// wrap the zero value so the type identity travels with it, mirroring
		// how the StructKind case above uses superType for the same purpose.
		// Plain "var x int32" is unaffected: there superType == t (both are
		// the shared builtin Int32Type), so no wrapping occurs.
		if superType != t && superType.kind == TypeKind {
			return NewScalar(superType, InstanceOfType(t))
		}

		return InstanceOfType(t)
	}
}

// NewType is a low-level constructor that builds a *Type with the given name,
// kind, and optional value/key types.  It is mainly used by code that builds
// type descriptors programmatically (e.g. the compiler or native package
// definitions).
//
// types[0] — if present, becomes the valueType (element type for arrays,
//             base type for pointer/user types, value type for maps).
// types[1] — if present, becomes the keyType (key type for maps).
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
