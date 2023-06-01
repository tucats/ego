package resources

import (
	"reflect"
	"testing"
)

func Test_describe(t *testing.T) {
	tests := []struct {
		name   string
		object interface{}
		want   []Column
	}{
		{
			name: "integer field",
			object: struct {
				Field1 int
			}{
				Field1: 42,
			},
			want: []Column{
				{
					Name:    "Field1",
					SQLType: "integer",
					Index:   0,
				},
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := describe(tt.object); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("describe() = %v, want %v", got, tt.want)
			}
		})
	}
}
