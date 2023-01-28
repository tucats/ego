package reflect

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func TestReflect(t *testing.T) {
	type args struct {
		s    *symbols.SymbolTable
		args []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple struct",
			args: args{
				s: nil,
				args: []interface{}{
					data.NewStructFromMap(map[string]interface{}{
						"name": "Tom",
						"age":  55,
					}),
				},
			},
			want: data.NewStructFromMap(map[string]interface{}{
				"basetype": "struct{age int, name string}",
				"type":     "struct{age int, name string}",
				"native":   true,
				"package":  false,
				"readonly": false,
				"static":   true,
				"istype":   false,
				"members":  data.NewArrayFromArray(data.StringType, []interface{}{"age", "name"}),
			}),
			wantErr: false,
		},
		{
			name: "simple integer value",
			args: args{s: nil, args: []interface{}{33}},
			want: data.NewStructFromMap(map[string]interface{}{
				"basetype": "int",
				"type":     "int",
				"istype":   false,
			}),
			wantErr: false,
		},
		{
			name: "simple string value",
			args: args{s: nil, args: []interface{}{"stuff"}},
			want: data.NewStructFromMap(map[string]interface{}{
				"basetype": "string",
				"type":     "string",
				"istype":   false,
			}),
			wantErr: false,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := describe(tt.args.s, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reflect() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reflect() = %v, want %v", got, tt.want)
			}
		})
	}
}
