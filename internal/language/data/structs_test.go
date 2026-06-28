package data

import (
	"testing"
)

func TestNewStruct(t *testing.T) {
	type fields struct {
		typeDef  *Type
		static   bool
		fields   map[string]any
		typeName string
	}

	type args struct {
		t *Type
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Struct
	}{
		{
			name: "test with empty struct",
			fields: fields{
				typeDef: &Type{
					kind:   StructKind,
					fields: map[string]*Type{},
				},
				static:   false,
				fields:   map[string]any{},
				typeName: "",
			},
			args: args{
				t: &Type{
					kind:   StructKind,
					fields: map[string]*Type{},
				},
			},
			want: &Struct{
				typeDef:  &Type{kind: StructKind, fields: map[string]*Type{}},
				static:   false,
				fields:   map[string]any{},
				typeName: "",
			},
		},
		{
			name: "test with struct with fields",
			fields: fields{
				typeDef: &Type{
					kind: StructKind,
					fields: map[string]*Type{
						"field1": IntType,
						"field2": StringType,
					},
				},
				static:   true,
				fields:   map[string]any{"field1": 0, "field2": ""},
				typeName: "",
			},
			args: args{
				t: &Type{
					kind: StructKind,
					fields: map[string]*Type{
						"field1": IntType,
						"field2": StringType,
					},
				},
			},
			want: &Struct{
				typeDef: &Type{
					kind: StructKind,
					fields: map[string]*Type{
						"field1": IntType,
						"field2": StringType,
					},
				},
				static:   true,
				fields:   map[string]any{"field1": 0, "field2": ""},
				typeName: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewStruct(tt.args.t); !got.DeepEqual(tt.want) {
				t.Errorf("NewStruct(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
