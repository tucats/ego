package datatypes

import (
	"sync"

	"github.com/tucats/ego/errors"
)

// Define data types as abstract identifiers.
const (
	UndefinedKind = iota
	intKind
	floatKind
	stringKind
	boolKind
	structKind
	errorKind
	chanKind
	mapKind
	interfaceKind // alias for "any"
	pointerKind   // Pointer to some type
	arrayKind     // Array of some type

	minimumNativeType // Before list of Go-native types mapped to Ego types
	waitGroupKind
	mutexKind
	maximumNativeType // After list of Go-native types

	VarArgs  // pseudo type used for variable argument list items
	UserType // something defined by a type statement
)

type Type struct {
	Name      string
	Kind      int
	KeyType   *Type
	ValueType *Type
}

// Type definitions for each given type.
var UndefinedTypeDef = Type{
	Name: "undefined",
	Kind: UndefinedKind,
}

var InterfaceTypeDef = Type{
	Name:      "interface{}",
	Kind:      interfaceKind,
	KeyType:   nil,
	ValueType: nil,
}

var InterfacePtrTypeDef = Type{
	Name:      "*interface{}",
	Kind:      pointerKind,
	KeyType:   nil,
	ValueType: &InterfaceTypeDef,
}

var InterfaceArrayTypeDef = Type{
	Name:      "[]interface{}",
	Kind:      arrayKind,
	KeyType:   nil,
	ValueType: &InterfaceTypeDef,
}

var ErrorTypeDef = Type{
	Name: "error",
	Kind: errorKind,
}

var BoolTypeDef = Type{
	Name:      "bool",
	Kind:      boolKind,
	KeyType:   nil,
	ValueType: nil,
}

var BoolPtrTypeDef = Type{
	Name:      "bool",
	Kind:      boolKind,
	KeyType:   nil,
	ValueType: &BoolTypeDef,
}

var IntTypeDef = Type{
	Name:      "int",
	Kind:      intKind,
	KeyType:   nil,
	ValueType: nil,
}

var IntPtrTypeDef = Type{
	Name:      "*int",
	Kind:      pointerKind,
	KeyType:   nil,
	ValueType: &IntTypeDef,
}

var FloatTypeDef = Type{
	Name:      "float",
	Kind:      floatKind,
	KeyType:   nil,
	ValueType: nil,
}

var FloatPtrTypeDef = Type{
	Name:      "*float",
	Kind:      pointerKind,
	KeyType:   nil,
	ValueType: &FloatTypeDef,
}

var StringTypeDef = Type{
	Name:      "string",
	Kind:      stringKind,
	KeyType:   nil,
	ValueType: nil,
}

var StringPtrTypeDef = Type{
	Name:      "*string",
	Kind:      pointerKind,
	KeyType:   nil,
	ValueType: &StringTypeDef,
}

var ChanTypeDef = Type{
	Name:      "chan",
	Kind:      chanKind,
	KeyType:   nil,
	ValueType: nil,
}

var ChanPtrTypeDef = Type{
	Name:      "*chan",
	Kind:      pointerKind,
	KeyType:   nil,
	ValueType: &ChanTypeDef,
}

var WGTypeDef = Type{
	Name:      "WaitGroup",
	Kind:      waitGroupKind,
	KeyType:   nil,
	ValueType: nil,
}

var WGPtrTypeDef = Type{
	Name:      "*WaitGroup",
	Kind:      pointerKind,
	KeyType:   nil,
	ValueType: &WGTypeDef,
}

var MutexTypeDef = Type{
	Name:      "Mutex",
	Kind:      mutexKind,
	KeyType:   nil,
	ValueType: nil,
}

var MutexPtrTypeDef = Type{
	Name:      "*Mutex",
	Kind:      pointerKind,
	KeyType:   nil,
	ValueType: &MutexTypeDef,
}

var VarArgsTypeDef = Type{
	Name: "...",
	Kind: VarArgs,
}

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

