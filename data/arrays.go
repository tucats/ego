package data

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/tucats/ego/errors"
)

// Array is the representation in native code of an Ego array. This includes
// an array of interfaces that contain the actual data items, a base type (which
// may be InterfaceType if the array is un-typed) and a counting semaphore used
// to track if the array should be considered writable or not.
type Array struct {
	// data is an array of each element of the Ego array, unless the base type is
	// ByteType.
	data []any

	// bytes is an array of each element of the Ego array when the base type is byte.
	// This facilitates efficient manipulation of byte arrays when passed to native
	// go functions.
	bytes []byte

	// The type of each element in the array. If the array is not homogeneous, the
	// type is InterfaceType.
	valueType *Type

	// A counting semaphore that tracks if the array is consider readonly or not.
	immutable int
}

// Create a new empty array of the given type and size. The values of the array
// members are all initialized to nil. Note special case for []byte which is stored
// natively so it can be used with native Go methods that expect a byte array.
func NewArray(valueType *Type, size int) *Array {
	if valueType.kind == ByteKind {
		m := &Array{
			bytes:     make([]byte, size),
			valueType: valueType,
			immutable: 0,
		}

		return m
	}

	m := &Array{
		data:      make([]any, size),
		valueType: valueType,
		immutable: 0,
	}

	// For common scalar types, set the initial value to appropriate "zero" value.
	for index := range m.data {
		switch valueType.kind {
		case BoolType.kind:
			m.data[index] = false

		case ByteType.kind:
			m.data[index] = byte(0)

		case Int32Type.kind:
			m.data[index] = int32(0)

		case Int64Type.kind:
			m.data[index] = int64(0)

		case IntType.kind:
			m.data[index] = int(0)

		case Float32Type.kind:
			m.data[index] = float32(0)

		case Float64Type.kind:
			m.data[index] = float64(0)

		case StringType.kind:
			m.data[index] = ""
		}
	}

	return m
}

// NewArrayFromInterfaces will generate an array that contains the values
// passed as the sources values.
func NewArrayFromInterfaces(valueType *Type, elements ...any) *Array {
	return NewArrayFromList(valueType, NewList(elements...))
}

// NewArrayFromStrings helper function creates an Ego []string from the provided
// list of string argument values.
func NewArrayFromStrings(elements ...string) *Array {
	lines := make([]any, len(elements))
	for n, s := range elements {
		lines[n] = s
	}

	return NewArrayFromList(StringType, NewList(lines...))
}

// NewArrayFromBytes helper function creates an Ego array of bytes
// from the provided list of byte argument values.
func NewArrayFromBytes(elements ...byte) *Array {
	b := make([]byte, len(elements))
	copy(b, elements)

	a := &Array{
		bytes:     b,
		valueType: ByteType,
		immutable: 0,
	}

	return a
}

