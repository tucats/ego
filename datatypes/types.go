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
	InterfaceType       // alias for "any"
	VarArgs             // pseudo type used for varible argument list items
	UserType            // something defined by a type statement
	ArrayType     = 100 // Can be added to a type to make it an array
)

// This defines the token structure for various type declarations, including a model of that
// type and the type designation.
type TypeDefinition struct {
	Tokens []string
	Model  interface{}
	Kind   int
}

// TypeDeclarationMap is a dictionary of all the type declaration token sequences.
var TypeDeclarationMap = []TypeDefinition{
	{
		[]string{"chan"},
		&Channel{},
		ChanType,
	},
	{
		[]string{"[", "]", "int"},
		1,
		IntType + ArrayType,
	},
	{
		[]string{"[", "]", "bool"},
		true,
		BoolType + ArrayType,
	},
	{
		[]string{"[", "]", "float"},
		0.0,
		FloatType + ArrayType,
	},
	{
		[]string{"[", "]", "string"},
		"",
		StringType + ArrayType,
	},
	{
		[]string{"[", "]", "interface{}"},
		nil,
		InterfaceType + ArrayType,
	},
	{
		[]string{"[", "]", "struct"},
		map[string]interface{}{},
		ArrayType,
	},
	{
		[]string{"[", "]", "{", "}"},
		map[string]interface{}{},
		StructType,
	},
	{
		[]string{"bool"},
		true,
		BoolType,
	},
	{
		[]string{"int"},
		0,
		IntType,
	},
	{
		[]string{"float"},
		0.0,
		FloatType,
	},
	{
		[]string{"string"},
		"",
		StringType,
	},
	{
		[]string{"interface{}"},
		nil,
		InterfaceType,
	},
}

func TypeOf(i interface{}) int {
	switch i.(type) {
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

	case *EgoMap:
		return MapType

	case *Channel:
		return ChanType
	default:
		return InterfaceType
	}
}

func TypeString(kind int) string {
	r := "interface{}"

	for _, t := range TypeDeclarationMap {
		if kind == t.Kind {
			r = strings.Join(t.Tokens, "")
		}
	}

	return r
}

func IsType(v interface{}, kind int) bool {
	if kind == InterfaceType {
		return true
	}

	switch v.(type) {
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
