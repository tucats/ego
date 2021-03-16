package datatypes

import (
	"strings"
	"sync"

	"github.com/tucats/ego/errors"
)

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
	minimumNativeType
	WaitGroupType
	maximumNativeType
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
var chanModel = NewChannel(1)

var intInterface interface{} = 0
var boolInterface interface{} = false
var floatInterface interface{} = 0.0
var stringInterface interface{} = ""

//var arrayModel = []interface{}{}
//var structModle = map[string]interface{}{}

// TypeDeclarationMap is a dictionary of all the type declaration token sequences.
// This includes _Ego_ types and also native types, such as sync.WaitGroup.  Note
// that for native types, you may also have to update InstanceOf() to generate a
// unique instance of the required type, usually via pointer so the native function
// can reference/update the native value.
var TypeDeclarationMap = []TypeDefinition{
	{
		[]string{"sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		WaitGroupType,
	},
	{
		[]string{"*", "sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		WaitGroupType + PointerType,
	},
	{
		[]string{"chan"},
		chanModel,
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
		&boolInterface,
		BoolType + PointerType,
	},
	{
		[]string{"*", "int"},
		&intInterface,
		IntType + PointerType,
	},
	{
		[]string{"*", "float"},
		&floatInterface,
		FloatType + PointerType,
	},
	{
		[]string{"*", "string"},
		&stringInterface,
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
	switch v := i.(type) {
	case *interface{}:
		return TypeOf(*v) + PointerType

	case *sync.WaitGroup:
		return WaitGroupType

	case **sync.WaitGroup:
		return WaitGroupType + PointerType

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
// model of that type. This uses either the model value found in the
// types dictionary, or for some special native objects (like a sync.WaitGroup)
// code here creates a new instance of that type and returns it's address.
func InstanceOf(kind int) interface{} {
	// Waitgroups must be uniquely created.
	switch kind {
	case WaitGroupType:
		return &sync.WaitGroup{}

	case WaitGroupType + PointerType:
		wg := &sync.WaitGroup{}

		return &wg

	default:
		for _, typeDef := range TypeDeclarationMap {
			if typeDef.Kind == kind {
				return typeDef.Model
			}
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
	case *sync.WaitGroup:
		return kind == WaitGroupType

	case **sync.WaitGroup:
		return kind == WaitGroupType+PointerType

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

func PointerTo(v interface{}) int {
	// Is this a pointer to an interface at all?
	p, ok := v.(*interface{})
	if !ok {
		return UndefinedType
	}

	actual := *p

	return TypeOf(actual)
}

func IsNil(v interface{}) bool {
	// Is it outright a nil value?
	if v == nil {
		return true
	}

	// Is it a nil error message?
	if err, ok := v.(*errors.EgoError); ok {
		return errors.Nil(err)
	}

	// If it's not a pointer, then it can't be nil
	addr, ok := v.(*interface{})
	if !ok {
		return false
	}

	// Compare the pointer to the known "Zero values"
	// used to initialize empty pointers.
	if addr == nil {
		return true
	} else if addr == &boolInterface {
		return true
	} else if addr == &intInterface {
		return true
	} else if addr == &stringInterface {
		return true
	} else if addr == &floatInterface {
		return true
	} else if addr == &interfaceModel {
		return true
	}

	return false
}

// Is this type associated with a native Ego type that has
// extended native function support?
func IsNative(kind int) bool {
	return kind > minimumNativeType && kind < maximumNativeType
}

// For a given type, return the native package that contains
// it. For example, sync.WaitGroup would return "sync".
func NativePackage(kind int) string {
	for _, item := range TypeDeclarationMap {
		if item.Kind == kind {
			// If this is a pointer type, skip the pointer token
			if item.Tokens[0] == "*" {
				return item.Tokens[1]
			}

			return item.Tokens[0]
		}
	}

	return ""
}
