package datatypes

// Prebuilt definitions for each given type.

var UndefinedType = Type{
	name: "undefined",
	kind: UndefinedKind,
}

var PackageType = Type{
	name: "package",
	kind: PackageKind,
}

var StructType = Type{
	name: "struct",
	kind: StructKind,
}

var InterfaceType = Type{
	name:      "interface{}",
	kind:      InterfaceKind,
	keyType:   nil,
	valueType: nil,
}

var ErrorType = Type{
	name: "error",
	kind: ErrorKind,
}

var BoolType = Type{
	name:      BoolTypeName,
	kind:      BoolKind,
	keyType:   nil,
	valueType: nil,
}

var ByteType = Type{
	name:      ByteTypeName,
	kind:      ByteKind,
	keyType:   nil,
	valueType: nil,
}

var Int32Type = Type{
	name:      Int32TypeName,
	kind:      Int32Kind,
	keyType:   nil,
	valueType: nil,
}

var IntType = Type{
	name:      IntTypeName,
	kind:      IntKind,
	keyType:   nil,
	valueType: nil,
}

var Int64Type = Type{
	name:      Int64TypeName,
	kind:      Int64Kind,
	keyType:   nil,
	valueType: nil,
}

var Float64Type = Type{
	name:      Float64TypeName,
	kind:      Float64Kind,
	keyType:   nil,
	valueType: nil,
}

var Float32Type = Type{
	name:      Float32TypeName,
	kind:      Float32Kind,
	keyType:   nil,
	valueType: nil,
}

var StringType = Type{
	name:      StringTypeName,
	kind:      StringKind,
	keyType:   nil,
	valueType: nil,
}

var ChanType = Type{
	name:      "chan",
	kind:      ChanKind,
	keyType:   nil,
	valueType: nil,
}

var WaitGroupType = Type{
	name:      "WaitGroup",
	kind:      WaitGroupKind,
	keyType:   nil,
	valueType: nil,
}

var MutexType = Type{
	name:      "Mutex",
	kind:      MutexKind,
	keyType:   nil,
	valueType: nil,
}

var VarArgsType = Type{
	name: "...",
	kind: varArgs,
}

// Construct a type that is an array of the given type.
func Array(t *Type) *Type {
	return &Type{
		name:      "[]",
		kind:      ArrayKind,
		valueType: t,
	}
}

// Construct a type that is a pointer to the given type.
func Pointer(t *Type) *Type {
	return &Type{
		name:      "*",
		kind:      PointerKind,
		valueType: t,
	}
}

// Construct a type that is a map, specifying the key and value types.
func Map(key, value *Type) *Type {
	return &Type{
		name:      "map",
		kind:      MapKind,
		keyType:   key,
		valueType: value,
	}
}

// Construct a structure type, with optional field definitions. You
// can later add additional fields using the AddField method.
func Structure(fields ...Field) *Type {
	t := Type{
		name:   "struct",
		kind:   StructKind,
		fields: map[string]*Type{},
	}

	for _, field := range fields {
		t.DefineField(field.Name, field.Type)
	}

	return &t
}

// Create a type that is a named type definition, with the
// given type name and base type.
func TypeDefinition(name string, base *Type) *Type {
	return &Type{
		name:      name,
		kind:      TypeKind,
		valueType: base,
	}
}

// Construct a type for a package of the given name.
func Package(name string) *Type {
	return &Type{
		name:      name,
		kind:      PackageKind,
		valueType: &StructType,
	}
}
