package datatypes

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
			want:  boolKind,
		},
		{
			name:  "int",
			value: 55,
			want:  intKind,
		},
		{
			name:  "float",
			value: 1.23,
			want:  floatKind,
		},
		{
			name:  "string",
			value: "whizz",
			want:  stringKind,
		},
		{
			name:  "nil",
			value: nil,
			want:  interfaceKind,
		},
	}

	for _, tt := range tests {
		v = tt.value
		p := &v

		got := TypeOfPointer(p)

		if got.Kind != tt.want {
			t.Errorf("PointerTo(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTypeString(t *testing.T) {
	tests := []struct {
		name string
		arg  interface{}
	}{
		{
			name: "int",
			arg:  intKind,
		},
		{
			name: "int",
			arg:  Type{Name: "int", Kind: intKind},
		},
		{
			name: "[]int",
			arg: Type{
				Name: "[]",
				Kind: arrayKind,
				ValueType: &Type{
					Name: "int",
					Kind: intKind},
			},
		},
		{
			name: "[]*int",
			arg: Type{
				Name: "[]",
				Kind: arrayKind,
				ValueType: &Type{
					Name:      "*",
					Kind:      pointerKind,
					ValueType: &IntTypeDef,
				},
			},
		},
		{
			name: "map[string][]int",
			arg: Type{
				Name:    "map",
				Kind:    mapKind,
				KeyType: &StringTypeDef,
				ValueType: &Type{
					Name:      "[]",
					Kind:      arrayKind,
					ValueType: &IntTypeDef,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TypeString(tt.arg); got != tt.name {
				t.Errorf("TypeString() = %v, want %v", got, tt.name)
			}
		})
	}
}