// TypeDeclarationMap is a dictionary of all the type declaration token sequences.
// This includes _Ego_ types and also native types, such as sync.WaitGroup.  Note
// that for native types, you may also have to update InstanceOf() to generate a
// unique instance of the required type, usually via pointer so the native function
// can reference/update the native value.
var TypeDeclarationMap = []TypeDefinition{
	{
		[]string{"sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		WGTypeDef,
	},
	{
		[]string{"*", "sync", ".", "WaitGroup"},
		nil, // Model generated in instance-of
		WGPtrTypeDef,
	},
	{
		[]string{"sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		MutexTypeDef,
	},
	{
		[]string{"*", "sync", ".", "Mutex"},
		nil, // Model generated in instance-of
		MutexPtrTypeDef,
	},
	{
		[]string{"chan"},
		chanModel,
		ChanTypeDef,
	},
	{
		[]string{"[", "]", "int"},
		NewArray(IntTypeDef, 0),
		Type{
			Name:      "[]int",
			Kind:      arrayKind,
			ValueType: &IntTypeDef,
		},
	},
	{
		[]string{"[", "]", "bool"},
		NewArray(BoolTypeDef, 0),
		Type{
			Name:      "[]bool",
			Kind:      arrayKind,
			ValueType: &BoolTypeDef,
		},
	},
	{
		[]string{"[", "]", "float"},
		NewArray(FloatTypeDef, 0),
		Type{
			Name:      "[]float",
			Kind:      arrayKind,
			ValueType: &FloatTypeDef,
		},
	},
	{
		[]string{"[", "]", "string"},
		NewArray(StringTypeDef, 0),
		Type{
			Name:      "[]string",
			Kind:      arrayKind,
			ValueType: &StringTypeDef,
		},
	},
	{
		[]string{"[", "]", "interface{}"},
		NewArray(InterfaceTypeDef, 0),
		Type{
			Name:      "[]interface{}",
			Kind:      arrayKind,
			ValueType: &InterfaceTypeDef,
		},
	},
	{
		[]string{"bool"},
		boolModel,
		BoolTypeDef,
	},
	{
		[]string{"int"},
		intModel,
		IntTypeDef,
	},
	{
		[]string{"float"},
		floatModel,
		FloatTypeDef,
	},
	{
		[]string{"string"},
		stringModel,
		StringTypeDef,
	},
	{
		[]string{"interface{}"},
		interfaceModel,
		InterfaceTypeDef,
	},
	{
		[]string{"*", "bool"},
		&boolInterface,
		BoolPtrTypeDef,
	},
	{
		[]string{"*", "int"},
		&intInterface,
		IntPtrTypeDef,
	},
	{
		[]string{"*", "float"},
		&floatInterface,
		FloatPtrTypeDef,
	},
	{
		[]string{"*", "string"},
		&stringInterface,
		StringPtrTypeDef,
	},
	{
		[]string{"*", "interface{}"},
		&interfaceModel,
		InterfacePtrTypeDef,
	},
}

// TypeOF accepts an interface of arbitrary Ego or native data type,
// and returns an integer containing the datatype specification, such
// as datatypes.intKind or datatypes.stringKind.
func TypeOf(i interface{}) Type {
	switch v := i.(type) {
	case *interface{}:
		return InterfacePtrTypeDef

	case *sync.WaitGroup:
		return WGTypeDef

	case **sync.WaitGroup:
		return WGPtrTypeDef

	case *sync.Mutex:
		return MutexTypeDef

	case **sync.Mutex:
		return MutexPtrTypeDef

	case int:
		return IntTypeDef

	case float32, float64:
		return FloatTypeDef

	case string:
		return StringTypeDef

	case bool:
		return BoolTypeDef

	case map[string]interface{}:
		return Type{
			Name: "struct",
			Kind: structKind,
		}

	case *int:
		return IntPtrTypeDef

	case *float32, *float64:
		return FloatPtrTypeDef

	case *string:
		return StringPtrTypeDef

	case *bool:
		return BoolPtrTypeDef

	case *map[string]interface{}:
		return Type{
			Name: "*struct",
			Kind: pointerKind,
			ValueType: &Type{
				Name: "struct",
				Kind: structKind,
			},
		}

	case *EgoMap:
		return v.Type()

	case *Channel:
		return ChanPtrTypeDef

	default:
		return InterfaceTypeDef
	}
}

func TypeString(i interface{}) string {
	if i == nil {
		return "nil"
	}

	if k, ok := i.(int); ok {
		for _, v := range TypeDeclarationMap {
			if v.Kind.Kind == k {
				return v.Kind.Name
			}
		}
	}

	if t, ok := i.(Type); ok {
		switch t.Kind {
		case mapKind:
			return "map[" + TypeString(*t.KeyType) + "]" + TypeString(*t.ValueType)

		case pointerKind:
			return "*" + TypeString(*t.ValueType)

		case arrayKind:
			return "[]" + TypeString(*t.ValueType)

		default:
			return t.Name
		}
	}

	return "unknown"
}

// InstanceOf accepts a kind type indicator, and returns the zero-value
// model of that type. This uses either the model value found in the
// types dictionary, or for some special native objects (like a sync.WaitGroup)
// code here creates a new instance of that type and returns it's address.
func InstanceOf(kind Type) interface{} {
	// Waitgroups must be uniquely created.
	switch kind.Kind {
	case mutexKind:
		return &sync.Mutex{}

	case waitGroupKind:
		return &sync.WaitGroup{}

	case pointerKind:
		switch kind.ValueType.Kind {
		case mutexKind:
			mt := &sync.Mutex{}

			return &mt

		case waitGroupKind:
			wg := &sync.WaitGroup{}

			return &wg
		}

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
func IsType(v interface{}, kind Type) bool {
	if kind.Kind == interfaceKind {
		return true
	}

	switch v.(type) {
	case *sync.Mutex:
		return kind.IsKind(mutexKind)

	case **sync.Mutex:
		return kind.IsPointerToType(mutexKind)

	case *sync.WaitGroup:
		return kind.IsKind(waitGroupKind)

	case **sync.WaitGroup:
		return kind.IsPointerToType(waitGroupKind)

	case *int, *int32, *int64:
		return kind.IsPointerToType(intKind)

	case *float32, *float64:
		return kind.IsPointerToType(floatKind)

	case *string:
		return kind.IsPointerToType(stringKind)

	case *bool:
		return kind.IsPointerToType(boolKind)

	case *[]interface{}:
		return kind.Kind == pointerKind &&
			kind.ValueType != nil &&
			kind.ValueType.Kind == arrayKind &&
			kind.ValueType.ValueType != nil &&
			kind.ValueType.ValueType.Kind == interfaceKind

	case *map[string]interface{}:
		if typeName, ok := GetMetadata(v, TypeMDKey); ok {
			return typeName == kind.Name
		}

		return kind.Kind == pointerKind &&
			kind.ValueType != nil &&
			kind.ValueType.Kind == structKind

	case int, int32, int64:
		return kind.IsKind(intKind)

	case float32, float64:
		return kind.IsKind(floatKind)

	case string:
		return kind.IsKind(stringKind)

	case bool:
		return kind.IsKind(boolKind)

	case []interface{}:
		return kind.Kind == arrayKind &&
			kind.ValueType != nil &&
			kind.ValueType.Kind == interfaceKind

	case map[string]interface{}:
		if typeName, ok := GetMetadata(v, TypeMDKey); ok {
			return typeName == kind.Name
		}

		return kind.IsKind(structKind)

	case EgoMap:
		return kind.IsKind(mapKind)

	case error:
		return kind.IsKind(errorKind)
	}

	return false
}

// For a given type, construct a type that is an array of it.
func ArrayOfType(t Type) Type {
	return Type{
		Name:      "[]",
		Kind:      arrayKind,
		ValueType: &t,
	}
}

// For a given type, construct a type that is an pointer to a value of that type.
func PointerToType(t Type) Type {
	return Type{
		Name:      "[]",
		Kind:      pointerKind,
		ValueType: &t,
	}
}

// For a given interface pointer, unwrap the pointer and return the type it
// actually points to
func TypeOfPointer(v interface{}) Type {
	if p, ok := v.(Type); ok {
		if p.Kind != pointerKind || p.ValueType == nil {
			return UndefinedTypeDef
		}

		return *p.ValueType
	}

	// Is this a pointer to an actual native interface?
	p, ok := v.(*interface{})
	if !ok {
		return UndefinedTypeDef
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
		if item.Kind.Kind == kind {
			// If this is a pointer type, skip the pointer token
			if item.Tokens[0] == "*" {
				return item.Tokens[1]
			}

			return item.Tokens[0]
		}
	}

	return ""
}

func (t Type) IsType(i Type) bool {
	if t.Kind != i.Kind {
		return false
	}

	if t.KeyType != nil && i.KeyType == nil {
		return false
	}

	if t.ValueType != nil && i.ValueType == nil {
		return false
	}

	if t.KeyType != nil && i.KeyType != nil {
		if !t.KeyType.IsType(*i.KeyType) {
			return false
		}
	}

	if t.ValueType != nil && i.ValueType != nil {
		if !t.ValueType.IsType(*i.ValueType) {
			return false
		}
	}

	return true
}

func (t Type) IsKind(baseType int) bool {
	return t.Kind == baseType
}

func (t Type) IsPointerToType(tt int) bool {
	if t.Kind != pointerKind || t.ValueType == nil {
		return false
	}

	return t.ValueType.Kind == tt
}

func (t Type) IsArray() bool {
	return t.Kind == arrayKind
}
