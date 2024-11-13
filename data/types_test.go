package data

import (
	"testing"
)

func TestPointerTo(t *testing.T) {
	var v interface{}

	type test struct {
		name  string
		value interface{}
		want  int
	}

	tests := []test{
		{
			name:  "bool",
			value: true,
			want:  BoolKind,
		},
		{
			name:  "int",
			value: 55,
			want:  IntKind,
		},
		{
			name:  "float64",
			value: 1.23,
			want:  Float64Kind,
		},
		{
			name:  "string",
			value: "whizz",
			want:  StringKind,
		},
		{
			name:  "nil",
			value: nil,
			want:  NilKind,
		},
	}

	for _, tt := range tests {
		v = tt.value
		p := &v

		got := TypeOfPointer(p)

		if got.kind != tt.want {
			t.Errorf("PointerTo(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTypeString(t *testing.T) {
	tests := []struct {
		name string
		arg  Type
	}{
		{
			name: "int",
			arg:  Type{name: "int", kind: IntKind},
		},
		{
			name: "[]int",
			arg: Type{
				name: "[]",
				kind: ArrayKind,
				valueType: &Type{
					name: "int",
					kind: IntKind},
			},
		},
		{
			name: "[]*int",
			arg: Type{
				name: "[]",
				kind: ArrayKind,
				valueType: &Type{
					name:      "*",
					kind:      PointerKind,
					valueType: IntType,
				},
			},
		},
		{
			name: "map[string][]int",
			arg: Type{
				name:    "map",
				kind:    MapKind,
				keyType: StringType,
				valueType: &Type{
					name:      "[]",
					kind:      ArrayKind,
					valueType: IntType,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.arg.String(); got != tt.name {
				t.Errorf("TypeString() = %v, want %v", got, tt.name)
			}
		})
	}
}
