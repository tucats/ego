package reflect

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
)

func TestFunctionMembers(t *testing.T) {
	type args struct {
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
			args: args{[]interface{}{
				data.NewStructFromMap(
					map[string]interface{}{"name": "Tom", "age": 55},
				),
			}},
			want: data.NewArrayFromArray(data.StringType, []interface{}{"age", "name"}),
		},
		{
			name: "empty struct",
			args: args{[]interface{}{data.NewStruct(data.StructType)}},
			want: data.NewArrayFromArray(data.StringType, []interface{}{}),
		},
		{
			name:    "wrong type struct",
			args:    args{[]interface{}{55}},
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := members(nil, tt.args.args)
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
