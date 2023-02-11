package data

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		arg  interface{}
		want string
	}{
		{
			name: "float64",
			arg:  3.14,
			want: "3.14",
		},
		{
			name: "struct type",
			arg:  TypeDefinition("bang", StructType),
			want: "bang struct",
		},
		{
			name: "struct",
			arg:  StructType,
			want: "struct",
		},
		{
			name: "array",
			arg:  NewArrayFromList(IntType, NewList(1, 2, 3)),
			want: "[1, 2, 3]",
		},
		{
			name: "array type",
			arg:  ArrayType(IntType),
			want: "[]int",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.arg); got != tt.want {
				t.Errorf("Format() = %v, want %v", got, tt.want)
			}
		})
	}
}
