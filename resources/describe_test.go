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
					SQLName: "field1",
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

func TestResHandle_explode(t *testing.T) {
	tests := []struct {
		name   string
		object interface{}
		want   []interface{}
	}{
		{
			name: "struct with integer",
			object: struct {
				Foo int
			}{Foo: 42},
			want: []interface{}{42},
		},
		{
			name: "pointer to struct with integer",
			object: &struct {
				Foo int
			}{Foo: 42},
			want: []interface{}{42},
		},
		{
			name: "complex struct 1",
			object: struct {
				Name        string
				DSN         string
				Permissions int
			}{Name: "fred", DSN: "test01", Permissions: 8},
			want: []interface{}{"fred", "test01", 8},
		}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ResHandle{
				Columns: describe(tt.object),
				Type:    reflect.TypeOf(tt.object),
			}

			if got := r.explode(tt.object); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResHandle.explode() = %v, want %v", got, tt.want)
			}
		})
	}
}
