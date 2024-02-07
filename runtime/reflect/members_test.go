package reflect

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
)

func TestFunctionMembers(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple struct",
			args: data.NewList(
				data.NewStructFromMap(
					map[string]interface{}{"name": "Tom", "age": 55},
				),
			),
			want: data.NewArrayFromList(data.StringType, data.NewList("age", "name")),
		},
		{
			name:    "wrong type struct",
			args:    data.NewList(55),
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := members(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionMembers() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionMembers() = %v, want %v", got, tt.want)
			}
		})
	}
}
