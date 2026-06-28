package data

import (
	"reflect"
	"testing"
)

func TestNewMapFromMap(t *testing.T) {
	type args struct {
		v any
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
				keyType:     StringType,
				elementType: IntType,
				immutable:   0,
				data:        map[any]any{"tom": 15, "sue": 19},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMapFromMap(tt.args.v); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMapFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
