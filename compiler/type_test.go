package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

func TestCompileTypeSpec(t *testing.T) {
	tests := []struct {
		name string
		want *datatypes.Type
	}{
		{
			name: "int",
			want: &datatypes.IntType,
		},
		{
			name: "float64",
			want: &datatypes.Float64Type,
		},
		{
			name: "string",
			want: &datatypes.StringType,
		},
		{
			name: "*int",
			want: datatypes.Pointer(&datatypes.IntType),
		},
		{
			name: "[]string",
			want: datatypes.Array(&datatypes.StringType),
		},
		{
			name: "struct { name int }",
			want: datatypes.Structure(datatypes.Field{Name: "name", Type: &datatypes.IntType}),
		},
		{
			name: "struct { name string, age int }",
			want: datatypes.Structure(
				datatypes.Field{Name: "name", Type: &datatypes.StringType},
				datatypes.Field{Name: "age", Type: &datatypes.IntType},
			),
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompileTypeSpec(tt.name)
			if !errors.Nil(err) {
				t.Errorf("CompileTypeSpec() error %v", err)
			} else {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("CompileTypeSpec() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
