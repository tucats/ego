package datatypes

// Prebuilt definitions for each given type.

var UndefinedType = Type{
	name: "undefined",
	kind: undefinedKind,
}

var PackageType = Type{
	name: "package",
	kind: packageKind,
}

var StructType = Type{
	name: "struct",
	kind: structKind,
}

var InterfaceType = Type{
	name:      "interface{}",
	kind:      interfaceKind,
	keyType:   nil,
	valueType: nil,
}

var ErrorType = Type{
	name: "error",
	kind: errorKind,
}

var BoolType = Type{
	name:      "bool",
	kind:      boolKind,
	keyType:   nil,
	valueType: nil,
}

var IntType = Type{
	name:      "int",
	kind:      intKind,
	keyType:   nil,
	valueType: nil,
}

var FloatType = Type{
	name:      "float",
	kind:      floatKind,
	keyType:   nil,
	valueType: nil,
}

var StringType = Type{
	name:      "string",
	kind:      stringKind,
	keyType:   nil,
	valueType: nil,
}

var ChanType = Type{
	name:      "chan",
	kind:      chanKind,
	keyType:   nil,
	valueType: nil,
}

var WaitGroupType = Type{
	name:      "WaitGroup",
	kind:      waitGroupKind,
	keyType:   nil,
	valueType: nil,
}

var MutexType = Type{
	name:      "Mutex",
	kind:      mutexKind,
	keyType:   nil,
	valueType: nil,
}

var VarArgsType = Type{
	name: "...",
	kind: varArgs,
}

// Construct a type that is an array of the given type.
func Array(t Type) Type {
	return Type{
		name:      "[]",
		kind:      arrayKind,
		valueType: &t,
	}
}

// Construct a type that is a pointer to the given type.
func Pointer(t Type) Type {
	return Type{
		name:      "*",
		kind:      pointerKind,
		valueType: &t,
	}
}

// Construct a type that is a map, specifying the key and value types.
func Map(key, value Type) Type {
	return Type{
		name:      "map",
		kind:      mapKind,
		keyType:   &key,
		valueType: &value,
	}
}

// Construct a structure type of the given name, with optional field
// definitions. You can add additional fields using the AddField method
// on the result of this function.
func Structure(name string, fields ...Field) Type {
	t := Type{
		name:   name,
		kind:   structKind,
		fields: map[string]Type{},
	}

	for _, field := range fields {
		_ = t.AddField(field.Name, field.Type)
	}

	return t
}

// Create a type that is a named type definition, with the
// given type name and base type.
func TypeDefinition(name string, base Type) Type {
	return Type{
		name:      name,
		kind:      typeKind,
		valueType: &base,
	}
}

// Construct a type for a package of the given name.
func Package(name string) Type {
	return Type{
		name:      name,
		kind:      packageKind,
		valueType: &StructType,
	}
}
