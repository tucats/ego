package data

import (
	"testing"
)

func TestSizeOf(t *testing.T) {
	tests := []struct {
		name string
		arg  interface{}
		want int
	}{
		{
			name: "bool",
			arg:  true,
			want: 1,
		},
		{
			name: "byte",
			arg:  byte(0),
			want: 1,
		},
		{
			name: "int32",
			arg:  int32(0),
			want: 4,
		},
		{
			name: "int",
			arg:  int(0),
			want: 64,
		},
		{
			name: "int64",
			arg:  int64(0),
			want: 8,
		},
		{
			name: "float32",
			arg:  float32(0),
			want: 4,
		},
		{
			name: "float64",
			arg:  float64(0),
			want: 8,
		},
		{
			name: "string",
			arg:  "test",
			want: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SizeOf(tt.arg); got != tt.want {
				t.Errorf("SizeOf(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
