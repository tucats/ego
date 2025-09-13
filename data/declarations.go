package data

import "github.com/tucats/ego/errors"

// This defines the semantic information needed to define a type in Ego. This
// includes the token sequence for the type declaration, a model of that
// type, and the type designation. This is used by the Type compiler to
// parse all base type strings and convert them to the appropriate Type.
type TypeDeclaration struct {
	Tokens []string
	Model  any
	Kind   *Type
}

// This is the "zero instance" value for various builtin types.
var interfaceModel any
var byteModel byte = 0
var int32Model int32 = 0
var intModel int = 0
var int64Model int64 = 0
var float64Model float64 = 0.0
var float32Model float32 = 0.0
var boolModel = false
var stringModel = ""
var chanModel = NewChannel(1)

// These are instances of the zero value of each object, expressed
// as an any.
var byteInterface any = byte(0)
var int32Interface any = int32(0)
var intInterface any = int(0)
var int64Interface any = int64(0)
var boolInterface any = false
var float64Interface any = 0.0
var float32Interface any = float32(0.0)
var stringInterface any = ""

// TypeDeclarations is a dictionary of all the type declaration token sequences.
// This includes _Ego_ types and also native types, such as sync.WaitGroup.  There
// should be a type in InstanceOf to match each of these types.
var TypeDeclarations = []TypeDeclaration{
	{
		[]string{"chan"},
		chanModel,
		ChanType,
	},
	{
		[]string{ErrorTypeName},
		&errors.Error{},
		ErrorType,
	},
	{
		[]string{"[", "]", ByteTypeName},
		NewArray(ByteType, 0),
		ArrayType(ByteType),
	},
	{
		[]string{"[", "]", Int32TypeName},
		NewArray(Int32Type, 0),
		ArrayType(Int32Type),
	},
	{
		[]string{"[", "]", IntTypeName},
		NewArray(IntType, 0),
		ArrayType(IntType),
	},
	{
		[]string{"[", "]", Int64TypeName},
		NewArray(Int64Type, 0),
		ArrayType(Int64Type),
	},
	{
		[]string{"[", "]", BoolTypeName},
		NewArray(BoolType, 0),
		ArrayType(BoolType),
	},
	{
		[]string{"[", "]", Float64TypeName},
		NewArray(Float64Type, 0),
		ArrayType(Float64Type),
	},
	{
		[]string{"[", "]", Float32TypeName},
		NewArray(Float32Type, 0),
		ArrayType(Float32Type),
	},
	{
		[]string{"[", "]", StringTypeName},
		NewArray(StringType, 0),
		ArrayType(StringType),
	},
	{
		[]string{"[", "]", InterfaceTypeName},
		NewArray(InterfaceType, 0),
		ArrayType(InterfaceType),
	},
	{
		[]string{BoolTypeName},
		boolModel,
		BoolType,
	},
	{
		[]string{ByteTypeName},
		byteModel,
		ByteType,
	},
	{
		[]string{Int32TypeName},
		int32Model,
		Int32Type,
	},
	{
		[]string{IntTypeName},
		intModel,
		IntType,
	},
	{
		[]string{Int64TypeName},
		int64Model,
		Int64Type,
	},
	{
		[]string{Float64TypeName},
		float64Model,
		Float64Type,
	},
	{
		[]string{Float32TypeName},
		float32Model,
		Float32Type,
	},
	{
		[]string{StringTypeName},
		stringModel,
		StringType,
	},
	{
		[]string{InterfaceTypeName},
		interfaceModel,
		InterfaceType,
	},
	{
		[]string{"any"},
		interfaceModel,
		InterfaceType,
	},
	{
		[]string{"interface", "{}"},
		interfaceModel,
		InterfaceType,
	},
	{
		[]string{"*", BoolTypeName},
		&boolInterface,
		PointerType(BoolType),
	},
	{
		[]string{"*", Int32TypeName},
		&int32Interface,
		PointerType(Int32Type),
	},
	{
		[]string{"*", ByteTypeName},
		&byteInterface,
		PointerType(ByteType),
	},
	{
		[]string{"*", IntTypeName},
		&intInterface,
		PointerType(IntType),
	},
	{
		[]string{"*", Int64TypeName},
		&int64Interface,
		PointerType(Int64Type),
	},
	{
		[]string{"*", Float64TypeName},
		&float64Interface,
		PointerType(Float64Type),
	},
	{
		[]string{"*", Float32TypeName},
		&float32Interface,
		PointerType(Float32Type),
	},
	{
		[]string{"*", StringTypeName},
		&stringInterface,
		PointerType(StringType),
	},
	{
		[]string{"*", InterfaceTypeName},
		&interfaceModel,
		PointerType(InterfaceType),
	},
	{
		[]string{"*", "interface", "{}"},
		&interfaceModel,
		PointerType(InterfaceType),
	},
	{
		[]string{"type"},
		nil,
		TypeType,
	},
}
