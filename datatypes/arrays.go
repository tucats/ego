package datatypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tucats/ego/errors"
)

// EgoArray is the representation in native code of an Ego array. This includes
// an array of interfaces that contain the actual data items, a base type (which
// may be InterfaceType if the array is untypted) and a counting semaphore used
// to track if the array should be considered writable or not.
type EgoArray struct {
	data      []interface{}
	bytes     []byte
	valueType Type
	immutable int
}

// Create a new empty array of the given type and size. The values of the array
// members are all initialized to nil. Note special case for []byte which is stored
// natively so it can be used with native Go methods that expect a byte array.
func NewArray(valueType Type, size int) *EgoArray {
	if valueType.kind == ByteKind {
		m := &EgoArray{
			bytes:     make([]byte, size),
			valueType: valueType,
			immutable: 0,
		}

		return m
	}

	m := &EgoArray{
		data:      make([]interface{}, size),
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// NewArrayFromArray accepts a type and an array of interfaces, and constructs
// an EgoArray that uses the source array as it's base array. Note special
// processing for []byte which results in a native Go []byte array.
func NewArrayFromArray(valueType Type, source []interface{}) *EgoArray {
	if valueType.kind == ArrayKind && valueType.BaseType().kind == ByteKind {
		m := &EgoArray{
			bytes:     make([]byte, len(source)),
			valueType: valueType,
			immutable: 0,
		}

		for n, v := range source {
			m.bytes[n] = byte(GetInt(v))
		}

		return m
	}

	m := &EgoArray{
		data:      source,
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// Make creates a new array patterned off of the type of the receiver array,
// of the given size. Note special handling for []byte types which creates
// a native Go array.
func (a *EgoArray) Make(size int) *EgoArray {
	if a.valueType.kind == ArrayKind && a.valueType.BaseType().kind == ByteKind {
		m := &EgoArray{
			bytes:     make([]byte, size),
			valueType: a.valueType,
			immutable: 0,
		}

		return m
	}

	m := &EgoArray{
		data:      make([]interface{}, size),
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
func (a *EgoArray) DeepEqual(b *EgoArray) bool {
	if a.valueType.IsType(InterfaceType) || b.valueType.IsType(InterfaceType) {
		return reflect.DeepEqual(a.data, b.data)
	}

	return reflect.DeepEqual(a, b)
}

// BaseArray returns the underlying native array that contains the individual
// array members. This is needed for things like sort.Slice(). Note that if its
// a []byte type, we must convert the native Go array into an []interface{}
// first...
func (a *EgoArray) BaseArray() []interface{} {
	r := a.data

	if a.valueType.kind == ByteKind {
		r = make([]interface{}, len(a.bytes))

		for index := range r {
			r[index] = a.bytes[index]
		}
	}

	return r
}

// ValueType returns the base type of the array.
func (a *EgoArray) ValueType() Type {
	return a.valueType
}

// Validate checks that all the members of the array are of a given
// type. This is used to validate anonymous arrays for use as a typed
// array.
func (a *EgoArray) Validate(kind Type) *errors.EgoError {
	if kind.IsType(InterfaceType) {
		return nil
	}

	// Special case for []byte, which requires an integer value. For
	// all other array types, validate compatible type with existing
	// array member
	if a.valueType.Kind() == ByteType.kind {
		if !kind.IsIntegerType() {
			return errors.New(errors.ErrWrongArrayValueType)
		}
	} else {
		for _, v := range a.data {
			if !IsType(v, kind) {
				return errors.New(errors.ErrWrongArrayValueType)
			}
		}
	}

	return nil
}

// Immutable sets or clears the flag that marks the array as immutable. When
// an array is marked as immutable, it cannot be modified (but can be deleted
// in it's entirety). Note that this function actually uses a semaphore to
// track the state, so there must bre an exact match of calls to Immutable(false)
// as there were to Immutable(true) to allow modifiations to the array.
func (a *EgoArray) Immutable(b bool) {
	if b {
		a.immutable++
	} else {
		a.immutable--
	}
}

// Get retrieves a member of the array. If the array index is out-of-bounds
// for the array size, an error is returned.
func (a *EgoArray) Get(i interface{}) (interface{}, *errors.EgoError) {
	index := getInt(i)

	if a.valueType.Kind() == ByteKind {
		if index < 0 || index >= len(a.bytes) {
			return nil, errors.New(errors.ErrArrayBounds)
		}

		return a.bytes[index], nil
	}

	if index < 0 || index >= len(a.data) {
		return nil, errors.New(errors.ErrArrayBounds)
	}

	return a.data[index], nil
}

// Len returns the length of the array.
func (a *EgoArray) Len() int {
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
func (a *EgoArray) SetType(i Type) *errors.EgoError {
	if a.valueType.IsType(InterfaceType) {
		a.valueType = i

		return nil
	}

	return errors.New(errors.ErrImmutableArray)
}

// Force the size of the array. Existing values are retained if the
// array grows; existing values are truncated if the size is reduced.
func (a *EgoArray) SetSize(size int) {
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
		a.data = append(a.data, make([]interface{}, size-len(a.data))...)
	}
}

// Set stores a value in the array. The array must not be set to immutable.
// The array index must be within the size of the array. If the array is a
// typed array, the type must match the array type. The value can handle
// conversion of integer and float types to fit the target array base type.
func (a *EgoArray) Set(i interface{}, value interface{}) *errors.EgoError {
	v := value

	if a.immutable > 0 {
		return errors.New(errors.ErrImmutableArray)
	}

	index := getInt(i)

	// If it's a []byte array, use the native byte array, else use
	// the Ego array.
	if a.valueType.Kind() == ByteType.Kind() {
		if index < 0 || index >= len(a.bytes) {
			return errors.New(errors.ErrArrayBounds)
		}
	} else {
		if index < 0 || index >= len(a.data) {
			return errors.New(errors.ErrArrayBounds)
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
	if a.valueType.Kind() == ByteType.Kind() && !TypeOf(v).IsIntegerType() {
		return errors.New(errors.ErrWrongArrayValueType)
	}

	if a.valueType.Kind() == ByteKind {
		i := GetInt32(value)
		a.bytes[index] = byte(i)
	} else {
		a.data[index] = v
	}

	return nil
}

// Simplified Set() that does no type checking. Used internally to
// load values into an array that is known to be of the correct
// kind.
func (a *EgoArray) SetAlways(i interface{}, value interface{}) {
	if a.immutable > 0 {
		return
	}

	index := getInt(i)
	if index < 0 || index >= len(a.data) {
		return
	}

	if a.valueType.Kind() == ByteKind {
		a.bytes[index] = byte(GetInt(value))
	} else {
		a.data[index] = value
	}
}

// Generate a type description string for this array.
func (a *EgoArray) TypeString() string {
	return fmt.Sprintf("[]%s", a.valueType.String())
}

// Make a string representation of the array suitable for display.
func (a *EgoArray) String() string {
	var b strings.Builder

	b.WriteString("[")

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

			b.WriteString(Format(element))
		}
	}

	b.WriteString("]")

	return b.String()
}

// Fetach a slide of the underlying array and return it as an array of interfaces.
// This can't be used directly as a new array, but can be used to create a new
// array.
func (a *EgoArray) GetSlice(first, last int) ([]interface{}, *errors.EgoError) {
	if first < 0 || last > len(a.data) {
		return nil, errors.New(errors.ErrArrayBounds)
	}

	// If it's a []byte we must build an Ego slide from the native bytes.
	if a.valueType.Kind() == ByteType.kind {
		slice := a.bytes[first:last]

		r := make([]interface{}, len(slice))
		for index := range r {
			r[index] = slice[index]
		}

		return r, nil
	}

	return a.data[first:last], nil
}

// Append an item to the array. If the item being appended is an array itself,
// we append the elements of the array.
func (a *EgoArray) Append(i interface{}) {
	if a.valueType.Kind() == ByteType.kind {
		v := byte(GetInt32(i))
		a.bytes = append(a.bytes, v)
	} else {
		switch v := i.(type) {
		case *EgoArray:
			a.data = append(a.data, v.data...)

		default:
			a.data = append(a.data, v)
		}
	}
}

// getInt is a helper function that is used to retrieve an int value from
// an abstract interface, regardless of the interface type. It returns -1
// for any interface type that can't be converted to an int value. This is
// used to retrieve array index parameters.
func getInt(i interface{}) int {
	switch v := i.(type) {
	case byte:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	default:
		return -1
	}
}

// Delete removes an item from the array by index number. The index
// must be a valid array index. The return value is nil if no error
// occurs, else an error if the index is out-of-bounds or the array
// is marked as immutable.
func (a *EgoArray) Delete(i int) *errors.EgoError {
	if i >= len(a.data) || i < 0 {
		return errors.New(errors.ErrArrayBounds)
	}

	if a.immutable != 0 {
		return errors.New(errors.ErrImmutableArray)
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
func (a *EgoArray) Sort() *errors.EgoError {
	var err *errors.EgoError

	switch a.valueType.kind {
	case StringType.kind:
		stringArray := make([]string, a.Len())
		for i, v := range a.data {
			stringArray[i] = GetString(v)
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
			integerArray[i] = GetInt64(v)
		}

		sort.Slice(integerArray, func(i, j int) bool { return integerArray[i] < integerArray[j] })

		for i, v := range integerArray {
			switch a.valueType.kind {
			case ByteType.kind:
				a.data[i] = byte(v)

			case IntType.kind:
				a.data[i] = int(v)

			case Int32Type.kind:
				a.data[i] = int32(v)

			case Int64Type.kind:
				a.data[i] = v

			default:
				return errors.New(errors.ErrInvalidType).Context("sort")
			}
		}

	case Float32Type.kind, Float64Type.kind:
		floatArray := make([]float64, a.Len())
		for i, v := range a.data {
			floatArray[i] = GetFloat64(v)
		}

		sort.Float64s(floatArray)

		for i, v := range floatArray {
			switch a.valueType.kind {
			case Float32Type.kind:
				a.data[i] = float32(v)

			case Float64Type.kind:
				a.data[i] = v

			default:
				return errors.New(errors.ErrInvalidType).Context("sort")
			}
		}

	default:
		err = errors.New(errors.ErrInvalidArgType)
	}

	return err
}

// MarshalJSON converts the array representation to valid JSON and returns
// the data as a byte array.
func (a EgoArray) MarshalJSON() ([]byte, error) {
	b := strings.Builder{}
	b.WriteString("[")

	if a.valueType.Kind() == ByteType.Kind() {
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
