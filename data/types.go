package data

import (
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// Define data kinds as abstract identifiers. These are the base
// types for all other types. For example, a pointer to an integer
// in constructed from a PointerKind type that references an IntKind
// type.
//
// Note that these are integer values for rapid comparison, etc. The
// kind of a type is just one of it's attributes (others include
// dependent type information, names, and other metadata).
//
// IMPORTANT: the order of these must be from less-precise to most-precise
// for numeric values, as this ordering is used to normalize two values of
// different types before performing math on them.
const (
	// The "not-a-type" kind.
	UndefinedKind = iota

	// Boolean kind.
	BoolKind

	// Byte (8-bit integer) kind.
	ByteKind

	// Int32 (32-bit integer) kind.
	Int32Kind

	// Int (native integer) kind.
	IntKind

	// Int64 (64-bit integer) kind.
	Int64Kind

	// Float32 (32-bit floatting point) kind.
	Float32Kind

	// Float64 (64-bit floatting point) kind.
	Float64Kind

	// Unicode string kind.
	StringKind

	// Struct kind. A struct has an _Ego_ implementation
	// that includes the fields and their values as well as
	// additional typing metata.
	StructKind

	// Error kind. This holds an _Ego_ error value (errors.Error).
	ErrorKind

	// Channel kind. This holds an _Ego_ channel, which is a Go
	// channel with additioanl state information.
	ChanKind

	// Map kind. An _Ego_ map functions like a standard Go map
	// with the addition of being inherently thread-safe.  The
	// map type includes metadata about the type of the key
	// and value objects in the map.
	MapKind

	// Interface kind. This is used as a placeholder for an untyped
	// value.
	InterfaceKind // alias for defs.Any

	// Pointer kind. This includes metadata for what data type pointer
	// actually points to.
	PointerKind // Pointer to some type

	// Array kind. An _Ego_ array is an array of interface values, along
	// with metadata on the expected concrete type of those values and other
	// metadata.
	ArrayKind

	// Package kind. A package describes everything that is known about a
	// package, including name, exported constants and types, and function
	// and receiver function definitions.
	PackageKind

	minimumNativeType // Before list of Go-native types mapped to Ego types

	// WaitGroup kind. This is a type that describes a Go native WaitGroup.
	// Special method call operations can be done that pass control to helper
	// functions that call the native method on the actual object value.
	WaitGroupKind

	// Mutex kind. This is a type that describes a Go native Mutex.
	// Special method call operations can be done that pass control to helper
	// functions that call the native method on the actual object value.
	MutexKind
	maximumNativeType // After list of Go-native types

	// Type kind. This is the type that wrappers any defined Type value. It
	// has an underlying base type (struct, int, etc) as well as metadata about
	// the type (including what package it is owned by, etc).
	TypeKind // something defined by a type statement

	// Function kind. This type contains a function declaration, which is all
	// the metadata about the function.
	FunctionKind

	// Variable Arguments kind. This is used as a wrapper for argument lists that
	// are variadic.
	VarArgsKind
)

// These constants are used to map a type name to a string. This creates a single place
// where the "common" name for built-in types is found.
const (
	InterfaceTypeName = "interface{}"
	BoolTypeName      = "bool"
	ByteTypeName      = "byte"
	IntTypeName       = "int"
	Int32TypeName     = "int32"
	Int64TypeName     = "int64"
	Float32TypeName   = "float32"
	Float64TypeName   = "float64"
	StringTypeName    = "string"
	StructTypeName    = "struct"
	MapTypeName       = "map"
	PackageTypeName   = "package"
	ErrorTypeName     = "error"
	VoidTypeName      = "void"
	FunctionTypeName  = "func"
	UndefinedTypeName = "undefined"
	ChanTypeName      = "chan"
)

// These are miscellaneous constants used through-out the data package.
const (
	True   = "true"
	False  = "false"
	NoName = ""
)

// Function defines a function, which includes the declaration
// metadata for the function as well as the actual function pointer,
// which can be either bytecode or a runtime package function.
type Function struct {
	// The declaration for the function. If nil, then there is no
	// declaration defined, This should be an error condition.
	Declaration *Declaration

	// The value of the function. For a compiled Ego function, this
	// is a pointer to the assicated byte code. For a native function,
	// this is the function value.
	Value interface{}
}

// Type defines the type of an Ego object. All types have a kind which
// is a unique numeric identifier for the type. They may have additional
// information abou the type, such as it's name, the package that defines
// it, the list of fields in the object if it's a struct, and the type of
// the underlying type, key, or data values. Additionally, it contains a
// list of the receiver functions that can respond to an object of this
// type.
type Type struct {
	name      string
	pkg       string
	kind      int
	fields    map[string]*Type
	functions map[string]Function
	keyType   *Type
	valueType *Type
}

// Field defines the name and type of a structure field.
type Field struct {
	Name string
	Type *Type
}

// This map caches whether a given type implements a given interface.
// Initially this is not known, but after the first validation, the
// result is stored here to accelerate any subsequent evaluations.
// The cache consists of a map for each type::interface pair, and
// a mutex to ensure serialized access to this cache.
var implements map[string]bool

var validationLock sync.Mutex

// Get retrieves a named attribute (a field or a method)
// from the type.
func (t Type) Get(name string) interface{} {
	if v, found := t.fields[name]; found {
		return v
	}

	if v, found := t.functions[name]; found {
		return v
	}

	return nil
}

// ValidateInterfaceConformity compares the functions for a given type against
// the functions for an associated interface definition. This is used
// to determine if a given type conforms to an interface type.
func (t Type) ValidateInterfaceConformity(i *Type) error {
	if i.kind != TypeKind || i.valueType == nil {
		return errors.ErrArgumentType
	}

	// Sadly, multple threads using the same type could have a collision in
	// the map object, so serialize this check.
	validationLock.Lock()
	defer validationLock.Unlock()

	// Have we already checked this type once before? If so, use the cached
	// value if it previously was valid.
	if implements[t.name+"::"+i.name] {
		return nil
	}

	for k, bc1 := range t.functions {
		if bc1.Declaration == nil {
			return errors.ErrMissingInterface.Context(k)
		}

		if bc2, ok := i.functions[k]; ok {
			if bc2.Declaration == nil {
				return errors.ErrMissingInterface.Context(bc1.Declaration.String())
			}

			if bc1.Declaration.String() != bc2.Declaration.String() {
				return errors.ErrMissingInterface.Context(bc1.Declaration.String())
			}
		} else {
			return errors.ErrMissingInterface.Context(bc1.Declaration.String())
		}
	}

	// First time checking this type object, let's record that we validated
	// it against the interface type. Make a new map if needed, and set the
	// cache value.
	if implements == nil {
		implements = map[string]bool{}
	}

	implements[t.name+"::"+i.name] = true

	return nil
}

// Return a flag indicating if the given type includes receiver function
// definitions.
func (t Type) HasFunctions() bool {
	if t.kind == TypeKind {
		t = *t.valueType
	}

	return len(t.functions) > 0
}

// Return a string containing the list of receiver functions for
// this type. If there are no functions defined, it returns an
// empty string. The results are a comma-separated list of function
// names plus "()".
func (t Type) FunctionNameList() string {
	if t.kind == TypeKind {
		t = *t.valueType
	}

	if len(t.functions) == 0 || t.kind == FunctionKind {
		return ""
	}

	b := strings.Builder{}
	b.WriteString(",")

	keys := make([]string, 0)

	for k, v := range t.functions {
		if v.Declaration != nil {
			keys = append(keys, v.Declaration.String())
		} else {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}

		b.WriteString(k)

		// If the name isn't a fully-formed name from the
		// types formatter, add a "()" as a courtesy to
		// otherwise-undecorated method names.
		if !strings.Contains(k, "(") {
			b.WriteString("()")
		}
	}

	return b.String()
}

// Return a string array containing the list of receiver functions for
// this type. If there are no functions defined, it returns an
// empty array.
func (t Type) FunctionNames() []string {
	result := []string{}

	if t.kind == TypeKind {
		t = *t.valueType
	}

	if len(t.functions) == 0 {
		return result
	}

	keys := make([]string, 0)

	for k, v := range t.functions {
		if v.Declaration != nil {
			keys = append(keys, v.Declaration.String())
		} else {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	for _, k := range keys {
		b := strings.Builder{}
		b.WriteString(k)

		// If the name isn't a fully-formed name from the
		// types formatter, add a "()" as a courtesy to
		// otherwise-undecorated method names.
		if !strings.Contains(k, "(") {
			b.WriteString("()")
		}

		result = append(result, b.String())
	}

	return result
}

func (t Type) TypeString() string {
	if t.IsTypeDefinition() {
		name := t.name
		if t.pkg != "" {
			name = t.pkg + "." + name
		}

		return name
	}

	return t.String()
}

// FullTypeString returns the type by name but also includes the
// full underlying type definition.
func (t Type) ShortTypeString() string {
	var ptr string

	if t.kind == PointerKind {
		t = *t.valueType
		ptr = "*"
	}

	if t.kind == TypeKind {
		name := t.name
		if t.pkg != "" {
			name = t.pkg + "." + name
		}

		return ptr + name
	}

	return t.String()
}

// Produce a short-form string of the Type. In short (!) if
// the type is a declared type, we show it's name. Otherwise,
// if it's a primitive type, we show the text of the type.
func (t Type) ShortString() string {
	if t.kind == TypeKind {
		return t.name
	}

	return t.String()
}

// Produce a human-readable version of the type definition.
func (t Type) String() string {
	switch t.kind {
	case FunctionKind:
		f := t.functions[t.name]

		return "func" + f.Declaration.String()

	case TypeKind:
		name := t.name
		if t.pkg != "" {
			name = t.pkg + "." + name
		}

		return name + " " + t.valueType.String()

	case InterfaceKind:
		name := "interface{"

		keys := []string{}

		for k := range t.functions {
			keys = append(keys, k)
		}

		for i, f := range keys {
			if i > 0 {
				name = name + ","
			}

			if fd, ok := t.functions[f]; ok {
				name = name + fd.Declaration.String()
			} else {
				name = name + f
			}
		}

		name = name + "}"

		return name

	case MapKind:
		return "map[" + t.keyType.String() + "]" + t.valueType.String()

	case PointerKind:
		return "*" + t.valueType.String()

	case ArrayKind:
		return "[]" + t.valueType.String()

	case StructKind:
		// If there are fields, let's include that in the type info?
		b := strings.Builder{}
		b.WriteString(StructTypeName)

		if t.fields != nil && len(t.fields) > 0 {
			b.WriteString("{")

			keys := make([]string, 0)
			for k := range t.fields {
				keys = append(keys, k)
			}

			sort.Strings(keys)

			for i, k := range keys {
				if i > 0 {
					b.WriteString(", ")
				}

				b.WriteString(k)
				b.WriteString(" ")
				b.WriteString(t.fields[k].String())
			}

			b.WriteString("}")
		}

		return b.String()

	default:
		return t.name
	}
}

// For a given type, return it's kind (i.e. int, string, etc.). This
// is mostly used for parsing, and for switch statements based on
// Ego data types.
func (t Type) Kind() int {
	return t.kind
}

// For a given type, return true if the type is of the given scalar
// base kind. Note this cannot be used for user-defined Types.
func (t Type) IsKind(k int) bool {
	return t.kind == k
}

// Return true if this type is a pointer to something.
func (t Type) IsPointer() bool {
	return t.kind == PointerKind
}

// Determine if the type is an interface. This could be
// a simple interface object ("interface{}") or a type
// that specifies an interface.
func (t Type) IsInterface() bool {
	// Is it a straightforward interface?
	if t.Kind() == InterfaceKind {
		return true
	}

	// Is it a user type with a basetype that is an interface?
	return t.Kind() == TypeKind && t.valueType != nil && t.valueType.Kind() == InterfaceKind
}

// IsString returns true if the type represents a string type.
func (t Type) IsString() bool {
	kind := t.kind

	return kind == StringKind
}

// IsIntegerType returns true if the type represents any of the integer
// types. This is used to relax type checking for array initializers,
// for example.
func (t Type) IsIntegerType() bool {
	kind := t.kind

	return kind == ByteKind || kind == IntKind || kind == Int32Kind || kind == Int64Kind
}

// IsFloatType returns true if the type represents any of the floating
// point types. This is used to relax type checking for array initializers,
// for example.
func (t Type) IsFloatType() bool {
	kind := t.kind

	return kind == Float32Kind || kind == Float64Kind
}

// Return true if this type is the same as the provided type.
func (t Type) IsType(i *Type) bool {
	// If one of these is just a type wrapper, we can compare the underlying type.
	if i.kind == TypeKind {
		i = i.valueType
	}

	if t.kind == TypeKind {
		t = *t.valueType
	}

	// Basic kind match. Note special case for interface matching
	// a struct being allowed up to this point (we'll check the
	// function list later).
	if t.kind != TypeKind || i.kind != InterfaceKind {
		if t.kind != i.kind {
			return false
		}
	}

	if t.keyType != nil && i.keyType == nil {
		return false
	}

	if t.kind != InterfaceKind && t.valueType != nil && i.valueType == nil {
		return false
	}

	if t.keyType != nil && i.keyType != nil {
		if !t.keyType.IsType(i.keyType) {
			return false
		}
	}

	if i.kind != InterfaceKind && t.valueType != nil && i.valueType != nil {
		if !t.valueType.IsType(i.valueType) {
			return false
		}
	}

	// If it's a map, we tolerate a key type or value type that are
	// interfaces.
	if t.kind == MapKind && i.kind == MapKind {
		if t.keyType != InterfaceType && !t.keyType.IsType(i.keyType) {
			return false
		}

		if t.valueType != InterfaceType && !t.valueType.IsType(i.valueType) {
			return false
		}

		return true
	}

	// If it's a structure, let's go one better and compare the
	// structure fields to ensure they match in name, number,
	// and types.
	if t.kind == StructKind {
		// Time to see if this is a check for interface matchups
		if i.kind == InterfaceKind {
			for fname := range i.functions {
				fn := TypeOf(t).Function(fname)

				if fn == nil {
					return false
				}
			}

			return true
		} else {
			typeFields := t.FieldNames()
			valueFields := i.FieldNames()

			if len(typeFields) != len(valueFields) {
				return false
			}

			for _, fieldName := range typeFields {
				typeFieldType := t.fields[fieldName]
				if valueFieldType, found := i.fields[fieldName]; found {
					// If either one is a type, find the underlying type(s)
					for typeFieldType.kind == TypeKind {
						typeFieldType = typeFieldType.valueType
					}

					for valueFieldType.kind == TypeKind {
						valueFieldType = valueFieldType.valueType
					}

					// Special case of letting float64/int issues slide?
					if (typeFieldType.kind == Float64Type.kind &&
						valueFieldType.kind == IntType.kind) ||
						(typeFieldType.kind == IntType.kind &&
							valueFieldType.kind == Float64Type.kind) {
						continue
					}

					if !typeFieldType.IsType(valueFieldType) {
						return false
					}
				} else {
					return false
				}
			}
		}
	}

	return true
}

// Returns true if the current type is an array.
func (t Type) IsArray() bool {
	return t.kind == ArrayKind
}

// Returns true if the current type is a type definition created
// by code (as opposed to a base type).
func (t Type) IsTypeDefinition() bool {
	return t.kind == TypeKind
}

// Define a function for a type, that can be used as a receiver
// function.
func (t *Type) DefineFunction(name string, declaration *Declaration, value interface{}) {
	if t.functions == nil {
		t.functions = map[string]Function{}
	}

	t.functions[name] = Function{
		Declaration: declaration,
		Value:       value,
	}
}

// Helper function that defines a set of functions in a single call.
// Note this can only define functipoin values, not declarations.
func (t *Type) DefineFunctions(functions map[string]Function) {
	for k, v := range functions {
		t.DefineFunction(k, v.Declaration, v.Value)
	}
}

// For a given type, add a new field of the given name and type. Returns
// an error if the current type is not a structure, or if the field already
// is defined.
func (t *Type) DefineField(name string, ofType *Type) *Type {
	kind := t.kind
	if kind == TypeKind {
		kind = t.BaseType().kind
	}

	if kind != StructKind {
		ui.WriteLog(ui.InternalLogger, "ERROR: DefineField() called for a type that is not a struct")

		return nil
	}

	if t.fields == nil {
		t.fields = map[string]*Type{}
	} else {
		if _, found := t.fields[name]; found {
			ui.WriteLog(ui.InternalLogger, "ERROR: DefineField() called with duplicate field name %s", name)

			return nil
		}
	}

	t.fields[name] = ofType

	return t
}

// Return a list of all the fieldnames for the type. The array is empty if
// this is not a struct or a struct type.
func (t Type) FieldNames() []string {
	keys := make([]string, 0)

	// If it's a type but isn't a struct type, we're done.
	if (t.kind == TypeKind) && (t.valueType.kind != StructKind) {
		return keys
	}

	// Otherwise, if it isn't a struct at all, we're done.
	if t.kind != StructKind && t.kind != TypeKind {
		return keys
	}

	for k := range t.fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

// Retrieve the type of a field by name. The current type must
// be a structure type, and the field name must exist.
func (t Type) Field(name string) (*Type, error) {
	if t.kind != StructKind {
		return UndefinedType, errors.ErrInvalidStruct
	}

	if t.fields == nil {
		return UndefinedType, errors.ErrInvalidField.Context(name)
	}

	ofType, found := t.fields[name]
	if !found {
		return UndefinedType, errors.ErrInvalidField.Context(name)
	}

	return ofType, nil
}

// Return true if the current type is the undefined type.
func (t Type) IsUndefined() bool {
	return t.kind == UndefinedKind
}

// GetFunctionDeclaration retrieves a function declaration from a
// type. If the type is not a function type, or the declaration
// is not found by name, a nil is returned.
func (t Type) GetFunctionDeclaration(name string) *Declaration {
	if t.functions != nil {
		if fd, found := t.functions[name]; found {
			return fd.Declaration
		}
	}

	return nil
}

// Retrieve a receiver function from the given type. Returns
// nil if there is no such function.
func (t Type) Function(name string) interface{} {
	var v interface{}

	ok := false

	if t.functions != nil {
		v, ok = t.functions[name]
	}

	if !ok && t.kind == TypeKind && t.valueType != nil {
		return t.valueType.Function(name)
	}

	return v
}

// For a given type, return the type of its base type. So for
// an array, this is the type of each array element. For a pointer,
// it is the type it points to.
func (t *Type) BaseType() *Type {
	if t.valueType != nil {
		return t.valueType
	}

	return t
}

// For a given type, return the key type. This only applies to arrays
// and will return a nil pointer for any other type.
func (t Type) KeyType() *Type {
	return t.keyType
}

// Return the name of the type (not the same as the
// formatted string, but usually refers to a user-defined
// type name).
func (t Type) Name() string {
	return t.name
}

// Return the kind of the type passed in. All pointers are reported
// as a pointer type, without the pointer designation.

func KindOf(i interface{}) int {
	switch i.(type) {
	case *interface{}, **sync.WaitGroup, **sync.Mutex, *string:
		return PointerKind

	case *bool, *int, *int32, *byte, *int64:
		return PointerKind

	case *float32, *float64:
		return PointerKind

	case *sync.WaitGroup:
		return WaitGroupKind

	case *sync.Mutex:
		return MutexKind

	case bool:
		return BoolKind

	case byte:
		return ByteKind

	case int32:
		return Int32Kind

	case int:
		return IntKind

	case int64:
		return Int64Kind

	case float32:
		return Float32Kind

	case float64:
		return Float64Kind

	case string:
		return StringKind

	case Package:
		return UndefinedKind

	case *Package:
		return PackageKind

	case *Map:
		return MapKind

	case *Struct:
		return StructKind

	case *Channel:
		return PointerKind

	default:
		return InterfaceKind
	}
}

// IsNumeric determines if the value passed is an numeric type. The
// parameter value can be an actual value (int, byte, float32, etc)
// or a Type which represents a numeric value.
func IsNumeric(i interface{}) bool {
	switch actual := i.(type) {
	case int, int32, int64, byte, float32, float64:
		return true

	case *Type:
		if actual.kind == ByteKind ||
			actual.kind == IntKind ||
			actual.kind == Int32Kind ||
			actual.kind == Int64Kind ||
			actual.kind == Float32Kind ||
			actual.kind == Float64Kind {
			return true
		}
	}

	return false
}

// TypeOf accepts an interface of arbitrary Ego or native data type,
// and returns the associated type specification, such as data.intKind
// or data.stringKind.
func TypeOf(i interface{}) *Type {
	switch v := i.(type) {
	case Type:
		if baseType := v.BaseType(); baseType != nil {
			return baseType
		}

		return &v

	case *Type:
		if v.kind == TypeKind {
			if baseType := v.BaseType(); baseType != nil {
				return baseType
			}
		}

		return v

	case *interface{}:
		baseType := TypeOf(*v)

		return PointerType(baseType)

	case *sync.WaitGroup:
		return WaitGroupType

	case **sync.WaitGroup:
		return PointerType(WaitGroupType)

	case *sync.Mutex:
		return MutexType

	case **sync.Mutex:
		return PointerType(MutexType)

	case bool:
		return BoolType

	case byte:
		return ByteType

	case int32:
		return Int32Type

	case int:
		return IntType

	case int64:
		return Int64Type

	case float32:
		return Float32Type

	case float64:
		return Float64Type

	case string:
		return StringType

	case *byte:
		return PointerType(ByteType)

	case *int32:
		return PointerType(Int32Type)

	case *int:
		return PointerType(IntType)

	case *int64:
		return PointerType(Int64Type)

	case *float32:
		return PointerType(Float32Type)

	case *float64:
		return PointerType(Float64Type)

	case *string:
		return PointerType(StringType)

	case *bool:
		return PointerType(BoolType)

	case *Package:
		return PointerType(TypeOf(*v))

	case *Map:
		return v.Type()

	case *Struct:
		return v.typeDef

	case *Array:
		return &Type{
			name:      "[]",
			kind:      ArrayKind,
			valueType: v.valueType,
		}

	case *Channel:
		return PointerType(ChanType)

	case *errors.Error, errors.Error:
		return ErrorType

	default:
		return InterfaceType
	}
}

// IsType accepts an arbitrary value that is either an Ego or native data
// value, and a type specification, and indicates if it is of the provided
// Ego datatype indicator.
func IsType(v interface{}, t *Type) bool {
	if t.kind == InterfaceKind {
		// If it is an empty interface (no methods) then it's always true
		if len(t.functions) == 0 {
			return true
		}

		// Search to see if the interface has all the functions available.
		for m := range t.functions {
			found := true
			switch mv := v.(type) {
			case *Struct:
				_, found = mv.Get(m)

			case *Type:
				_, found = mv.functions[m]
			}

			if !found {
				return false
			}
		}

		return true
	}

	return t.IsType(TypeOf(v))
}

// Compare the value to the base type of the type given. This recursively peels
// away any type definition layers and compares the value type to the ultimate
// base type.  If the type passed in is already a base type, this is no different
// than calling IsType() directly.
func IsBaseType(v interface{}, t *Type) bool {
	valid := IsType(v, t)
	if !valid && t.IsTypeDefinition() {
		valid = IsBaseType(v, t.valueType)
	}

	return valid
}

// For a given interface pointer, unwrap the pointer and return the type it
// actually points to.
func TypeOfPointer(v interface{}) *Type {
	if p, ok := v.(Type); ok {
		if p.kind != PointerKind || p.valueType == nil {
			return UndefinedType
		}

		return p.valueType
	}

	// Is this a pointer to an actual native interface?
	p, ok := v.(*interface{})
	if !ok {
		return UndefinedType
	}

	actual := *p

	return TypeOf(actual)
}

// Determine if the given value is "nil". This an be either an actual
// nil value, or a value that represents the "nil values" for the given
// type (which are recorded as the address of the zero value).
func IsNil(v interface{}) bool {
	// Is it outright a nil value?
	if v == nil {
		return true
	}

	// Is it a nil error message?
	if err, ok := v.(error); ok {
		return err == nil
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
	} else if addr == &float64Interface {
		return true
	} else if addr == &interfaceModel {
		return true
	}

	return false
}

// Is this type associated with a native Ego type that has
// extended native function support?
func IsNativeType(kind int) bool {
	return kind > minimumNativeType && kind < maximumNativeType
}

// For a given type, return the native package that contains
// it. For example, sync.WaitGroup would return "sync".
func PackageForKind(kind int) string {
	for _, item := range TypeDeclarations {
		if item.Kind.kind == kind {
			// If this is a pointer type, skip the pointer token
			if item.Tokens[0] == "*" {
				return item.Tokens[1]
			}

			return item.Tokens[0]
		}
	}

	return ""
}

func (t *Type) SetPackage(name string) *Type {
	if name == "main" {
		name = ""
	}

	t.pkg = name

	return t
}

func (t *Type) SetName(name string) *Type {
	t.name = name

	return t
}
