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
			if got := isStackMarker(tt.i, tt.types...); got != tt.want {
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
			want:   "Marker<test>",
		},
		{
			name:   "marker with one item",
			marker: NewStackMarker("test", 33),
			want:   "Marker<test, 33>",
		},
		{
			name:   "marker with multiple items",
			marker: NewStackMarker("test", 33, "foo", true),
			want:   "Marker<test, 33, foo, true>",
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
func TestFindMarker(t *testing.T) {
	tests := []struct {
		name     string
		context  *Context
		input    interface{}
		expected int
	}{
		{
			name: "find marker in after multiple items on stack",
			context: &Context{
				stack:        []interface{}{NewStackMarker("marker1"), 103, 102, 101},
				stackPointer: 3,
				framePointer: 0,
			},
			input:    "marker1",
			expected: 3,
		},
		{
			name: "find marker in stack",
			context: &Context{
				stack:        []interface{}{"test", NewStackMarker("marker1"), "marker2"},
				stackPointer: 2,
				framePointer: 0,
			},
			input:    "marker1",
			expected: 1,
		},
		{
			name: "find marker in stack with multiple markers",
			context: &Context{
				stack:        []interface{}{"test", NewStackMarker("marker1"), "marker2", NewStackMarker("marker3")},
				stackPointer: 3,
				framePointer: 0,
			},
			input:    "marker3",
			expected: 0,
		},
		{
			name: "find marker in stack with no match",
			context: &Context{
				stack:        []interface{}{"test", NewStackMarker("marker1"), "marker2"},
				stackPointer: 2,
				framePointer: 0,
			},
			input:    "marker3",
			expected: 0,
		},
		{
			name: "find marker in empty stack",
			context: &Context{
				stack:        []interface{}{},
				stackPointer: 0,
				framePointer: 0,
			},
			input:    "marker1",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findMarker(tt.context, tt.input); got != tt.expected {
				t.Errorf("findMarker() = %v, want %v", got, tt.expected)
			}
		})
	}
}
