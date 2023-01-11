package compiler

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/tokenizer"
)

func TestCompiler_typeCompiler(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		typeName string
		want     *data.Type
		wantErr  error
		debug    bool
	}{
		{
			name:     "map",
			arg:      "map[string]int",
			typeName: "type0",
			want: data.TypeDefinition("type0", data.MapType(
				data.StringType,
				data.IntType)),
			wantErr: nil,
		},
		{
			name:     "struct",
			arg:      "struct{ age int name string }",
			typeName: "type1",
			want: data.TypeDefinition("type1", data.StructureType(
				data.Field{Name: "age", Type: data.IntType},
				data.Field{Name: "name", Type: data.StringType},
			)),
			wantErr: nil,
		},
		{
			name:     "int",
			arg:      "int",
			typeName: "type2",
			want:     data.TypeDefinition("type2", data.IntType),
			wantErr:  nil,
		},

		// TODO: Add test cases.
	}

	c := New("unit test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.t = tokenizer.New(tt.arg)

			if tt.debug {
				fmt.Println("DEBUG")
			}

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
