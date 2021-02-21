package datatypes

import "strings"

// Define data types as abstract identifiers.
const (
	UndefinedType = iota
	IntType
	FloatType
	StringType
	BoolType
	StructType
	ErrorType
	ChanType
	MapType
	InterfaceType        // alias for "any"
	VarArgs              // pseudo type used for varible argument list items
	UserType             // something defined by a type statement
	PointerType   = 2048 // Can be added to any type to make it an array
	ArrayType     = 4096 // Can be added to a type to make it an array
)

// This defines the token structure for various type declarations, including a model of that
// type and the type designation.
type TypeDefinition struct {
	Tokens []string
	Model  interface{}
	Kind   int
}

// This is the "zero instance" value for various types.
var interfaceModel interface{}
var intModel = 0
var floatModel = 0.0
var boolModel = false
var stringModel = ""

//var arrayModel = []interface{}{}
//var structModle = map[string]interface{}{}

// TypeDeclarationMap is a dictionary of all the type declaration token sequences.
var TypeDeclarationMap = []TypeDefinition{
	{
		[]string{"chan"},
		&Channel{},
		ChanType,
	},
	{
		[]string{"[", "]", "int"},
		NewArray(IntType, 0),
		IntType + ArrayType,
	},
	{
		[]string{"[", "]", "bool"},
		NewArray(BoolType, 0),
		BoolType + ArrayType,
	},
	{
		[]string{"[", "]", "float"},
		NewArray(FloatType, 0),
		FloatType + ArrayType,
	},
	{
		[]string{"[", "]", "string"},
		NewArray(StringType, 0),
		StringType + ArrayType,
	},
	{
		[]string{"[", "]", "interface{}"},
		NewArray(InterfaceType, 0),
		InterfaceType + ArrayType,
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
		&boolModel,
		BoolType + PointerType,
	},
	{
		[]string{"*", "int"},
		&intModel,
		IntType + PointerType,
	},
	{
		[]string{"*", "float"},
		&floatModel,
		FloatType + PointerType,
	},
	{
		[]string{"*", "string"},
		&stringModel,
		StringType + PointerType,
	},
	{
		[]string{"*", "interface{}"},
		&interfaceModel,
		InterfaceType + PointerType,
	},
}

// TypeOF accepts an interface of arbitrary Ego or native data type,
// and returns an integer containing the datatype specification, such
// as datatypes.IntType or datatypes.StringType.
func TypeOf(i interface{}) int {
	switch actual := i.(type) {
	case int:
		return IntType

	case float32, float64:
		return FloatType

	case string:
		return StringType

	case bool:
		return BoolType

	case map[string]interface{}:
		return StructType

	case *int:
		return IntType + PointerType

	case *float32, *float64:
		return FloatType + PointerType

	case *string:
		return StringType + PointerType

	case *bool:
		return BoolType + PointerType

	case *map[string]interface{}:
		return StructType + PointerType

	case *EgoMap:
		return MapType

	case *Channel:
		return ChanType

	default:
		switch actual.(type) {
		case int:
			return IntType
		case *int:
			return IntType + PointerType
		}

		return InterfaceType
	}
}

// TypeString returns a textual representation of the type indicator
// passed in.
func TypeString(kind int) string {
	r := "interface{}"

	for _, t := range TypeDeclarationMap {
		if kind == t.Kind {
			r = strings.Join(t.Tokens, "")
		}
	}

	return r
}

// InstanceOf accepts a kind type indicator, and returns the zero-value
// model of that type.
func InstanceOf(kind int) interface{} {
	for _, typeDef := range TypeDeclarationMap {
		if typeDef.Kind == kind {
			return typeDef.Model
		}
	}

	return nil
}

// IsType accepts an arbitrary value that is either an Ego or native data
// value, and a type specification, and indicates if it is of the provided
// Ego datatype indicator.
func IsType(v interface{}, kind int) bool {
	if kind == InterfaceType {
		return true
	}

	switch v.(type) {
	case *int, *int32, *int64:
		return kind == IntType+PointerType

	case *float32, *float64:
		return kind == FloatType+PointerType

	case *string:
		return kind == StringType+PointerType

	case *bool:
		return kind == BoolType+PointerType

	case *[]interface{}:
		return kind == ArrayType+PointerType

	case *map[string]interface{}:
		if _, ok := GetMetadata(v, TypeMDKey); ok {
			return kind == UserType
		}

		return kind == StructType+PointerType

	case int, int32, int64:
		return kind == IntType

	case float32, float64:
		return kind == FloatType

	case string:
		return kind == StringType

	case bool:
		return kind == BoolType

	case []interface{}:
		return kind == ArrayType

	case map[string]interface{}:
		if _, ok := GetMetadata(v, TypeMDKey); ok {
			return kind == UserType
		}

		return kind == StructType

	case EgoMap:
		return kind == MapType

	case error:
		return kind == ErrorType
	}

	return false
}
