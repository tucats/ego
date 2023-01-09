package data

// This defines the token structure for various type declarations, including a model of that
// type and the type designation.
type TypeDeclaration struct {
	Tokens []string
	Model  interface{}
	Kind   *Type
}

// This is the "zero instance" value for various types.
var interfaceModel interface{}
var byteModel byte = 0
var int32Model int32 = 0
var intModel int = 0
var int64Model int64 = 0
var float64Model float64 = 0.0
var float32Model float32 = 0.0
var boolModel = false
var stringModel = ""
var chanModel = NewChannel(1)

var byteInterface interface{} = byte(0)
var int32Interface interface{} = int32(0)
var intInterface interface{} = int(0)
var int64Interface interface{} = int64(0)
var boolInterface interface{} = false
var float64Interface interface{} = 0.0
var float32Interface interface{} = float32(0.0)
var stringInterface interface{} = ""

// TypeDeclarations is a dictionary of all the type declaration token sequences.
// This includes _Ego_ types and also native types, such as sync.WaitGroup.  Note
// that for native types, you may also have to update InstanceOf() to generate a
// unique instance of the required type, usually via pointer so the native function
// can reference/update the native value.
var TypeDeclarations = []TypeDeclaration{
	{
		[]string{"sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		&WaitGroupType,
	},
	{
		[]string{"*", "sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		Pointer(&WaitGroupType),
	},
	{
		[]string{"sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		&MutexType,
	},
	{
		[]string{"*", "sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		Pointer(&MutexType),
	},
	{
		[]string{"chan"},
		chanModel,
		&ChanType,
	},
	{
		[]string{"[", "]", ByteTypeName},
		NewArray(&ByteType, 0),
		Array(&ByteType),
	},
	{
		[]string{"[", "]", Int32TypeName},
		NewArray(&Int32Type, 0),
		Array(&Int32Type),
	},
	{
		[]string{"[", "]", IntTypeName},
		NewArray(&IntType, 0),
		Array(&IntType),
	},
	{
		[]string{"[", "]", Int64TypeName},
		NewArray(&Int64Type, 0),
		Array(&Int64Type),
	},
	{
		[]string{"[", "]", BoolTypeName},
		NewArray(BoolType, 0),
		Array(BoolType),
	},
	{
		[]string{"[", "]", Float64TypeName},
		NewArray(&Float64Type, 0),
		Array(&Float64Type),
	},
	{
		[]string{"[", "]", Float32TypeName},
		NewArray(&Float32Type, 0),
		Array(&Float32Type),
	},
	{
		[]string{"[", "]", StringTypeName},
		NewArray(&StringType, 0),
		Array(&StringType),
	},
	{
		[]string{"[", "]", InterfaceTypeName},
		NewArray(&InterfaceType, 0),
		Array(&InterfaceType),
	},
	{
		[]string{BoolTypeName},
		boolModel,
		BoolType,
	},
	{
		[]string{ByteTypeName},
		byteModel,
		&ByteType,
	},
	{
		[]string{Int32TypeName},
		int32Model,
		&Int32Type,
	},
	{
		[]string{IntTypeName},
		intModel,
		&IntType,
	},
	{
		[]string{Int64TypeName},
		int64Model,
		&Int64Type,
	},
	{
		[]string{Float64TypeName},
		float64Model,
		&Float64Type,
	},
	{
		[]string{Float32TypeName},
		float32Model,
		&Float32Type,
	},
	{
		[]string{StringTypeName},
		stringModel,
		&StringType,
	},
	{
		[]string{InterfaceTypeName},
		interfaceModel,
		&InterfaceType,
	},
	{
		[]string{"interface", "{}"},
		interfaceModel,
		&InterfaceType,
	},
	{
		[]string{"*", BoolTypeName},
		&boolInterface,
		Pointer(BoolType),
	},
	{
		[]string{"*", Int32TypeName},
		&int32Interface,
		Pointer(&Int32Type),
	},
	{
		[]string{"*", ByteTypeName},
		&byteInterface,
		Pointer(&ByteType),
	},
	{
		[]string{"*", IntTypeName},
		&intInterface,
		Pointer(&IntType),
	},
	{
		[]string{"*", Int64TypeName},
		&int64Interface,
		Pointer(&Int64Type),
	},
	{
		[]string{"*", Float64TypeName},
		&float64Interface,
		Pointer(&Float64Type),
	},
	{
		[]string{"*", Float32TypeName},
		&float32Interface,
		Pointer(&Float32Type),
	},
	{
		[]string{"*", StringTypeName},
		&stringInterface,
		Pointer(&StringType),
	},
	{
		[]string{"*", InterfaceTypeName},
		&interfaceModel,
		Pointer(&InterfaceType),
	},
	{
		[]string{"*", "interface", "{}"},
		&interfaceModel,
		Pointer(&InterfaceType),
	},
}