// NewArrayFromList accepts a type and an array of interfaces, and constructs
// an EgoArray that uses the source array as it's base array. Note special
// processing for []byte which results in a native Go []byte array.
func NewArrayFromList(valueType *Type, source List) *Array {
	isByteArray := valueType.kind == ArrayKind && valueType.BaseType().kind == ByteKind
	isByteArray = isByteArray || (valueType.kind == ByteKind)

	if isByteArray {
		m := &Array{
			bytes:     make([]byte, source.Len()),
			valueType: valueType,
			immutable: 0,
		}

		for n, v := range source.Elements() {
			m.bytes[n], _ = Byte(v)
		}

		return m
	}

	data := make([]any, len(source.elements))

	for k, v := range source.elements {
		switch actual := v.(type) {
		case []any:
			v = NewArrayFromInterfaces(InterfaceType, actual...)

		case map[string]any:
			v = NewStructFromMap(actual)

		case map[any]any:
			v = NewMapFromMap(actual)
		}

		data[k] = v
	}

	m := &Array{
		data:      data,
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// Make creates a new array patterned off of the type of the receiver array,
// of the given size. Note special handling for []byte types which creates
// a native Go array.
func (a *Array) Make(size int) *Array {
	if a == nil {
		return nil
	}

	if a.valueType.kind == ByteKind {
		m := &Array{
			bytes:     make([]byte, size),
			valueType: a.valueType,
			immutable: 0,
		}

		return m
	}

	m := &Array{
		data:      make([]any, size),
		valueType: a.valueType,
		immutable: 0,
	}

	model := InstanceOfType(a.valueType)

	for index := range m.data {
		m.data[index] = model
	}

	return m
}

// DeepEqual is a recursive compare with another Ego array. The recursive
// compare is performed on each member of the array.
func (a *Array) DeepEqual(b *Array) bool {
	if a == nil {
		return false
	}

	if a.valueType.IsType(InterfaceType) || b.valueType.IsType(InterfaceType) {
		return reflect.DeepEqual(a.data, b.data)
	}

	return reflect.DeepEqual(a, b)
}

// BaseArray returns the underlying native array that contains the individual
// array members. This is needed for things like sort.Slice(). Note that if its
// a []byte type, we must convert the native Go array into an []any
// first...
func (a *Array) BaseArray() []any {
	if a == nil {
		return nil
	}

	r := a.data

	if a.valueType.kind == ByteKind {
		r = make([]any, len(a.bytes))

		for index := range r {
			r[index] = a.bytes[index]
		}
	}

	return r
}

// Type returns the base type of the array.
func (a *Array) Type() *Type {
	if a != nil {
		return a.valueType
	}

	return nil
}

// Validate checks that all the members of the array are of a given
// type. This is used to validate anonymous arrays for use as a typed
// array.
func (a *Array) Validate(kind *Type) error {
	if a == nil {
		return errors.ErrNilPointerReference
	}

	if kind.IsType(InterfaceType) {
		return nil
	}

	// Special case for []byte, which requires an integer value. For
	// all other array types, validate compatible type with existing
	// array member
	if a.valueType.Kind() == ByteType.kind {
		if !kind.IsIntegerType() {
			return errors.ErrWrongArrayValueType
		}
	} else {
		for _, v := range a.data {
			if !IsType(v, kind) {
				return errors.ErrWrongArrayValueType
			}
		}
	}

	return nil
}

// SetReadonly sets or clears the flag that marks the array as immutable. When
// an array is marked as immutable, it cannot be modified (but can be deleted
// in it's entirety). Note that this function actually uses a semaphore to
// track the state, so there must bre an exact match of calls to SetReadonly(false)
// as there were to SetReadonly(true) to allow modifications to the array.
func (a *Array) SetReadonly(b bool) *Array {
	if a == nil {
		return nil
	}

	if b {
		a.immutable++
	} else {
		a.immutable--
	}

	return a
}

// Get retrieves a member of the array. If the array index is out-of-bounds
// for the array size, an error is returned.
func (a *Array) Get(index int) (any, error) {
	if a == nil {
		return nil, errors.ErrNilPointerReference
	}

	if a.valueType.Kind() == ByteKind {
		if index < 0 || index >= len(a.bytes) {
			return nil, errors.ErrArrayBounds
		}

		return a.bytes[index], nil
	}

	if index < 0 || index >= len(a.data) {
		return nil, errors.ErrArrayBounds
	}

	return a.data[index], nil
}

// Len returns the length of the array.
func (a *Array) Len() int {
	if a == nil {
		return 0
	}

	if a.valueType.Kind() == ByteKind {
		return len(a.bytes)
	}

	return len(a.data)
}

// SetType can be called once on an anonymous array (whose members are
// all abstract interfaces). This sets the base type of the array to the
// given type. If the array already has a base type, you cannot set a new
// one. This (along with the Validate() function) can be used to convert
// an anonymous array to a typed array.
func (a *Array) SetType(i *Type) error {
	if a == nil {
		return errors.ErrNilPointerReference
	}

	if a.valueType.IsType(InterfaceType) {
		a.valueType = i

		return nil
	}

	return errors.ErrImmutableArray
}

// Force the size of the array. Existing values are retained if the
// array grows; existing values are truncated if the size is reduced.
func (a *Array) SetSize(size int) *Array {
	if a == nil {
		return nil
	}

	if size < 0 {
		size = 0
	}

	// If we are making it smaller, just convert the array to a slice of itself.
	// If we hae to expand it, append empty items to the array. Note that if the
	// type is []byte, we operate on the native Go []byte array instead.
	if a.valueType.Kind() == ByteKind {
		if size < len(a.bytes) {
			a.bytes = a.bytes[:size]
		} else {
			a.bytes = append(a.bytes, make([]byte, size-len(a.data))...)
		}
	}

	if size < len(a.data) {
		a.data = a.data[:size]
	} else {
		a.data = append(a.data, make([]any, size-len(a.data))...)
	}

	return a
}

// Set stores a value in the array. The array must not be set to immutable.
// The array index must be within the size of the array. If the array is a
// typed array, the type must match the array type. The value can handle
// conversion of integer and float types to fit the target array base type.
func (a *Array) Set(index int, value any) error {
	if a == nil {
		return errors.ErrNilPointerReference
	}

	v := value

	if a.immutable > 0 {
		return errors.ErrImmutableArray
	}

	// If it's a []byte array, use the native byte array, else use
	// the Ego array.
	if a.valueType.Kind() == ByteKind {
		if index < 0 || index >= len(a.bytes) {
			return errors.ErrArrayBounds
		}
	} else {
		if index < 0 || index >= len(a.data) {
			return errors.ErrArrayBounds
		}
	}

	// Address float64/int issues before testing the type.
	if a.valueType.kind == IntKind {
		if x, ok := v.(float64); ok {
			v = int(x)
		}
	}

	if a.valueType.kind == Float64Kind {
		if x, ok := v.(float32); ok {
			v = float64(x)
		} else if x, ok := v.(int); ok {
			v = float64(x)
		} else if x, ok := v.(int32); ok {
			v = float64(x)
		} else if x, ok := v.(int64); ok {
			v = float64(x)
		}
	}

	if a.valueType.kind == Float32Kind {
		if x, ok := v.(float64); ok {
			v = float32(x)
		} else if x, ok := v.(int); ok {
			v = float32(x)
		} else if x, ok := v.(int32); ok {
			v = float32(x)
		} else if x, ok := v.(int64); ok {
			v = float32(x)
		}
	}

	// Now, ensure it's of the right type for this array. As always, special case
	// for []byte arrays.
	if a.valueType.Kind() == ByteKind && !TypeOf(v).IsIntegerType() {
		return errors.ErrWrongArrayValueType
	}

	if a.valueType.Kind() == ByteKind {
		i, err := Byte(value)
		if err != nil {
			return err
		}

		a.bytes[index] = i
	} else {
		a.data[index] = v
	}

	return nil
}

// Simplified Set() that does no type checking. Used internally to
// load values into an array that is known to be of the correct
// kind.
func (a *Array) SetAlways(index int, value any) *Array {
	if a == nil {
		return nil
	}

	if a.immutable > 0 {
		return a
	}

	if index < 0 || index >= len(a.data) {
		return a
	}

	if a.valueType.Kind() == ByteKind {
		a.bytes[index], _ = Byte(value)
	} else {
		a.data[index] = value
	}

	return a
}

// Generate a type description string for this array.
func (a *Array) TypeString() string {
	if a == nil {
		return NilTypeName
	}

	return "[]" + a.valueType.String()
}

// Make a string representation of the array suitable for display.
// This is called when you use fmt.Printf with the "%v" operator,
// for example.
func (a *Array) String() string {
	if a == nil {
		return NilTypeName
	}

	var b strings.Builder

	b.WriteString("[")

	isInterface := a.valueType.IsInterface()

	if a.valueType.Kind() == ByteType.kind {
		for i, element := range a.bytes {
			if i > 0 {
				b.WriteString(", ")
			}

			b.WriteString(Format(element))
		}
	} else {
		for i, element := range a.data {
			if i > 0 {
				b.WriteString(", ")
			}

			if isInterface {
				b.WriteString(FormatWithType(element))
			} else {
				b.WriteString(Format(element))
			}
		}
	}

	b.WriteString("]")

	return b.String()
}

// StringWithType make a string representation of the array suitable
// for display, including the type specification and the element values.
// This is called when you use fmt.Printf with the "%V" operator,
// for example.
func (a *Array) StringWithType() string {
	if a == nil {
		return NilTypeName
	}

	var b strings.Builder

	b.WriteString(a.TypeString())
	b.WriteString("{")

	isInterface := a.valueType.IsInterface()

	if a.valueType.Kind() == ByteType.kind {
		for i, element := range a.bytes {
			if i > 0 {
				b.WriteString(", ")
			}

			b.WriteString(FormatWithType(element))
		}
	} else {
		for i, element := range a.data {
			if i > 0 {
				b.WriteString(", ")
			}

			if isInterface {
				b.WriteString(FormatWithType(element))
			} else {
				b.WriteString(Format(element))
			}
		}
	}

	b.WriteString("}")

	return b.String()
}

// Fetch a slice of the underlying array and return it as an array of interfaces.
// This can't be used directly as a new array, but can be used to create a new
// array.
func (a *Array) GetSlice(first, last int) ([]any, error) {
	if a == nil {
		return nil, errors.ErrNilPointerReference
	}

	if first < 0 || last < 0 || first > len(a.data) || last > len(a.data) {
		return nil, errors.ErrArrayBounds
	}

	// If it's a []byte we must build an Ego slide from the native bytes.
	if a.valueType.Kind() == ByteType.kind {
		slice := a.bytes[first:last]

		r := make([]any, len(slice))
		for index := range r {
			r[index] = slice[index]
		}

		return r, nil
	}

	return a.data[first:last], nil
}

// Fetch a slice of the underlying array and return it as an array of interfaces.
// This can't be used directly as a new array, but can be used to create a new
// array.
func (a *Array) GetSliceAsArray(first, last int) (*Array, error) {
	if a == nil {
		return nil, errors.ErrNilPointerReference
	}

	// If it's a byte array, we do this differently.
	if a.valueType.Kind() == ByteType.kind {
		if first < 0 || last < first || first > len(a.bytes) || last > len(a.bytes) {
			return nil, errors.ErrArrayBounds
		}

		slice := a.bytes[first:last]
		r := NewArrayFromBytes(slice...)

		return r, nil
	}

	if first < 0 || last < first || first > len(a.data) || last > len(a.data) {
		return nil, errors.ErrArrayBounds
	}

	slice, err := a.GetSlice(first, last)
	if err != nil {
		return nil, err
	}

	r := NewArrayFromInterfaces(a.valueType, slice...)

	return r, nil
}

// Append an item to the array. If the item being appended is an array itself,
// we append the elements of the array.
func (a *Array) Append(i any) *Array {
	if a == nil {
		return nil
	}

	if i == nil {
		return a
	}

	if a.valueType.Kind() == ByteKind {
		// If the value is already a byte array, then we just move it into our
		// byte array.
		if ba, ok := i.([]byte); ok {
			if len(a.bytes) == 0 {
				a.bytes = make([]byte, len(ba))
				copy(a.bytes, ba)
			} else {
				a.bytes = append(a.bytes, ba...)
			}
		} else {
			// Otherwise, append the value to the array...
			v, _ := Byte(i)
			a.bytes = append(a.bytes, v)
		}
	} else {
		switch v := i.(type) {
		case *Array:
			a.data = append(a.data, v.data...)

		default:
			a.data = append(a.data, v)
		}
	}

	return a
}

// GetBytes returns the native byte array for this array, or nil if this
// is not a byte array.
func (a *Array) GetBytes() []byte {
	if a == nil {
		return nil
	}

	return a.bytes
}

// Delete removes an item from the array by index number. The index
// must be a valid array index. The return value is nil if no error
// occurs, else an error if the index is out-of-bounds or the array
// is marked as immutable.
func (a *Array) Delete(i int) error {
	if a == nil {
		return errors.ErrNilPointerReference
	}

	if i >= len(a.data) || i < 0 {
		return errors.ErrArrayBounds
	}

	if a.immutable != 0 {
		return errors.ErrImmutableArray
	}

	if a.valueType.Kind() == ByteType.kind {
		a.bytes = append(a.bytes[:i], a.bytes[i+1:]...)
	} else {
		a.data = append(a.data[:i], a.data[i+1:]...)
	}

	return nil
}

// Sort will sort the array into ascending order. It uses either native
// sort functions or the native sort.Slice function to do the sort. This
// can only be performed on an array of scalar types (no structs, arrays,
// or maps).
func (a *Array) Sort() error {
	if a == nil {
		return errors.ErrNilPointerReference
	}

	var err error

	switch a.valueType.kind {
	case StringType.kind:
		stringArray := make([]string, a.Len())
		for i, v := range a.data {
			stringArray[i] = String(v)
		}

		sort.Strings(stringArray)

		for i, v := range stringArray {
			a.data[i] = v
		}

	case ByteType.kind:
		byteArray := make([]byte, len(a.bytes))

		copy(byteArray, a.bytes)
		sort.Slice(byteArray, func(i, j int) bool { return byteArray[i] < byteArray[j] })

		a.bytes = byteArray

	case IntType.kind, Int32Type.kind, Int64Type.kind:
		integerArray := make([]int64, a.Len())
		for i, v := range a.data {
			integerArray[i], err = Int64(v)
			if err != nil {
				return err
			}
		}

		sort.Slice(integerArray, func(i, j int) bool { return integerArray[i] < integerArray[j] })

		for i, v := range integerArray {
			switch a.valueType.kind {
			case ByteType.kind:
				a.data[i], _ = Byte(v)

			case IntType.kind:
				if (v < 0 && -v > math.MaxInt) || (v > 0 && v > math.MaxInt) {
					return errors.ErrOverflow.In("Sort")
				}

				a.data[i] = int(v)

			case Int32Type.kind:
				if (v < 0 && -v > math.MaxInt32) || (v > 0 && v > math.MaxInt32) {
					return errors.ErrOverflow.In("Sort")
				}

				a.data[i] = int32(v)

			case Int64Type.kind:
				a.data[i] = v

			default:
				return errors.ErrInvalidType.In("Sort")
			}
		}

	case Float32Type.kind, Float64Type.kind:
		floatArray := make([]float64, a.Len())
		for i, v := range a.data {
			floatArray[i], err = Float64(v)
			if err != nil {
				return err
			}
		}

		sort.Float64s(floatArray)

		for i, v := range floatArray {
			switch a.valueType.kind {
			case Float32Type.kind:
				a.data[i] = float32(v)

			case Float64Type.kind:
				a.data[i] = v

			default:
				return errors.ErrInvalidType.In("Sort")
			}
		}

	default:
		err = errors.ErrArgumentType
	}

	return err
}

// MarshalJSON converts the array representation to valid JSON and returns
// the data as a byte array.
func (a *Array) MarshalJSON() ([]byte, error) {
	if a == nil {
		return nil, errors.ErrNilPointerReference
	}

	b := strings.Builder{}
	b.WriteString("[")

	if a.valueType.Kind() == ByteKind {
		for k, v := range a.bytes {
			if k > 0 {
				b.WriteString(",")
			}

			b.WriteString(fmt.Sprintf("%v", v))
		}
	} else {
		for k, v := range a.data {
			if k > 0 {
				b.WriteString(",")
			}

			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, errors.New(err)
			}

			b.WriteString(string(jsonBytes))
		}
	}

	b.WriteString("]")

	return []byte(b.String()), nil
}

// Write adds a native []byte array to the array.
func (a *Array) Write(p []byte) (n int, err error) {
	if a == nil {
		return 0, errors.ErrNilPointerReference
	}

	if a.valueType.Kind() != ByteKind {
		return 0, errors.ErrInvalidType.In("Write")
	}

	a.bytes = append(a.bytes, p...)

	return len(p), nil
}
