package functions

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/errors"
)

func Test_scanner(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		format string
		want   []interface{}
		err    *errors.EgoError
	}{
		{
			name:   "Const mismatch",
			data:   "Hello Tom",
			format: "Name %s",
			want:   []interface{}{},
			err:    nil,
		},
		{
			name:   "Const and string",
			data:   "Name Tom",
			format: "Name %s",
			want:   []interface{}{"Tom"},
			err:    nil,
		},
		{
			name:   "Const, string, int",
			data:   "name Tom age 44",
			format: "name %s age %d",
			want:   []interface{}{"Tom", 44},
			err:    nil,
		},
		{
			name:   "Single string",
			data:   "Tom",
			format: "%s",
			want:   []interface{}{"Tom"},
			err:    nil,
		},
		{
			name:   "Single integer",
			data:   "35",
			format: "%d",
			want:   []interface{}{35},
			err:    nil,
		},
		{
			name:   "Single float64",
			data:   "3.14",
			format: "%f",
			want:   []interface{}{3.14},
			err:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, e := scanner(tt.data, tt.format)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scanner() got = %v, want %v", got, tt.want)
			}
			if e.Is(tt.err) {
				t.Errorf("scanner() got1 = %v, want %v", e, tt.err)
			}
		})
	}
}
