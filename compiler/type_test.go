package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
)

func TestCompileTypeSpec(t *testing.T) {
	tests := []struct {
		name string
		want *data.Type
	}{
		{
			name: "int",
			want: &data.IntType,
		},
		{
			name: "float64",
			want: &data.Float64Type,
		},
		{
			name: "string",
			want: &data.StringType,
		},
		{
			name: "*int",
			want: data.Pointer(&data.IntType),
		},
		{
			name: "[]string",
			want: data.Array(&data.StringType),
		},
		{
			name: "struct { name int }",
			want: data.Structure(data.Field{Name: "name", Type: &data.IntType}),
		},
		{
			name: "struct { name string, age int }",
			want: data.Structure(
				data.Field{Name: "name", Type: &data.StringType},
				data.Field{Name: "age", Type: &data.IntType},
			),
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompileTypeSpec(tt.name)
			if err != nil {
				t.Errorf("CompileTypeSpec() error %v", err)
			} else {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("CompileTypeSpec() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
