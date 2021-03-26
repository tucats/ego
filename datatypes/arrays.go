package datatypes

import (
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

// NewFromArray accepts a type and an array of interfaces, and constructs
// an EgoArray that uses the source array as it's base array.
func NewFromArray(valueType Type, source []interface{}) *EgoArray {
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

	model := InstanceOfKind(a.valueType)

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

	for i := 0; i < a.Len(); i++ {
		v, _ := a.Get(i)
		if !IsType(v, kind) {
			return errors.New(errors.WrongArrayValueType)
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
		return nil, errors.New(errors.ArrayBoundsError)
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

	return errors.New(errors.ImmutableArrayError)
}

func (a *EgoArray) Set(i interface{}, value interface{}) *errors.EgoError {
	if a.immutable > 0 {
		return errors.New(errors.ImmutableArrayError)
	}

	index := getInt(i)
	if index < 0 || index >= len(a.data) {
		return errors.New(errors.ArrayBoundsError)
	}

	if !IsType(value, a.valueType) {
		return errors.New(errors.WrongArrayValueType)
	}

	a.data[index] = value

	return nil
}

func (a *EgoArray) TypeString() string {
	return fmt.Sprintf("[]%s", a.valueType.String())
}

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

func (a *EgoArray) GetSlice(first, last int) ([]interface{}, *errors.EgoError) {
	if first < 0 || last > len(a.data) {
		return nil, errors.New(errors.ArrayBoundsError)
	}

	return a.data[first:last], nil
}

func (a *EgoArray) Append(i interface{}) {
	switch v := i.(type) {
	case []interface{}:
		a.data = append(a.data, v...)

	case *EgoArray:
		a.data = append(a.data, v.data)

	default:
		a.data = append(a.data, v)
	}
}

func getInt(i interface{}) int {
	switch v := i.(type) {
	case int:
		return v
	case int32:
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
		return errors.New(errors.ArrayBoundsError)
	}

	if a.immutable != 0 {
		return errors.New(errors.ImmutableArrayError)
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
	} else if a.valueType.IsType(FloatType) {
		values := make([]float64, a.Len())
		for i, v := range a.data {
			values[i] = GetFloat(v)
		}

		sort.Float64s(values)

		for i, v := range values {
			a.data[i] = v
		}
	} else {
		err = errors.New(errors.InvalidArgTypeError)
	}

	return err
}
