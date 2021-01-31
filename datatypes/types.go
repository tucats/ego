package datatypes

// Define data types as abstract identifiers

const (
	UndefinedType = iota
	IntType
	FloatType
	StringType
	BoolType
	ArrayType
	StructType
	ErrorType
	ChanType
	MapType
	InterfaceType // alias for "any"
	VarArgs       // pseudo type used for varible argument list items
	UserType      // something defined by a type statement
)

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
