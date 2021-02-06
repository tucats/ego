package datatypes

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/tucats/gopackages/datatypes"
)

type EgoArray struct {
	data      []interface{}
	valueType int
	immutable int
}

func NewArray(valueType, size int) *EgoArray {
	m := &EgoArray{
		data:      make([]interface{}, size),
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// NewFromArray accepts a type and an array of interfaces, and constructs
// an EgoArray that uses the source array as it's base array.
func NewFromArray(valueType int, source []interface{}) *EgoArray {
	m := &EgoArray{
		data:      source,
		valueType: valueType,
		immutable: 0,
	}

	return m
}

// Make creates a new array pattenered off of the type of the receiver array,
// of the given size.
func (a *EgoArray) Make(size int) *EgoArray {
	m := &EgoArray{
		data:      make([]interface{}, size),
		valueType: a.valueType,
		immutable: 0,
	}

	model := InstanceOf(a.valueType)

	for index := range m.data {
		m.data[index] = model
	}

	return m
}

func (a *EgoArray) DeepEqual(b *EgoArray) bool {
	if a.valueType == InterfaceType || b.valueType == InterfaceType {
		return reflect.DeepEqual(a.data, b.data)
	}

	return reflect.DeepEqual(a, b)
}

func (a *EgoArray) BaseArray() []interface{} {
	return a.data
}

func (a *EgoArray) ValueType() int {
	return a.valueType
}

func (a *EgoArray) Validate(kind int) error {
	if kind == datatypes.InterfaceType {
		return nil
	}

	for i := 0; i < a.Len(); i++ {
		v, _ := a.Get(i)
		if !datatypes.IsType(v, kind) {
			return errors.New(WrongArrayValueType)
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

func (a *EgoArray) Get(i interface{}) (interface{}, error) {
	index := getInt(i)
	if index < 0 || index >= len(a.data) {
		return nil, errors.New(ArrayBoundsError)
	}

	return a.data[index], nil
}

func (a *EgoArray) Len() int {
	return len(a.data)
}

func (a *EgoArray) SetType(i int) error {
	if a.valueType == InterfaceType {
		a.valueType = i

		return nil
	}

	return errors.New(ImmutableArrayError)
}

func (a *EgoArray) Set(i interface{}, value interface{}) error {
	if a.immutable > 0 {
		return errors.New(ImmutableArrayError)
	}

	index := getInt(i)
	if index < 0 || index >= len(a.data) {
		return errors.New(ArrayBoundsError)
	}

	if !IsType(value, a.valueType) {
		return errors.New(WrongArrayValueType)
	}

	a.data[index] = value

	return nil
}

func (a *EgoArray) TypeString() string {
	return fmt.Sprintf("[]%s", TypeString(a.valueType))
}

func (a *EgoArray) String() string {
	var b strings.Builder

	b.WriteString("[")

	for i, v := range a.data {
		if i > 0 {
			b.WriteString(", ")
		}

		if s, ok := v.(string); ok {
			b.WriteString(fmt.Sprintf("\"%s\"", s))
		} else {
			b.WriteString(fmt.Sprintf("%v", v))
		}
	}

	b.WriteString("]")

	return b.String()
}

func (a *EgoArray) GetSlice(first, last int) ([]interface{}, error) {
	if first < 0 || last > len(a.data) {
		return nil, errors.New(ArrayBoundsError)
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
