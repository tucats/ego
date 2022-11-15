package datatypes

import "fmt"

type Value interface {
	Add(v Value) Value

	Int32Value() int32
	IntValue() int
}

type ValueInt32 struct {
	i int32
}

type ValueInt struct {
	i int
}

func NewValue(anyValue interface{}) Value {
	switch actual := anyValue.(type) {
	case int32:
		return ValueInt32{
			i: actual,
		}

	case int:
		return ValueInt{
			i: actual,
		}
	}

	return nil
}

// Int32Value accessor functions.
func (v ValueInt32) Int32Value() int32 {
	return v.i
}

func (v ValueInt32) IntValue() int {
	return int(v.i)
}

// IntValue accessor funtions.
func (v ValueInt) Int32Value() int32 {
	return int32(v.i)
}

func (v ValueInt) IntValue() int {
	return v.i
}

// String functions.
func (v ValueInt) String() string {
	return fmt.Sprintf("%d", v.i)
}

func (v ValueInt32) String() string {
	return fmt.Sprintf("%d", v.i)
}

// Add functions.
func (v ValueInt32) Add(i Value) Value {
	return NewValue(v.Int32Value() + i.Int32Value())
}

func (v ValueInt) Add(i Value) Value {
	return NewValue(v.IntValue() + i.IntValue())
}
