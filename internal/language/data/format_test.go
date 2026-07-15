package data

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		arg  any
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
		{
			name: "complex128",
			arg:  complex(3, 4),
			want: "(3+4i)",
		},
		{
			name: "complex128 negative imaginary",
			arg:  complex(3, -4),
			want: "(3-4i)",
		},
		{
			name: "complex128 zero",
			arg:  complex128(0),
			want: "(0+0i)",
		},
		{
			name: "complex64",
			arg:  complex64(complex(2.5, -1.5)),
			want: "(2.5-1.5i)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.arg); got != tt.want {
				t.Errorf("Format() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatWithType_Complex(t *testing.T) {
	tests := []struct {
		name string
		arg  any
		want string
	}{
		{
			name: "complex128",
			arg:  complex(3, 4),
			want: "complex128((3+4i))",
		},
		{
			name: "complex64",
			arg:  complex64(complex(3, 4)),
			want: "complex64((3+4i))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatWithType(tt.arg); got != tt.want {
				t.Errorf("FormatWithType() = %v, want %v", got, tt.want)
			}
		})
	}
}
