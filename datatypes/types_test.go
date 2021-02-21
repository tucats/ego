package datatypes

import "testing"

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
			want:  BoolType,
		},
		{
			name:  "int",
			value: 55,
			want:  IntType,
		},
		{
			name:  "float",
			value: 1.23,
			want:  FloatType,
		},
		{
			name:  "string",
			value: "whizzy",
			want:  StringType,
		},
		{
			name:  "nil",
			value: nil,
			want:  InterfaceType,
		},
	}

	for _, tt := range tests {
		v = tt.value
		p := &v

		got := PointerTo(p)

		if got != tt.want {
			t.Errorf("PointerTo(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
