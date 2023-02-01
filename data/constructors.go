package data

// Prebuilt instances for each given type. These can be used to reference
// the type in comparison operations, or to define function parameter and
// return value types, etc.

// UndefinedType is instance of the Undefined type object.
var UndefinedType = &Type{
	name: UndefinedTypeName,
	kind: UndefinedKind,
}

// TypeType is instance of the Type type object.
var TypeType = &Type{
	name: "Type",
	kind: TypeKind,
}

// StructType is an instance of the Struct type.
var StructType = &Type{
	name: StructTypeName,
	kind: StructKind,
}

// InterfaceType is an instance of the Interface type.
var InterfaceType = &Type{
	name: InterfaceTypeName,
	kind: InterfaceKind,
}

// ErrorType is an instance of the Error type.
var ErrorType = &Type{
	name: ErrorTypeName,
	kind: ErrorKind,
}

// VoidType is an instance of the Void type.
var VoidType = &Type{
	name: VoidTypeName,
}

// BoolType is an instance of the Bool type.
var BoolType = &Type{
	name: BoolTypeName,
	kind: BoolKind,
}

// ByteType is an instance of the Byte type.
var ByteType = &Type{
	name: ByteTypeName,
	kind: ByteKind,
}

// Int32Type is an instance of the Int32 type.
var Int32Type = &Type{
	name: Int32TypeName,
	kind: Int32Kind,
}

// IntType is an instance of the Int type.
var IntType = &Type{
	name: IntTypeName,
	kind: IntKind,
}

// Int64Type is an instance of the Int64 type.
var Int64Type = &Type{
	name: Int64TypeName,
	kind: Int64Kind,
}

// Float32Type is an instance of the Float32 type.
var Float32Type = &Type{
	name:      Float32TypeName,
	kind:      Float32Kind,
	keyType:   nil,
	valueType: nil,
}

// Float64Type is an instance of the Float64 type.
var Float64Type = &Type{
	name:      Float64TypeName,
	kind:      Float64Kind,
	keyType:   nil,
	valueType: nil,
}

// StringType is an instance of the String type.
var StringType = &Type{
	name: StringTypeName,
	kind: StringKind,
}

// ChanType is an instance of the Chan type.
var ChanType = &Type{
	name: ChanTypeName,
	kind: ChanKind,
}

// WaitGroupType is an instance of the WaitGroup type.
var WaitGroupType = &Type{
	name: "WaitGroup",
	kind: WaitGroupKind,
}

// MutexType is an instance of the Mutex type.
var MutexType = &Type{
	name: "Mutex",
	kind: MutexKind,
}

// VarArgsType is an instance of the VarArgs type.
var VarArgsType = &Type{
	name: "...",
	kind: VarArgsKind,
}

// Construct a type that is an Array type with a base type
// of the provided type. This essentially creates a nested
// type object.
func ArrayType(t *Type) *Type {
	return &Type{
		name:      "[]",
		kind:      ArrayKind,
		valueType: t,
	}
}

// Construct a Type describing a single function.
func FunctionType(f *Function) *Type {
	return &Type{
		name: f.Declaration.Name,
		kind: FunctionKind,
		functions: map[string]Function{
			f.Declaration.Name: *f,
		},
	}
}

// Construct a type that is a Pointer type that points to
// an instance of the given type.
func PointerType(t *Type) *Type {
	return &Type{
		name:      "*",
		kind:      PointerKind,
		valueType: t,
	}
}

// Construct a type that is a Map type. This includes the type for
// both the key for the map as well as the type of the values stored
// in the map.
func MapType(key, value *Type) *Type {
	return &Type{
		name:      "map",
		kind:      MapKind,
		keyType:   key,
		valueType: value,
	}
}

// Construct a Structure type, with optional field definitions. The
// use of optional arguments to describe the fields of the type can
// be done now, or added later as the type is being built.
func StructureType(fields ...Field) *Type {
	t := Type{
		name:   StructTypeName,
		kind:   StructKind,
		fields: map[string]*Type{},
	}

	for _, field := range fields {
		t.DefineField(field.Name, field.Type)
	}

	return &t
}

// Create a type that is a named Type definition, with the
// given type name and base type. This is used to describe
// any type created by the 'type' statement in Ego code.
func TypeDefinition(name string, base *Type) *Type {
	return &Type{
		name:      name,
		kind:      TypeKind,
		valueType: base,
	}
}

// Construct a type for a Package of the given name.
func PackageType(name string) *Type {
	return &Type{
		name:      name,
		kind:      PackageKind,
		valueType: StructType,
	}
}

// Construct a new instance of an Interface type. This
// includes preparing the internal map used to describe
// the required functions in the Interface specification.
func NewInterfaceType(name string) *Type {
	if name == "" {
		name = "interface{}"
	}

	t := &Type{
		name:      name,
		kind:      InterfaceKind,
		functions: make(map[string]Function),
		valueType: InterfaceType,
	}

	return t
}
