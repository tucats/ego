package datatypes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tucats/ego/errors"
)

type EgoArray struct {
	data      []interface{}
	valueType Type
	immutable int
}

func NewArray(valueType Type, size int) *EgoArray {
	m := &EgoArray{
		data:      make([]interface{}, size),
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// NewArrayFromArray accepts a type and an array of interfaces, and constructs
// an EgoArray that uses the source array as it's base array.
func NewArrayFromArray(valueType Type, source []interface{}) *EgoArray {
	m := &EgoArray{
		data:      source,
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// Make creates a new array patterned off of the type of the receiver array,
// of the given size.
func (a *EgoArray) Make(size int) *EgoArray {
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

func (a *EgoArray) DeepEqual(b *EgoArray) bool {
	if a.valueType.IsType(InterfaceType) || b.valueType.IsType(InterfaceType) {
		return reflect.DeepEqual(a.data, b.data)
	}

	return reflect.DeepEqual(a, b)
}

func (a *EgoArray) BaseArray() []interface{} {
	return a.data
}

func (a *EgoArray) ValueType() Type {
	return a.valueType
}

func (a *EgoArray) Validate(kind Type) *errors.EgoError {
	if kind.IsType(InterfaceType) {
		return nil
	}

	for _, v := range a.data {
		if !IsType(v, kind) {
			return errors.New(errors.ErrWrongArrayValueType)
		}
	}

	return nil
}

func (a *EgoArray) Immutable(b bool) {
	if b {
		a.immutable++
	} else {
		a.immutable--
	}
}

func (a *EgoArray) Get(i interface{}) (interface{}, *errors.EgoError) {
	index := getInt(i)
	if index < 0 || index >= len(a.data) {
		return nil, errors.New(errors.ErrArrayBounds)
	}

	return a.data[index], nil
}

func (a *EgoArray) Len() int {
	return len(a.data)
}

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
	// If we hae to expand it, append empty items to the array
	if size < len(a.data) {
		a.data = a.data[:size]
	} else {
		a.data = append(a.data, make([]interface{}, size-len(a.data))...)
	}
}

func (a *EgoArray) Set(i interface{}, value interface{}) *errors.EgoError {
	v := value

	if a.immutable > 0 {
		return errors.New(errors.ErrImmutableArray)
	}

	index := getInt(i)
	if index < 0 || index >= len(a.data) {
		return errors.New(errors.ErrArrayBounds)
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

	// Now, ensure it's of the right type for this array.
	if !IsBaseType(v, a.valueType) {
		return errors.New(errors.ErrWrongArrayValueType)
	}

	a.data[index] = v

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

	a.data[index] = value
}

// Generate a type string for this array.
func (a *EgoArray) TypeString() string {
	return fmt.Sprintf("[]%s", a.valueType.String())
}

// Make a string representation of the array suitable for human reading.
func (a *EgoArray) String() string {
	var b strings.Builder

	b.WriteString("[")

	for i, element := range a.data {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(Format(element))
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

	return a.data[first:last], nil
}

// Append an item to the array. If the item being appended is an array itself,
// we append the elements of the array.
func (a *EgoArray) Append(i interface{}) {
	switch v := i.(type) {
	case *EgoArray:
		a.data = append(a.data, v.data...)

	default:
		a.data = append(a.data, v)
	}
}

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

	a.data = append(a.data[:i], a.data[i+1:]...)

	return nil
}

func (a *EgoArray) Sort() *errors.EgoError {
	var err *errors.EgoError

	if a.valueType.IsType(StringType) {
		stringArray := make([]string, a.Len())
		for i, v := range a.data {
			stringArray[i] = GetString(v)
		}

		sort.Strings(stringArray)

		for i, v := range stringArray {
			a.data[i] = v
		}
	} else if a.valueType.IsType(IntType) {
		values := make([]int, a.Len())
		for i, v := range a.data {
			values[i] = GetInt(v)
		}

		sort.Ints(values)

		for i, v := range values {
			a.data[i] = v
		}
	} else if a.valueType.IsType(Float64Type) {
		values := make([]float64, a.Len())
		for i, v := range a.data {
			values[i] = GetFloat64(v)
		}

		sort.Float64s(values)

		for i, v := range values {
			a.data[i] = v
		}
	} else {
		err = errors.New(errors.ErrInvalidArgType)
	}

	return err
}

func (a EgoArray) MarshalJSON() ([]byte, error) {
	b := strings.Builder{}
	b.WriteString("[")

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

	b.WriteString("]")

	buff := b.String()

	return []byte(buff), nil
}
