package data

import (
	"reflect"
	"testing"
)

func TestInstanceOfType(t *testing.T) {
	tests := []struct {
		name string
		t    *Type
		want any
	}{
		{
			name: "test with int type",
			t:    IntType,
			want: int(0),
		},
		{
			name: "test with bool type",
			t:    BoolType,
			want: false,
		},
		{
			name: "test with float64 type",
			t:    Float64Type,
			want: float64(0),
		},
		{
			name: "test with string type",
			t:    StringType,
			want: "",
		},
		{
			name: "test with byte type",
			t:    ByteType,
			want: byte(0),
		},
		{
			name: "test with int32 type",
			t:    Int32Type,
			want: int32(0),
		},
		{
			name: "test with interface type",
			t:    InterfaceType,
			want: Wrap(nil),
		},
		{
			name: "test with struct type",
			t:    NewType("testStruct", StructKind, nil),
			want: NewStruct(NewType("testStruct", StructKind, nil)),
		},
		{
			// InstanceOfType for a map type now returns a nil-state map, matching
			// Go's zero value for "var m map[K]V". The *Map wrapper is non-nil (it
			// retains key/value type metadata) but its internal data field is nil.
			// Map literals ("m := map[K]V{}") use $new() → NewMap() separately.
			name: "test with map type",
			t: &Type{
				kind:      MapKind,
				keyType:   StringType,
				valueType: IntType,
			},
			want: NewNilMap(StringType, IntType),
		},
		{
			name: "test with array type",
			t: &Type{
				kind:      ArrayKind,
				valueType: IntType,
			},
			want: NewArray(IntType, 0),
		},
		{
			name: "test with base type",
			t:    IntType,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InstanceOfType(tt.t); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InstanceOfType(%s) = %#v, want %#v", tt.name, got, tt.want)
			}
		})
	}
}
