package datatypes

// This defines the token structure for various type declarations, including a model of that
// type and the type designation.
type TypeDefinition struct {
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
var TypeDeclarations = []TypeDefinition{
	{
		[]string{"sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		WaitGroupType,
	},
	{
		[]string{"*", "sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		PointerToType(WaitGroupType),
	},
	{
		[]string{"sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		MutexType,
	},
	{
		[]string{"*", "sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		PointerToType(MutexType),
	},
	{
		[]string{"chan"},
		chanModel,
		ChanType,
	},
	{
		[]string{"[", "]", "int"},
		NewArray(IntType, 0),
		ArrayOfType(IntType),
	},
	{
		[]string{"[", "]", "bool"},
		NewArray(BoolType, 0),
		ArrayOfType(BoolType),
	},
	{
		[]string{"[", "]", "float"},
		NewArray(FloatType, 0),
		ArrayOfType(FloatType),
	},
	{
		[]string{"[", "]", "string"},
		NewArray(StringType, 0),
		ArrayOfType(StringType),
	},
	{
		[]string{"[", "]", "interface{}"},
		NewArray(InterfaceType, 0),
		ArrayOfType(InterfaceType),
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
		PointerToType(BoolType),
	},
	{
		[]string{"*", "int"},
		&intInterface,
		PointerToType(IntType),
	},
	{
		[]string{"*", "float"},
		&floatInterface,
		PointerToType(FloatType),
	},
	{
		[]string{"*", "string"},
		&stringInterface,
		PointerToType(StringType),
	},
	{
		[]string{"*", "interface{}"},
		&interfaceModel,
		PointerToType(InterfaceType),
	},
}
