package reflect

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/language/data"
)

func TestFunctionMembers(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    any
		wantErr bool
	}{
		{
			name: "simple struct",
			args: data.NewList(
				data.NewStructFromMap(
					map[string]any{"name": "Tom", "age": 55},
				).SetFieldOrder([]string{"name", "age"}),
			),
			want: data.NewArrayFromList(data.StringType, data.NewList("name", "age")),
		},
		{
			name:    "wrong type struct",
			args:    data.NewList(55),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := members(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionMembers() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			// members() now returns a data.List of (value, error), matching
			// the (value, error) convention used throughout the runtime
			// packages, so the value must be unwrapped before comparing.
			list, ok := result.(data.List)
			if !ok {
				t.Fatalf("FunctionMembers() returned %T, want data.List", result)
			}

			got := list.Get(0)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionMembers() = %v, want %v", got, tt.want)
			}
		})
	}
}
