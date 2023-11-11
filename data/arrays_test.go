package data

import (
	"testing"
)

func TestNewArray(t *testing.T) {
	type args struct {
		valueType *Type
		size      int
	}

	tests := []struct {
		name string
		args args
		want *Array
	}{
		{
			name: "test with int type",
			args: args{
				valueType: IntType,
				size:      5,
			},
			want: &Array{
				data:      []interface{}{0, 0, 0, 0, 0},
				valueType: IntType,
				immutable: 0,
			},
		},
		{
			name: "test with float64 type",
			args: args{
				valueType: Float64Type,
				size:      5,
			},
			want: &Array{
				data:      []interface{}{float64(0), float64(0), float64(0), float64(0), float64(0)},
				valueType: Float64Type,
				immutable: 0,
			},
		},
		{
			name: "test with string type",
			args: args{
				valueType: StringType,
				size:      3,
			},
			want: &Array{
				data:      []interface{}{"", "", ""},
				valueType: StringType,
				immutable: 0,
			},
		},
		{
			name: "test with byte type",
			args: args{
				valueType: ByteType,
				size:      2,
			},
			want: &Array{
				bytes:     []byte{0, 0},
				valueType: ByteType,
				immutable: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewArray(tt.args.valueType, tt.args.size); !got.DeepEqual(tt.want) {
				t.Errorf("NewArray(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
func TestNewArrayFromList(t *testing.T) {
	type args struct {
		valueType *Type
		source    List
	}

	tests := []struct {
		name string
		args args
		want *Array
	}{
		{
			name: "test with byte type",
			args: args{
				valueType: ByteType,
				source:    NewList(1, 2, 3),
			},
			want: &Array{
				bytes:     []byte{1, 2, 3},
				valueType: ByteType,
				immutable: 0,
			},
		},
		{
			name: "test with int type",
			args: args{
				valueType: IntType,
				source:    NewList(1, 2, 3),
			},
			want: &Array{
				data:      []interface{}{1, 2, 3},
				valueType: IntType,
				immutable: 0,
			},
		},
		{
			name: "test with float64 type",
			args: args{
				valueType: Float64Type,
				source:    NewList(1.1, 2.2, 3.3),
			},
			want: &Array{
				data:      []interface{}{1.1, 2.2, 3.3},
				valueType: Float64Type,
				immutable: 0,
			},
		},
		{
			name: "test with string type",
			args: args{
				valueType: StringType,
				source:    NewList("a", "b", "c"),
			},
			want: &Array{
				data:      []interface{}{"a", "b", "c"},
				valueType: StringType,
				immutable: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewArrayFromList(tt.args.valueType, tt.args.source); !got.DeepEqual(tt.want) {
				t.Errorf("NewArrayFromList(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
