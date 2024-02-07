package sort

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func TestFunctionSort(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "integer sort",
			args: data.NewList(
				data.NewArrayFromList(data.IntType,
					data.NewList(55, 2, 18),
				),
			),
			want: []interface{}{2, 18, 55},
		},
		{
			name:    "scalar args",
			args:    data.NewList(66, 55),
			want:    []interface{}{55, 66},
			wantErr: false,
		},
		{
			name:    "mixed scalar args",
			args:    data.NewList("tom", 3),
			want:    []interface{}{"3", "tom"},
			wantErr: false,
		},
		{
			name: "integer sort",
			args: data.NewList(
				data.NewArrayFromList(data.IntType,
					data.NewList(55, 2, 18),
				),
			),
			want: []interface{}{2, 18, 55},
		},
		{
			name: "float64 sort",
			args: data.NewList(
				data.NewArrayFromList(data.Float64Type,
					data.NewList(55.0, 2, 18.5),
				),
			),
			want: []interface{}{2.0, 18.5, 55.0},
		},
		{
			name: "string sort",
			args: data.NewList(
				data.NewArrayFromList(data.StringType,
					data.NewList("pony", "cake", "unicorn", 5),
				),
			),
			want: []interface{}{"5", "cake", "pony", "unicorn"},
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewSymbolTable("sort testing")
			s.SetAlways(defs.ExtensionsVariable, true)

			got, err := genericSort(s, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionSort() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			
			gotArray, ok := got.(*data.Array)
			if !ok || !reflect.DeepEqual(gotArray.BaseArray(), tt.want) {
				t.Errorf("FunctionSort() = %v, want %v", got, tt.want)
			}
		})
	}
}
