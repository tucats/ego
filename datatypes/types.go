package datatypes

import (
	"sync"

	"github.com/tucats/ego/errors"
)

// Define data types as abstract identifiers. These are the base
// types for all other types. For example, a pointer to an integer
// in constructed from a pointerKind type that references an IntKind
// type.
const (
	undefinedKind = iota
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
	packageKind   // A package

	minimumNativeType // Before list of Go-native types mapped to Ego types
	waitGroupKind
	mutexKind
	maximumNativeType // After list of Go-native types

	varArgs  // pseudo type used for variable argument list items
	userKind // something defined by a type statement
)

type Type struct {
	Name      string
	Kind      int
	KeyType   *Type
	ValueType *Type
}

// Type definitions for each given type.
var UndefinedType = Type{
	Name: "undefined",
	Kind: undefinedKind,
}

var PackageType = Type{
	Name: "package",
	Kind: packageKind,
}

var StructType = Type{
	Name: "struct",
	Kind: structKind,
}

var InterfaceType = Type{
	Name:      "interface{}",
	Kind:      interfaceKind,
	KeyType:   nil,
	ValueType: nil,
}

var ErrorType = Type{
	Name: "error",
	Kind: errorKind,
}

var BoolType = Type{
	Name:      "bool",
	Kind:      boolKind,
	KeyType:   nil,
	ValueType: nil,
}

var IntType = Type{
	Name:      "int",
	Kind:      intKind,
	KeyType:   nil,
	ValueType: nil,
}

var FloatType = Type{
	Name:      "float",
	Kind:      floatKind,
	KeyType:   nil,
	ValueType: nil,
}

var StringType = Type{
	Name:      "string",
	Kind:      stringKind,
	KeyType:   nil,
	ValueType: nil,
}

var ChanType = Type{
	Name:      "chan",
	Kind:      chanKind,
	KeyType:   nil,
	ValueType: nil,
}

var WaitGroupType = Type{
	Name:      "WaitGroup",
	Kind:      waitGroupKind,
	KeyType:   nil,
	ValueType: nil,
}

var MutexType = Type{
	Name:      "Mutex",
	Kind:      mutexKind,
	KeyType:   nil,
	ValueType: nil,
}

var VarArgsType = Type{
	Name: "...",
	Kind: varArgs,
}

// For a given struct type, set it's type value in the metadata. If the
// item is not a struct map then do no work.
func SetType(m map[string]interface{}, t Type) {
	SetMetadata(m, TypeMDKey, t)
}

// TypeOf accepts an interface of arbitrary Ego or native data type,
// and returns an integer containing the datatype specification, such
// as datatypes.intKind or datatypes.stringKind.
func TypeOf(i interface{}) Type {
	switch v := i.(type) {
	case *interface{}:
		return PointerToType(InterfaceType)

	case *sync.WaitGroup:
		return WaitGroupType

	case **sync.WaitGroup:
		return PointerToType(WaitGroupType)

	case *sync.Mutex:
		return MutexType

	case **sync.Mutex:
		return PointerToType(MutexType)

	case int:
		return IntType

	case float32, float64:
		return FloatType

	case string:
		return StringType

	case bool:
		return BoolType

	case map[string]interface{}:
		// Is it a struct with an embedded type metadata item?
		if t, ok := GetMetadata(v, TypeMDKey); ok {
			if t, ok := t.(Type); ok {
				return t
			}
		}

		// Nope, apparently just an anonymous struct
		return Type{
			Name: "struct",
			Kind: structKind,
		}

	case *int:
		return PointerToType(IntType)

	case *float32, *float64:
		return PointerToType(FloatType)

	case *string:
		return PointerToType(StringType)

	case *bool:
		return PointerToType(BoolType)

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
		return PointerToType(ChanType)

	default:
		return InterfaceType
	}
}

func (t Type) String() string {
	switch t.Kind {
	case userKind:
		return t.Name

	case mapKind:
		return "map[" + t.KeyType.String() + "]" + t.ValueType.String()

	case pointerKind:
		return "*" + t.ValueType.String()

	case arrayKind:
		return "[]" + t.ValueType.String()

	default:
		return t.Name
	}
}

// InstanceOf accepts a kind type indicator, and returns the zero-value
// model of that type. This uses either the model value found in the
// types dictionary, or for some special native objects (like a sync.WaitGroup)
// code here creates a new instance of that type and returns it's address.
func InstanceOf(kind Type) interface{} {
	// Waitgroups and mutexes (and pointers to them) must be uniquely created
	// to satisfy Go requirements for unique instances for any value.
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
		for _, typeDef := range TypeDeclarations {
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

	switch actual := v.(type) {
	case *EgoMap:
		return kind.IsType(Type{
			Kind:      mapKind,
			KeyType:   &actual.keyType,
			ValueType: &actual.valueType,
		})

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

func MapOfType(key, value Type) Type {
	return Type{
		Name:      "map",
		Kind:      mapKind,
		KeyType:   &key,
		ValueType: &value,
	}
}

// For a given interface pointer, unwrap the pointer and return the type it
// actually points to.
func TypeOfPointer(v interface{}) Type {
	if p, ok := v.(Type); ok {
		if p.Kind != pointerKind || p.ValueType == nil {
			return UndefinedType
		}

		return *p.ValueType
	}

	// Is this a pointer to an actual native interface?
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
	for _, item := range TypeDeclarations {
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

func UserType(name string, base Type) Type {
	return Type{
		Name:      name,
		Kind:      userKind,
		ValueType: &base,
	}
}

func Package(name string) Type {
	return Type{
		Name:      name,
		Kind:      packageKind,
		ValueType: &StructType,
	}
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

func (t Type) IsUserType() bool {
	return t.Kind == userKind
}
