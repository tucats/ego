package data

import (
	"testing"
)

func TestPointerTo(t *testing.T) {
	var v any

	type test struct {
		name  string
		value any
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
	// We're going to reuse this type a bunch, so make a common type value.
	personType := Type{
		name: "Person",
		kind: TypeKind,
		valueType: &Type{
			kind: StructKind,
			fields: map[string]*Type{
				"age":  {kind: IntKind},
				"name": {kind: StringKind},
			},
		},
	}

	// The test name is also the expected result of the string-ify of the
	// type definition.
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
		{
			name: "Person struct{age , name }",
			arg:  personType,
		},
		{
			name: "*Person struct{age , name }",
			arg: Type{
				kind:      PointerKind,
				valueType: &personType,
			},
		},
		{
			name: "[]Person struct{age , name }",
			arg: Type{
				kind:      ArrayKind,
				valueType: &personType,
			},
		},
		{
			name: "[]*Person struct{age , name }",
			arg: Type{
				kind: ArrayKind,
				valueType: &Type{
					kind:      PointerKind,
					valueType: &personType,
				},
			},
		}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.arg.String(); got != tt.name {
				t.Errorf("TypeString() = %v, want %v", got, tt.name)
			}
		})
	}
}
