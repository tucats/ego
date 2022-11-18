package compiler

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_typeCompiler(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		typeName string
		want     datatypes.Type
		wantErr  *errors.EgoError
	}{
		{
			name:     "map",
			arg:      "map[string]int",
			typeName: "type0",
			want: *datatypes.TypeDefinition("type0", datatypes.Map(
				&datatypes.StringType,
				&datatypes.IntType)),
			wantErr: nil,
		},
		{
			name:     "struct",
			arg:      "struct{ age int name string }",
			typeName: "type1",
			want: *datatypes.TypeDefinition("type1", datatypes.Structure(
				datatypes.Field{Name: "age", Type: &datatypes.IntType},
				datatypes.Field{Name: "name", Type: &datatypes.StringType},
			)),
			wantErr: nil,
		},
		{
			name:     "int",
			arg:      "int",
			typeName: "type2",
			want:     *datatypes.TypeDefinition("type2", &datatypes.IntType),
			wantErr:  nil,
		},

		// TODO: Add test cases.
	}

	c := New("unit test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.t = tokenizer.New(tt.arg)

			got, err := c.typeCompiler(tt.typeName)
			if err != tt.wantErr {
				t.Errorf("Compiler.typeCompiler() Unexpected error condition %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Compiler.typeCompiler() = %v, want %v", got, tt.want)
			}
		})
	}
}
