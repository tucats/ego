package bytecode

import (
	"testing"
)

func TestIsStackMarker(t *testing.T) {
	tests := []struct {
		name  string
		i     interface{}
		types []string
		want  bool
	}{
		{
			name: "simple type test of simple marker",
			i:    NewStackMarker("test"),
			want: true,
		},
		{
			name: "simple type test of not-a-marker",
			i:    "test",
			want: false,
		},
		{
			name: "simple type test of complex marker",
			i:    NewStackMarker("test", 33, true),
			want: true,
		},
		{
			name:  "complex type test of matching marker",
			i:     NewStackMarker("test", 33, true),
			types: []string{"true"},
			want:  true,
		},
		{
			name:  "complex type test of mismatched marker",
			i:     NewStackMarker("test", 33, true),
			types: []string{"call"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStackMarker(tt.i, tt.types...); got != tt.want {
				t.Errorf("IsStackMarker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStackMarker_String(t *testing.T) {
	tests := []struct {
		name   string
		marker StackMarker
		want   string
	}{
		{
			name:   "simple marker",
			marker: NewStackMarker("test"),
			want:   "M<test>",
		},
		{
			name:   "marker with one item",
			marker: NewStackMarker("test", 33),
			want:   "M<test, 33>",
		},
		{
			name:   "marker with multiple items",
			marker: NewStackMarker("test", 33, "foo", true),
			want:   "M<test, 33, foo, true>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.marker.String(); got != tt.want {
				t.Errorf("StackMarker.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
