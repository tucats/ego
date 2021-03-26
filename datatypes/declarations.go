package datatypes

// This defines the token structure for various type declarations, including a model of that
// type and the type designation.
type TypeDeclaration struct {
	Tokens []string
	Model  interface{}
	Kind   Type
}

// This is the "zero instance" value for various types.
var interfaceModel interface{}
var intModel = 0
var floatModel = 0.0
var boolModel = false
var stringModel = ""
var chanModel = NewChannel(1)

var intInterface interface{} = 0
var boolInterface interface{} = false
var floatInterface interface{} = 0.0
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
		WaitGroupType,
	},
	{
		[]string{"*", "sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		Pointer(WaitGroupType),
	},
	{
		[]string{"sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		MutexType,
	},
	{
		[]string{"*", "sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		Pointer(MutexType),
	},
	{
		[]string{"chan"},
		chanModel,
		ChanType,
	},
	{
		[]string{"[", "]", "int"},
		NewArray(IntType, 0),
		Array(IntType),
	},
	{
		[]string{"[", "]", "bool"},
		NewArray(BoolType, 0),
		Array(BoolType),
	},
	{
		[]string{"[", "]", "float"},
		NewArray(FloatType, 0),
		Array(FloatType),
	},
	{
		[]string{"[", "]", "string"},
		NewArray(StringType, 0),
		Array(StringType),
	},
	{
		[]string{"[", "]", "interface{}"},
		NewArray(InterfaceType, 0),
		Array(InterfaceType),
	},
	{
		[]string{"bool"},
		boolModel,
		BoolType,
	},
	{
		[]string{"int"},
		intModel,
		IntType,
	},
	{
		[]string{"float"},
		floatModel,
		FloatType,
	},
	{
		[]string{"string"},
		stringModel,
		StringType,
	},
	{
		[]string{"interface{}"},
		interfaceModel,
		InterfaceType,
	},
	{
		[]string{"*", "bool"},
		&boolInterface,
		Pointer(BoolType),
	},
	{
		[]string{"*", "int"},
		&intInterface,
		Pointer(IntType),
	},
	{
		[]string{"*", "float"},
		&floatInterface,
		Pointer(FloatType),
	},
	{
		[]string{"*", "string"},
		&stringInterface,
		Pointer(StringType),
	},
	{
		[]string{"*", "interface{}"},
		&interfaceModel,
		Pointer(InterfaceType),
	},
}
