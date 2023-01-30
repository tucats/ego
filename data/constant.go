package data

type Immutable struct {
	Value interface{}
}

func Constant(v interface{}) Immutable {
	return Immutable{Value: v}
}

func UnwrapConstant(i interface{}) interface{} {
	if v, ok := i.(Immutable); ok {
		return v.Value
	}

	return i
}
