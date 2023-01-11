package data

import (
	"reflect"
	"testing"
)

func TestNewMapFromMap(t *testing.T) {
	type args struct {
		v interface{}
	}

	tests := []struct {
		name string
		args args
		want *Map
	}{
		{
			name: "int map",
			args: args{v: map[string]int{"tom": 15, "sue": 19}},
			want: &Map{
				keyType:   StringType,
				valueType: IntType,
				immutable: 0,
				data:      map[interface{}]interface{}{"tom": 15, "sue": 19},
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMapFromMap(tt.args.v); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMapFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
