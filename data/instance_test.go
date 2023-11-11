package data

import (
	"reflect"
	"sync"
	"testing"
)

func TestInstanceOfType(t *testing.T) {
	tests := []struct {
		name string
		t    *Type
		want interface{}
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
			name: "test with map type",
			t: &Type{
				kind:      MapKind,
				keyType:   StringType,
				valueType: IntType,
			},
			want: NewMap(StringType, IntType),
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
			name: "test with mutex type",
			t:    MutexType,
			want: &sync.Mutex{},
		},
		{
			name: "test with waitgroup type",
			t:    WaitGroupType,
			want: &sync.WaitGroup{},
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
