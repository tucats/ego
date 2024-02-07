package fmt

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
		err    error
	}{
		{
			name:   "String with width int",
			data:   " 12 34 567 ",
			format: "%5s%d",
			want:   []interface{}{"12 34", 567},
			err:    nil,
		},
		{
			name:   "Invalid integer",
			data:   "thirty",
			format: "%d",
			want:   []interface{}{},
			err:    errors.ErrInvalidValue,
		},
		{
			name:   "Single float64",
			data:   "3.14",
			format: "%f",
			want:   []interface{}{3.14},
			err:    nil,
		},
		{
			name:   "two hexadecimal ints with widths",
			data:   "DEAD BEEF",
			format: "%4x %4x",
			want:   []interface{}{57005, 48879},
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
			name:   "boolean",
			data:   "so true",
			format: "so %t",
			want:   []interface{}{true},
			err:    nil,
		},
		{
			name:   "binary int",
			data:   "1101",
			format: "%b",
			want:   []interface{}{13},
			err:    nil,
		},
		{
			name:   "hexadecimal int",
			data:   "DEADBEEF",
			format: "%x",
			want:   []interface{}{3735928559},
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
			name:   "Const mismatch",
			data:   "Hello Tom",
			format: "Name %s",
			want:   []interface{}{},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, e := scanner(tt.data, tt.format)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scanner() got = %v, want %v", got, tt.want)
			}
			
			if !errors.Equals(e, tt.err) {
				t.Errorf("scanner() got1 = %v, want %v", e, tt.err)
			}
		})
	}
}
